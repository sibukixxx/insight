package publicengine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"insight-lab/internal/analytical"
	"insight-lab/internal/buildinfo"
	"insight-lab/internal/domain"
	"insight-lab/internal/execution"
	"insight-lab/internal/input"
	"insight-lab/internal/repository"
	"insight-lab/internal/service"
	"insight-lab/internal/usecase"
)

// Limits of the v1 contract (schema.json x-limits).
const (
	MaxRequestBytes          = 16 << 20
	maxMetadataEntries       = 32
	maxMetadataKeyLength     = 64
	maxMetadataValueLength   = 1024
	maxDocumentsPerRequest   = 500
	maxDocumentContentLength = 200000
	maxArtifactsPerRequest   = 50
	maxQuestionLength        = 2000
	maxIdentifierLength      = 128
)

// metadataExternalRef records the caller's reference on a document created
// through the public contract.
const metadataExternalRef = "public_external_ref"

var metadataKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,64}$`)

// Enqueuer starts an analysis run. *service.JobManager implements it.
type Enqueuer interface {
	Enqueue(ctx context.Context, req service.EnqueueRequest) (*domain.Analysis, error)
}

// Engine executes Public Engine Contract operations.
type Engine struct {
	app       *usecase.Application
	public    repository.PublicRepository
	documents repository.DocumentRepository
	jobs      Enqueuer
	build     buildinfo.Info
	now       func() time.Time
	triage    *triageDeps

	resolver      input.Resolver
	maxRawBytes   int64
	allowedModels []string
	modelBacked   func() bool

	// mu serializes mutating operations so an idempotency replay check and
	// an identity check can never interleave with the write they guard.
	mu sync.Mutex
}

func New(app *usecase.Application, public repository.PublicRepository, documents repository.DocumentRepository, jobs Enqueuer, build buildinfo.Info, opts ...Option) *Engine {
	e := &Engine{app: app, public: public, documents: documents, jobs: jobs, build: build, now: func() time.Time { return time.Now().UTC() }}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Engine returns the contract and build identity of this engine.
func (e *Engine) Engine() EngineInfo {
	return EngineInfo{
		ContractSchema: ContractSchema, ContractVersion: ContractVersion,
		SupportedContractVersions: append([]string(nil), SupportedContractVersions...),
		Engine:                    EngineBuild{Version: e.build.Version, Commit: e.build.Commit, Dirty: e.build.Dirty},
		ResearchArtifact:          SchemaRef{Schema: usecase.ResearchArtifactSchema, Version: usecase.ResearchArtifactVersion},
		AnalyticalArtifact:        SchemaRef{Schema: analytical.Schema, Version: analytical.Version},
		ExecutionProfiles:         profileInfos(e.capabilities()),
		InputSourceKinds:          input.Kinds(),
		ModelRouting:              e.modelRouting(),
		ModelBacked:               e.modelBacked != nil && e.modelBacked(),
		ReasoningProfiles:         reasoningProfileNames(),
	}
}

func reasoningProfileNames() []string {
	var out []string
	for _, p := range domain.ReasoningProfiles() {
		out = append(out, string(p))
	}
	return out
}

// ---------- idempotency ----------

// idempotent runs op at most once per idempotency key. A retry with the same
// key and the same request replays the first successful response verbatim;
// the same key with a different request is IDEMPOTENCY_CONFLICT. Failed
// attempts are not stored, so a caller may retry them.
func (e *Engine) idempotent(ctx context.Context, version, key, operation string, request any, op func() (int, any, error)) (int, []byte, error) {
	if err := checkEnvelope(version, key); err != nil {
		return 0, nil, err
	}
	hash, err := service.Fingerprint(struct {
		Operation string `json:"operation"`
		Request   any    `json:"request"`
	}{operation, request})
	if err != nil {
		return 0, nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	stored, err := e.public.GetResponse(ctx, key)
	switch {
	case err == nil:
		if stored.RequestHash != hash {
			return 0, nil, newError(CodeIdempotencyConflict, "idempotency key %q was already used for a different request", key)
		}
		return stored.StatusCode, stored.Body, nil
	case !errors.Is(err, repository.ErrNotFound):
		return 0, nil, err
	}
	status, value, err := op()
	if err != nil {
		return 0, nil, err
	}
	body, err := json.Marshal(value)
	if err != nil {
		return 0, nil, err
	}
	if err := e.public.SaveResponse(ctx, &repository.IdempotentResponse{Key: key, RequestHash: hash, StatusCode: status, Body: body, CreatedAt: e.now()}); err != nil {
		return 0, nil, err
	}
	return status, body, nil
}

func checkEnvelope(version, key string) error {
	if version != ContractVersion {
		return newError(CodeUnsupportedContractVersion, "contractVersion %q is not supported; supported versions: %s", version, strings.Join(SupportedContractVersions, ", "))
	}
	if key == "" || len(key) > maxIdentifierLength {
		return newError(CodeInvalidRequest, "idempotencyKey must be 1-%d characters", maxIdentifierLength)
	}
	return nil
}

// ---------- subjects ----------

// CreateSubject creates a subject, or returns the existing one when the same
// opaque (namespace, id) was already created with identical content.
func (e *Engine) CreateSubject(ctx context.Context, req CreateSubjectRequest) (int, []byte, error) {
	return e.idempotent(ctx, req.ContractVersion, req.IdempotencyKey, "createSubject", req, func() (int, any, error) {
		if err := validateSubject(req); err != nil {
			return 0, nil, err
		}
		title := strings.TrimSpace(req.Title)
		if title == "" {
			title = req.Subject.Namespace + "/" + req.Subject.ID
		}
		if existing, err := e.public.FindSubject(ctx, req.Subject.Namespace, req.Subject.ID); err == nil {
			return e.existingSubject(ctx, existing, req.Subject, title, req.Metadata)
		} else if !errors.Is(err, repository.ErrNotFound) {
			return 0, nil, err
		}
		now := e.now()
		project := &domain.Project{ID: newID("proj"), Name: title, CreatedAt: now}
		subject := &repository.PublicSubject{ProjectID: project.ID, Namespace: req.Subject.Namespace, ExternalID: req.Subject.ID, Type: req.Subject.Type, Metadata: req.Metadata, CreatedAt: now}
		if err := e.public.CreateSubject(ctx, project, subject); err != nil {
			return 0, nil, err
		}
		return http.StatusCreated, toSubject(subject, title), nil
	})
}

func (e *Engine) existingSubject(ctx context.Context, existing *repository.PublicSubject, ref SubjectRef, title string, metadata map[string]string) (int, any, error) {
	project, err := e.app.GetProject(ctx, existing.ProjectID)
	if err != nil {
		return 0, nil, err
	}
	if existing.Type != ref.Type || project.Name != title || !maps.Equal(existing.Metadata, metadata) {
		return 0, nil, newError(CodeIdentityConflict, "subject %s/%s already exists with different content", ref.Namespace, ref.ID)
	}
	return http.StatusOK, toSubject(existing, project.Name), nil
}

func toSubject(s *repository.PublicSubject, title string) Subject {
	return Subject{
		ContractVersion: ContractVersion, SubjectID: s.ProjectID,
		Subject: SubjectRef{Namespace: s.Namespace, ID: s.ExternalID, Type: s.Type},
		Title:   title, Metadata: s.Metadata, CreatedAt: formatTime(s.CreatedAt),
	}
}

func (e *Engine) requireSubject(ctx context.Context, subjectID string) (*repository.PublicSubject, error) {
	subject, err := e.public.GetSubject(ctx, subjectID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, newError(CodeNotFound, "subject %q not found", subjectID)
	}
	return subject, err
}

// ---------- evidence ----------

// AddEvidence adds documents and Analytical Artifacts to a subject. Each
// document is identified by its externalRef and each artifact by its id;
// resending identical content is UNCHANGED, different content under the
// same identity is IDENTITY_CONFLICT. Nothing is written unless every item
// is accepted.
func (e *Engine) AddEvidence(ctx context.Context, subjectID string, req AddEvidenceRequest) (int, []byte, error) {
	return e.idempotent(ctx, req.ContractVersion, req.IdempotencyKey, "addEvidence:"+subjectID, req, func() (int, any, error) {
		subject, err := e.requireSubject(ctx, subjectID)
		if err != nil {
			return 0, nil, err
		}
		if len(req.Documents) == 0 && len(req.AnalyticalArtifacts) == 0 && len(req.InputSources) == 0 {
			return 0, nil, newError(CodeInvalidRequest, "at least one document, analytical artifact or input source is required")
		}
		if len(req.Documents) > maxDocumentsPerRequest || len(req.AnalyticalArtifacts) > maxArtifactsPerRequest {
			return 0, nil, newError(CodeInvalidRequest, "at most %d documents and %d analytical artifacts per request", maxDocumentsPerRequest, maxArtifactsPerRequest)
		}
		existing, err := e.app.ListDocuments(ctx, subject.ProjectID)
		if err != nil {
			return 0, nil, err
		}
		byRef, byArtifact := map[string]*domain.Document{}, map[string]*domain.Document{}
		for _, d := range existing {
			if ref := d.Metadata[metadataExternalRef]; ref != "" {
				byRef[ref] = d
			}
			if id := d.Metadata[service.MetadataAnalyticalArtifactID]; id != "" {
				byArtifact[id] = d
			}
		}

		receipt := EvidenceReceipt{ContractVersion: ContractVersion, SubjectID: subjectID, Documents: []EvidenceItemReceipt{}, AnalyticalArtifacts: []EvidenceItemReceipt{}}
		var created []*domain.Document
		seen := map[string]bool{}
		for i, in := range req.Documents {
			if err := validateDocument(in); err != nil {
				return 0, nil, newError(CodeInvalidRequest, "documents[%d]: %s", i, err.Message)
			}
			if seen[in.ExternalRef] {
				return 0, nil, newError(CodeInvalidRequest, "documents[%d]: externalRef %q appears more than once", i, in.ExternalRef)
			}
			seen[in.ExternalRef] = true
			if stored, ok := byRef[in.ExternalRef]; ok {
				if !sameDocument(stored, in) {
					return 0, nil, newError(CodeIdentityConflict, "document %q already exists with different content", in.ExternalRef)
				}
				receipt.Documents = append(receipt.Documents, EvidenceItemReceipt{ExternalRef: in.ExternalRef, DocumentID: stored.ID, Status: "UNCHANGED"})
				continue
			}
			metadata := maps.Clone(in.Metadata)
			if metadata == nil {
				metadata = map[string]string{}
			}
			metadata[metadataExternalRef] = in.ExternalRef
			doc := &domain.Document{ID: newID("doc"), ProjectID: subject.ProjectID, Source: domain.SourceType(in.Source), Title: in.Title, Content: in.Content, Metadata: metadata, CreatedAt: e.now()}
			created = append(created, doc)
			receipt.Documents = append(receipt.Documents, EvidenceItemReceipt{ExternalRef: in.ExternalRef, DocumentID: doc.ID, Status: "CREATED"})
		}

		seenArtifacts := map[string]bool{}
		for i, raw := range req.AnalyticalArtifacts {
			artifact, err := analytical.Import(raw)
			if err != nil {
				return 0, nil, newError(CodeInvalidRequest, "analyticalArtifacts[%d]: %v", i, err)
			}
			if seenArtifacts[artifact.ID] {
				return 0, nil, newError(CodeInvalidRequest, "analyticalArtifacts[%d]: artifact %q appears more than once", i, artifact.ID)
			}
			seenArtifacts[artifact.ID] = true
			item := EvidenceItemReceipt{ArtifactID: artifact.ID, ReproducibilityKey: artifact.ReproducibilityKey()}
			if stored, ok := byArtifact[artifact.ID]; ok {
				previous, valid := service.ArtifactFromDocument(stored)
				if !valid {
					return 0, nil, newError(CodeIdentityConflict, "analytical artifact %q already exists and cannot be compared", artifact.ID)
				}
				if _, err := analytical.CheckDuplicate(*previous, *artifact); err != nil {
					return 0, nil, newError(CodeIdentityConflict, "analytical artifact %q already exists with a different artifactHash", artifact.ID)
				}
				item.DocumentID, item.Status = stored.ID, "UNCHANGED"
				receipt.AnalyticalArtifacts = append(receipt.AnalyticalArtifacts, item)
				continue
			}
			doc, err := service.AnalyticalArtifactDocument(subject.ProjectID, *artifact, e.now())
			if err != nil {
				return 0, nil, newError(CodeInvalidRequest, "analyticalArtifacts[%d]: %v", i, err)
			}
			created = append(created, doc)
			item.DocumentID, item.Status = doc.ID, "CREATED"
			receipt.AnalyticalArtifacts = append(receipt.AnalyticalArtifacts, item)
		}
		if len(req.InputSources) > 0 {
			refs, receipts, err := e.addInputSources(ctx, subject.ProjectID, byRef, req.InputSources, seen)
			if err != nil {
				return 0, nil, err
			}
			created = append(created, refs...)
			receipt.InputSources = receipts
		}
		if err := e.documents.CreateBatch(ctx, created); err != nil {
			return 0, nil, err
		}
		return http.StatusOK, receipt, nil
	})
}

func sameDocument(stored *domain.Document, in EvidenceDocument) bool {
	metadata := maps.Clone(stored.Metadata)
	delete(metadata, metadataExternalRef)
	if len(metadata) == 0 {
		metadata = nil
	}
	incoming := in.Metadata
	if len(incoming) == 0 {
		incoming = nil
	}
	return string(stored.Source) == in.Source && stored.Title == in.Title && stored.Content == in.Content && maps.Equal(metadata, incoming)
}

// ---------- analyses ----------

// StartAnalysis enqueues an analysis run over the subject's current evidence.
func (e *Engine) StartAnalysis(ctx context.Context, subjectID string, req StartAnalysisRequest) (int, []byte, error) {
	return e.idempotent(ctx, req.ContractVersion, req.IdempotencyKey, "startAnalysis:"+subjectID, req, func() (int, any, error) {
		subject, err := e.requireSubject(ctx, subjectID)
		if err != nil {
			return 0, nil, err
		}
		mode, err := semanticMode(req.SemanticAnalysisMode)
		if err != nil {
			return 0, nil, err
		}
		if len(req.Label) > 256 || len(req.Note) > 2000 {
			return 0, nil, newError(CodeInvalidRequest, "label is limited to 256 and note to 2000 characters")
		}
		researchQuestion := strings.TrimSpace(req.ResearchQuestion)
		if len(researchQuestion) > maxQuestionLength {
			return 0, nil, newError(CodeInvalidRequest, "researchQuestion is limited to %d characters", maxQuestionLength)
		}
		profile, err := execution.Parse(req.ExecutionProfile)
		if err != nil {
			return 0, nil, newError(CodeInvalidRequest, "%v", err)
		}
		reasoningProfile := domain.ReasoningProfile(req.ReasoningProfile)
		if reasoningProfile != "" && !reasoningProfile.Valid() {
			return 0, nil, newError(CodeInvalidRequest, "unknown reasoningProfile %q", req.ReasoningProfile)
		}
		if len(req.ModelBindings) > 16 {
			return 0, nil, newError(CodeInvalidRequest, "modelBindings is limited to 16 stages")
		}
		analysis, err := e.jobs.Enqueue(ctx, service.EnqueueRequest{ProjectID: subject.ProjectID, Label: strings.TrimSpace(req.Label), Note: strings.TrimSpace(req.Note), SemanticAnalysisMode: mode, ResearchQuestion: researchQuestion, ExecutionProfile: profile, ReasoningProfile: reasoningProfile, ModelBindings: req.ModelBindings})
		if err != nil {
			return 0, nil, err
		}
		return http.StatusAccepted, toAnalysisRun(subjectID, analysis), nil
	})
}

// GetAnalysis returns an analysis run of the subject.
func (e *Engine) GetAnalysis(ctx context.Context, subjectID, analysisID string) (AnalysisRun, error) {
	analysis, err := e.subjectAnalysis(ctx, subjectID, analysisID)
	if err != nil {
		return AnalysisRun{}, err
	}
	return toAnalysisRun(subjectID, analysis), nil
}

func (e *Engine) subjectAnalysis(ctx context.Context, subjectID, analysisID string) (*domain.Analysis, error) {
	subject, err := e.requireSubject(ctx, subjectID)
	if err != nil {
		return nil, err
	}
	if analysisID == "" {
		return nil, newError(CodeNotFound, "analysis not found")
	}
	analysis, err := e.app.ResolveAnalysis(ctx, subject.ProjectID, analysisID)
	if errors.Is(err, usecase.ErrNotFound) {
		return nil, newError(CodeNotFound, "analysis %q not found for subject %q", analysisID, subjectID)
	}
	return analysis, err
}

// GetAnalysisResults returns the observations and findings one completed run
// produced. Results of other runs are never mixed in.
func (e *Engine) GetAnalysisResults(ctx context.Context, subjectID, analysisID string) (AnalysisResults, error) {
	analysis, err := e.subjectAnalysis(ctx, subjectID, analysisID)
	if err != nil {
		return AnalysisResults{}, err
	}
	if analysis.Status != domain.AnalysisCompleted {
		return AnalysisResults{}, newError(CodeAnalysisNotCompleted, "analysis %q is %s", analysisID, analysis.Status)
	}
	observations, err := e.app.ListObservations(ctx, analysis.ProjectID, analysis.ID)
	if err != nil {
		return AnalysisResults{}, err
	}
	documents, err := e.app.ListDocuments(ctx, analysis.ProjectID)
	if err != nil {
		return AnalysisResults{}, err
	}
	refs := map[string]string{}
	for _, d := range documents {
		refs[d.ID] = firstNonEmpty(d.Metadata[metadataExternalRef], d.Metadata[service.MetadataAnalyticalArtifactID])
	}
	patterns, err := e.app.ListPatterns(ctx, analysis.ProjectID, analysis.ID)
	if err != nil {
		return AnalysisResults{}, err
	}
	out := AnalysisResults{ContractVersion: ContractVersion, SubjectID: subjectID, AnalysisID: analysis.ID, Observations: []Observation{}, Findings: []Finding{}}
	for _, o := range observations {
		out.Observations = append(out.Observations, Observation{
			ObservationID: o.ID, DocumentID: o.DocumentID, ExternalRef: refs[o.DocumentID], Quote: o.Quote,
			StartOffset: o.StartOffset, EndOffset: o.EndOffset, Behavior: o.Behavior, Topic: o.Topic,
		})
	}
	for _, detail := range patterns {
		p := detail.Pattern
		ids := append([]string{}, p.ObservationIDs...)
		out.Findings = append(out.Findings, Finding{FindingID: p.ID, Kind: string(p.Kind), Title: p.Title, Description: p.Description, Expectation: p.Expectation, ObservationIDs: ids})
	}
	if json.Valid([]byte(analysis.Metrics)) {
		out.Metrics = json.RawMessage(analysis.Metrics)
	}
	return out, nil
}

func toAnalysisRun(subjectID string, a *domain.Analysis) AnalysisRun {
	run := AnalysisRun{
		ContractVersion: ContractVersion, SubjectID: subjectID, AnalysisID: a.ID, Status: string(a.Status), Error: a.Error,
		Label: a.Label, Note: a.Note, SemanticAnalysisMode: string(a.SemanticAnalysisMode),
		ExecutionFingerprint: a.ExecutionFingerprint, InputFingerprint: a.InputFingerprint,
		CreatedAt: formatTime(a.CreatedAt), StartedAt: formatTimePtr(a.StartedAt), FinishedAt: formatTimePtr(a.FinishedAt),
	}
	run.ResearchQuestion = strings.TrimSpace(a.ResearchQuestion)
	var execution service.ExecutionSnapshot
	if json.Unmarshal([]byte(a.ExecutionSnapshot), &execution) == nil {
		run.ExecutionMode = string(execution.ExecutionMode)
		run.Engine = &EngineBuild{Version: execution.EngineVersion, Commit: execution.GitCommit, Dirty: execution.GitDirty}
		if p := execution.ExecutionProfile; p != nil {
			run.ExecutionProfile = &ExecutionProfileResolution{Requested: string(p.Requested), Resolved: string(p.Resolved), Reason: p.Reason, StrategyVersion: p.StrategyVersion}
		}
		if execution.ReasoningProfileResolution != nil {
			run.ReasoningProfile = string(service.AnalysisReasoningProfile(a))
		}
	}
	provenance := AnalysisProvenance{}
	if json.Valid([]byte(a.ExecutionSnapshot)) {
		provenance.Execution = json.RawMessage(a.ExecutionSnapshot)
	}
	if json.Valid([]byte(a.InputSnapshot)) {
		provenance.Input = json.RawMessage(a.InputSnapshot)
	}
	if provenance.Execution != nil || provenance.Input != nil {
		run.Provenance = &provenance
	}
	return run
}

func analysisResearchQuestion(a *domain.Analysis) string {
	if a == nil {
		return ""
	}
	return strings.TrimSpace(a.ResearchQuestion)
}

// ---------- research ----------

// CreateResearchRun starts research on one completed analysis run.
func (e *Engine) CreateResearchRun(ctx context.Context, subjectID string, req CreateResearchRunRequest) (int, []byte, error) {
	return e.idempotent(ctx, req.ContractVersion, req.IdempotencyKey, "createResearchRun:"+subjectID, req, func() (int, any, error) {
		subject, err := e.requireSubject(ctx, subjectID)
		if err != nil {
			return 0, nil, err
		}
		question := strings.TrimSpace(req.Question)
		if question == "" || len(question) > maxQuestionLength || req.AnalysisID == "" {
			return 0, nil, newError(CodeInvalidRequest, "question (1-%d characters) and analysisId are required", maxQuestionLength)
		}
		mode, err := semanticMode(req.SemanticAnalysisMode)
		if err != nil {
			return 0, nil, err
		}
		analysis, err := e.subjectAnalysis(ctx, subjectID, req.AnalysisID)
		if err != nil {
			return 0, nil, err
		}
		if analysisQuestion := analysisResearchQuestion(analysis); analysisQuestion != "" && analysisQuestion != question {
			return 0, nil, newError(CodeInvalidRequest, "research question does not match the question-conditioned analysis")
		}
		run, err := e.app.CreateResearchRun(ctx, usecase.CreateResearchRunInput{
			ProjectID: subject.ProjectID, AnalysisID: req.AnalysisID, Question: question,
			InputReferences: req.InputReferences, AnalysisMode: mode, ObservationWindow: req.ObservationWindow,
		})
		if err != nil {
			return 0, nil, err
		}
		result, err := e.researchResult(ctx, subjectID, run)
		return http.StatusCreated, result, err
	})
}

// AppendIteration evaluates a research run again on a completed analysis
// run, recording which added evidence was meant to address which gaps.
// Earlier iterations are never modified.
func (e *Engine) AppendIteration(ctx context.Context, researchRunID string, req AppendIterationRequest) (int, []byte, error) {
	return e.idempotent(ctx, req.ContractVersion, req.IdempotencyKey, "appendIteration:"+researchRunID, req, func() (int, any, error) {
		run, err := e.publicResearchRun(ctx, researchRunID)
		if err != nil {
			return 0, nil, err
		}
		if req.AnalysisID == "" || len(req.Question) > maxQuestionLength {
			return 0, nil, newError(CodeInvalidRequest, "analysisId is required and question is limited to %d characters", maxQuestionLength)
		}
		effectiveQuestion := strings.TrimSpace(req.Question)
		if effectiveQuestion == "" {
			effectiveQuestion = run.Question
		}
		analysis, err := e.subjectAnalysis(ctx, run.ProjectID, req.AnalysisID)
		if err != nil {
			return 0, nil, err
		}
		if analysisQuestion := analysisResearchQuestion(analysis); analysisQuestion != "" && analysisQuestion != effectiveQuestion {
			return 0, nil, newError(CodeInvalidRequest, "research question does not match the question-conditioned analysis")
		}
		var references []string
		var links []domain.AddedEvidenceLink
		for i, link := range req.AddedEvidence {
			if strings.TrimSpace(link.Reference) == "" {
				return 0, nil, newError(CodeInvalidRequest, "addedEvidence[%d].reference is required", i)
			}
			references = append(references, link.Reference)
			links = append(links, domain.AddedEvidenceLink{Reference: link.Reference, GapIDs: link.GapIDs, Note: link.Note})
		}
		if _, err := e.app.AppendResearchIteration(ctx, usecase.AppendResearchIterationInput{
			RunID: run.ID, AnalysisID: req.AnalysisID, Question: effectiveQuestion,
			AddedEvidence: references, AddedEvidenceLinks: links, ObservationWindow: req.ObservationWindow,
		}); err != nil {
			return 0, nil, err
		}
		updated, err := e.app.GetResearchRun(ctx, run.ID)
		if err != nil {
			return 0, nil, err
		}
		result, err := e.researchResult(ctx, run.ProjectID, updated)
		return http.StatusCreated, result, err
	})
}

// GetResearchRun returns the latest iteration of a research run.
func (e *Engine) GetResearchRun(ctx context.Context, researchRunID string) (ResearchResult, error) {
	run, err := e.publicResearchRun(ctx, researchRunID)
	if err != nil {
		return ResearchResult{}, err
	}
	return e.researchResult(ctx, run.ProjectID, run)
}

// publicResearchRun loads a research run that belongs to a public subject.
// Research created through the local UI is not reachable here.
func (e *Engine) publicResearchRun(ctx context.Context, researchRunID string) (*domain.ResearchRun, error) {
	run, err := e.app.GetResearchRun(ctx, researchRunID)
	if errors.Is(err, usecase.ErrNotFound) {
		return nil, newError(CodeNotFound, "research run %q not found", researchRunID)
	}
	if err != nil {
		return nil, err
	}
	if _, err := e.public.GetSubject(ctx, run.ProjectID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, newError(CodeNotFound, "research run %q not found", researchRunID)
		}
		return nil, err
	}
	return run, nil
}

func (e *Engine) researchResult(ctx context.Context, subjectID string, run *domain.ResearchRun) (ResearchResult, error) {
	iteration, ok := run.LatestIteration()
	if !ok {
		return ResearchResult{}, fmt.Errorf("research run %s has no iterations", run.ID)
	}
	artifact, err := e.app.GetResearchArtifact(ctx, run.ID)
	if err != nil {
		return ResearchResult{}, err
	}
	encoded, err := json.Marshal(artifact)
	if err != nil {
		return ResearchResult{}, err
	}
	result := ResearchResult{
		ContractVersion: ContractVersion, SubjectID: subjectID, ResearchRunID: run.ID,
		IterationID: iteration.ID, IterationSequence: iteration.Sequence, Artifact: encoded,
	}
	if iteration.AnalysisID != "" {
		analysis, err := e.app.GetAnalysis(ctx, iteration.AnalysisID)
		if err != nil {
			return ResearchResult{}, err
		}
		result.Analysis = toAnalysisRun(subjectID, analysis)
	}
	return result, nil
}

// ---------- validation helpers ----------

func validateSubject(req CreateSubjectRequest) error {
	for name, value := range map[string]string{"subject.namespace": req.Subject.Namespace, "subject.id": req.Subject.ID} {
		if strings.TrimSpace(value) == "" || len(value) > maxIdentifierLength {
			return newError(CodeInvalidRequest, "%s must be 1-%d characters", name, maxIdentifierLength)
		}
	}
	if len(req.Subject.Type) > maxIdentifierLength || len(req.Title) > 256 {
		return newError(CodeInvalidRequest, "subject.type is limited to %d and title to 256 characters", maxIdentifierLength)
	}
	return validateMetadata(req.Metadata, false)
}

func validateDocument(in EvidenceDocument) *Error {
	if strings.TrimSpace(in.ExternalRef) == "" || len(in.ExternalRef) > 256 {
		return newError(CodeInvalidRequest, "externalRef must be 1-256 characters")
	}
	if !domain.SourceType(in.Source).Valid() {
		return newError(CodeInvalidRequest, "source %q is not a supported evidence source", in.Source)
	}
	if strings.TrimSpace(in.Content) == "" || len([]rune(in.Content)) > maxDocumentContentLength {
		return newError(CodeInvalidRequest, "content must be 1-%d characters", maxDocumentContentLength)
	}
	if len(in.Title) > 512 {
		return newError(CodeInvalidRequest, "title is limited to 512 characters")
	}
	if err := validateMetadata(in.Metadata, true); err != nil {
		var contractErr *Error
		errors.As(err, &contractErr)
		return contractErr
	}
	return nil
}

// validateMetadata enforces the bounded opaque metadata rules. Document
// metadata may not use keys reserved for engine-written provenance.
func validateMetadata(metadata map[string]string, document bool) error {
	if len(metadata) > maxMetadataEntries {
		return newError(CodeInvalidRequest, "metadata is limited to %d entries", maxMetadataEntries)
	}
	for key, value := range metadata {
		if !metadataKeyPattern.MatchString(key) {
			return newError(CodeInvalidRequest, "metadata key %q must match %s", key, metadataKeyPattern)
		}
		if len(value) > maxMetadataValueLength {
			return newError(CodeInvalidRequest, "metadata value for %q exceeds %d characters", key, maxMetadataValueLength)
		}
		if document && reservedDocumentKey(key) {
			return newError(CodeInvalidRequest, "metadata key %q is reserved for provenance the engine records itself", key)
		}
	}
	return nil
}

// reservedDocumentKey reports keys the engine writes as measured provenance.
// A consumer must not be able to claim a file hash or an acquisition manifest
// that the engine would then report as if it had verified it.
func reservedDocumentKey(key string) bool {
	return strings.HasPrefix(key, "public_") || strings.HasPrefix(key, "analytical_") ||
		key == service.MetadataDatasetHash || key == service.MetadataAcquisitionManifest
}

func semanticMode(value string) (domain.AnalysisMode, error) {
	mode := domain.AnalysisMode(value)
	if value != "" && !mode.Valid() {
		return "", newError(CodeInvalidRequest, "semanticAnalysisMode %q is not supported", value)
	}
	return mode, nil
}

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// GetResearchTimeline returns the longitudinal read model (#71) of a public
// research run.
func (e *Engine) GetResearchTimeline(ctx context.Context, researchRunID string) (ResearchTimeline, error) {
	run, err := e.publicResearchRun(ctx, researchRunID)
	if err != nil {
		return ResearchTimeline{}, err
	}
	t, err := e.app.GetResearchTimeline(ctx, run.ID)
	if err != nil {
		return ResearchTimeline{}, err
	}
	out := ResearchTimeline{
		ContractVersion: ContractVersion, SubjectID: run.ProjectID, ResearchRunID: t.ResearchRunID, Question: t.Question,
		Iterations: t.Iterations, EvidenceEvents: t.EvidenceEvents, ObservationDeltas: t.ObservationDeltas,
		HypothesisEvents: t.HypothesisEvents, InsightVersions: t.InsightVersions, InstrumentChanges: t.InstrumentChanges,
		ScenarioEvents: t.ScenarioEvents, Limitations: t.Limitations,
	}
	if t.AsOf != nil {
		out.AsOf = formatTime(*t.AsOf)
	}
	return out, nil
}
