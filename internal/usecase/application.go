// Package usecase contains application-specific orchestration.
package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
	"insight-lab/internal/service"
)

// ErrNotFound is the transport-facing sentinel for a missing aggregate.
// Keeping it at this boundary prevents handlers from importing persistence
// ports solely to classify an application error.
var ErrNotFound = repository.ErrNotFound

// Repositories groups the domain repository ports required by Application.
type Repositories struct {
	Projects     repository.ProjectRepository
	Documents    repository.DocumentRepository
	Observations repository.ObservationRepository
	Patterns     repository.PatternRepository
	Analyses     repository.AnalysisRepository
	Insights     repository.InsightRepository
	Evidence     repository.EvidenceRepository
}

// Application implements synchronous user-facing use cases. Transport layers
// depend on this type instead of coordinating repositories directly.
type Application struct {
	repos Repositories
	now   func() time.Time
}

func New(repos Repositories) *Application {
	return &Application{repos: repos, now: func() time.Time { return time.Now().UTC() }}
}

func (a *Application) RequireProject(ctx context.Context, projectID string) error {
	_, err := a.repos.Projects.Get(ctx, projectID)
	return err
}

func (a *Application) ListProjects(ctx context.Context) ([]*domain.Project, error) {
	return a.repos.Projects.List(ctx)
}

func (a *Application) CreateProject(ctx context.Context, name string) (*domain.Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	p := &domain.Project{ID: newID("proj"), Name: name, CreatedAt: a.now()}
	if err := a.repos.Projects.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return p, nil
}

func (a *Application) GetProject(ctx context.Context, id string) (*domain.Project, error) {
	return a.repos.Projects.Get(ctx, id)
}

func (a *Application) DeleteProject(ctx context.Context, id string) error {
	return a.repos.Projects.Delete(ctx, id)
}

func (a *Application) ListDocuments(ctx context.Context, projectID string) ([]*domain.Document, error) {
	if err := a.RequireProject(ctx, projectID); err != nil {
		return nil, err
	}
	return a.repos.Documents.ListByProject(ctx, projectID)
}

type CreateDocumentInput struct {
	ProjectID  string
	Source     domain.SourceType
	Provenance domain.Provenance // empty = domain.DefaultProvenance(Source)
	Title      string
	Content    string
	RawContent string
	Spans      []domain.Span
	Metadata   map[string]string
	// SpeakerRoles are the label->role choices the user confirmed in the
	// intake preview; they are remembered in the project's intake profile
	// so the next transcript gets them as defaults.
	SpeakerRoles map[string]domain.SpeakerRole
	// DetectSpeakers asks the server to derive Spans by parsing Content
	// as a transcript (after masking), instead of trusting client Spans.
	// The UI always uses this so spans can never disagree with the
	// masked content that is stored.
	DetectSpeakers bool
	// SkipMask disables PII masking for this document (masking is on by
	// default; the API caller must opt out explicitly).
	SkipMask bool
}

func (a *Application) CreateDocument(ctx context.Context, in CreateDocumentInput) (*domain.Document, error) {
	project, err := a.repos.Projects.Get(ctx, in.ProjectID)
	if err != nil {
		return nil, err
	}
	if !in.Source.Valid() {
		return nil, fmt.Errorf("invalid source type")
	}
	if strings.TrimSpace(in.Content) == "" {
		return nil, fmt.Errorf("content is required")
	}
	provenance := in.Provenance
	if provenance == "" {
		provenance = domain.DefaultProvenance(in.Source)
	}
	if !provenance.Valid() {
		return nil, fmt.Errorf("invalid provenance")
	}
	content := strings.ReplaceAll(in.Content, "\r\n", "\n")
	raw := in.RawContent
	spans := in.Spans
	if !in.SkipMask {
		if r := service.NewMasker(project.IntakeProfile.MaskTerms).Mask(content); r.Count > 0 {
			content, raw = r.Masked, r.Raw
			if !in.DetectSpeakers && len(spans) > 0 {
				// Client spans index the unmasked text; they cannot be
				// trusted once masking changed the content.
				return nil, fmt.Errorf("content was masked; send detectSpeakers instead of spans, or skipMask")
			}
		}
	}
	if in.DetectSpeakers {
		roles := a.mergedSpeakerRoles(project, in.SpeakerRoles)
		if parsed := service.ParseTranscript(content, roles); parsed.Detected {
			spans = parsed.Spans()
		} else {
			spans = nil
		}
	}
	if err := validateSpans(spans, content); err != nil {
		return nil, err
	}
	d := &domain.Document{ID: newID("doc"), ProjectID: in.ProjectID, Source: in.Source, Provenance: provenance,
		Title: in.Title, Content: content, RawContent: raw, Spans: spans, Metadata: in.Metadata, CreatedAt: a.now()}
	if err := a.repos.Documents.Create(ctx, d); err != nil {
		return nil, fmt.Errorf("create document: %w", err)
	}
	if len(in.SpeakerRoles) > 0 {
		profile := project.IntakeProfile
		profile.MergeSpeakerRoles(in.SpeakerRoles)
		if err := a.repos.Projects.UpdateIntakeProfile(ctx, in.ProjectID, profile); err != nil {
			return nil, fmt.Errorf("remember speaker roles: %w", err)
		}
	}
	return d, nil
}

// mergedSpeakerRoles overlays per-request role choices on the project's
// remembered mapping.
func (a *Application) mergedSpeakerRoles(project *domain.Project, overrides map[string]domain.SpeakerRole) map[string]domain.SpeakerRole {
	roles := map[string]domain.SpeakerRole{}
	for label, role := range project.IntakeProfile.SpeakerRoles {
		roles[label] = role
	}
	for label, role := range overrides {
		if role.Valid() {
			roles[label] = role
		}
	}
	return roles
}

// IntakePreview is what the user sees before a paste becomes a document:
// how the text was split into speakers, what will count as the customer's
// voice, what was left out, and what PII was masked.
type IntakePreview struct {
	Transcript    service.TranscriptParse
	Spans         []domain.Span
	Provenance    domain.Provenance
	CustomerChars int
	ExcludedChars int
	TotalChars    int
	// Masked is the content after PII masking - what would be stored and
	// what the turns index.
	Masked      string
	MaskCount   int
	MaskByKind  map[service.MaskKind]int
	MaskSkipped bool
}

type PreviewIntakeInput struct {
	ProjectID    string
	Source       domain.SourceType
	Provenance   domain.Provenance
	Content      string
	SpeakerRoles map[string]domain.SpeakerRole // overrides on top of the project profile
	SkipMask     bool
}

func (a *Application) PreviewIntake(ctx context.Context, in PreviewIntakeInput) (*IntakePreview, error) {
	project, err := a.repos.Projects.Get(ctx, in.ProjectID)
	if err != nil {
		return nil, err
	}
	if !in.Source.Valid() {
		return nil, fmt.Errorf("invalid source type")
	}
	if strings.TrimSpace(in.Content) == "" {
		return nil, fmt.Errorf("content is required")
	}
	roles := a.mergedSpeakerRoles(project, in.SpeakerRoles)

	provenance := in.Provenance
	if provenance == "" {
		provenance = domain.DefaultProvenance(in.Source)
	}
	if !provenance.Valid() {
		return nil, fmt.Errorf("invalid provenance")
	}

	content := strings.ReplaceAll(in.Content, "\r\n", "\n")
	preview := &IntakePreview{Provenance: provenance, MaskByKind: map[service.MaskKind]int{}, MaskSkipped: in.SkipMask}
	if !in.SkipMask {
		r := service.NewMasker(project.IntakeProfile.MaskTerms).Mask(content)
		content = r.Masked
		preview.MaskCount = r.Count
		preview.MaskByKind = r.ByKind
	}
	preview.Masked = content
	preview.Transcript = service.ParseTranscript(content, roles)
	preview.TotalChars = len([]rune(content))
	if preview.Transcript.Detected {
		preview.Spans = preview.Transcript.Spans()
		for _, t := range preview.Transcript.Turns {
			n := len([]rune(t.Text))
			if t.Role == domain.RoleCustomer {
				preview.CustomerChars += n
			} else {
				preview.ExcludedChars += n
			}
		}
	} else {
		preview.CustomerChars = preview.TotalChars
	}
	return preview, nil
}

func (a *Application) GetIntakeProfile(ctx context.Context, projectID string) (*domain.IntakeProfile, error) {
	project, err := a.repos.Projects.Get(ctx, projectID)
	if err != nil {
		return nil, err
	}
	profile := project.IntakeProfile
	return &profile, nil
}

func (a *Application) UpdateIntakeProfile(ctx context.Context, projectID string, profile domain.IntakeProfile) error {
	for label, role := range profile.SpeakerRoles {
		if !role.Valid() {
			return fmt.Errorf("invalid role %q for speaker %q", role, label)
		}
	}
	if profile.ColumnMapping != nil && profile.ColumnMapping.DefaultSource != "" && !profile.ColumnMapping.DefaultSource.Valid() {
		return fmt.Errorf("invalid default source")
	}
	return a.repos.Projects.UpdateIntakeProfile(ctx, projectID, profile)
}

func (a *Application) GetDocument(ctx context.Context, id string) (*domain.Document, error) {
	return a.repos.Documents.Get(ctx, id)
}

// ImportDocumentsCSV imports the fixed id,source,title,content layout.
func (a *Application) ImportDocumentsCSV(ctx context.Context, projectID string, r io.Reader) (*service.ImportResult, error) {
	if err := a.RequireProject(ctx, projectID); err != nil {
		return nil, err
	}
	return service.ImportCSV(ctx, a.repos.Documents, projectID, r)
}

// PreviewImport parses a spreadsheet export and proposes a column
// mapping; the project's remembered mapping, if any, is returned
// alongside so the UI can prefer it.
func (a *Application) PreviewImport(ctx context.Context, projectID string, r io.Reader) (*service.TablePreview, *domain.ColumnMapping, error) {
	project, err := a.repos.Projects.Get(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	preview, err := service.PreviewTable(r)
	if err != nil {
		return nil, nil, err
	}
	return preview, project.IntakeProfile.ColumnMapping, nil
}

// ImportDocumentsTable imports any CSV/TSV under an explicit mapping and
// remembers the mapping in the project's intake profile. Cells that look
// like transcripts get speaker spans using the profile's speaker roles.
func (a *Application) ImportDocumentsTable(ctx context.Context, projectID string, r io.Reader, mapping domain.ColumnMapping) (*service.ImportResult, error) {
	project, err := a.repos.Projects.Get(ctx, projectID)
	if err != nil {
		return nil, err
	}
	table, err := service.ParseTable(r)
	if err != nil {
		return nil, err
	}
	result, err := service.ImportTable(ctx, a.repos.Documents, projectID, table, mapping, service.ImportOptions{
		SpeakerRoles: project.IntakeProfile.SpeakerRoles,
		Masker:       service.NewMasker(project.IntakeProfile.MaskTerms).MaskFunc(),
	})
	if err != nil {
		return nil, err
	}
	profile := project.IntakeProfile
	m := mapping
	profile.ColumnMapping = &m
	if err := a.repos.Projects.UpdateIntakeProfile(ctx, projectID, profile); err != nil {
		return nil, fmt.Errorf("remember column mapping: %w", err)
	}
	return result, nil
}

func (a *Application) GetAnalysis(ctx context.Context, id string) (*domain.Analysis, error) {
	return a.repos.Analyses.Get(ctx, id)
}

func (a *Application) ListAnalyses(ctx context.Context, projectID string) ([]*domain.Analysis, error) {
	if err := a.RequireProject(ctx, projectID); err != nil {
		return nil, err
	}
	return a.repos.Analyses.ListByProject(ctx, projectID)
}

func (a *Application) LatestAnalysis(ctx context.Context, projectID string) (*domain.Analysis, error) {
	if err := a.RequireProject(ctx, projectID); err != nil {
		return nil, err
	}
	return a.repos.Analyses.LatestByProject(ctx, projectID)
}

type InsightDetail struct {
	Insight  *domain.Insight
	Evidence []*domain.Evidence
	Patterns []PatternDetail
}

type PatternDetail struct {
	Pattern      *domain.Pattern
	Observations []*domain.Observation
}

func (a *Application) ListInsights(ctx context.Context, projectID string) ([]*domain.Insight, error) {
	if err := a.RequireProject(ctx, projectID); err != nil {
		return nil, err
	}
	return a.repos.Insights.ListByProject(ctx, projectID)
}

func (a *Application) GetInsight(ctx context.Context, id string) (*InsightDetail, error) {
	insight, err := a.repos.Insights.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	evidence, err := a.repos.Evidence.ListByInsight(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list insight evidence: %w", err)
	}
	patterns, err := a.repos.Patterns.ListByInsight(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list insight patterns: %w", err)
	}
	details, err := a.resolvePatterns(ctx, patterns)
	if err != nil {
		return nil, err
	}
	return &InsightDetail{Insight: insight, Evidence: evidence, Patterns: details}, nil
}

func (a *Application) GetInsightEvidence(ctx context.Context, id string) ([]*domain.Evidence, error) {
	if _, err := a.repos.Insights.Get(ctx, id); err != nil {
		return nil, err
	}
	return a.repos.Evidence.ListByInsight(ctx, id)
}

func (a *Application) ListPatterns(ctx context.Context, projectID string) ([]PatternDetail, error) {
	if err := a.RequireProject(ctx, projectID); err != nil {
		return nil, err
	}
	patterns, err := a.repos.Patterns.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return a.resolvePatterns(ctx, patterns)
}

func (a *Application) resolvePatterns(ctx context.Context, patterns []*domain.Pattern) ([]PatternDetail, error) {
	var ids []string
	for _, p := range patterns {
		ids = append(ids, p.ObservationIDs...)
	}
	observations, err := a.repos.Observations.ListByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("resolve pattern observations: %w", err)
	}
	byID := make(map[string]*domain.Observation, len(observations))
	for _, observation := range observations {
		byID[observation.ID] = observation
	}
	result := make([]PatternDetail, 0, len(patterns))
	for _, p := range patterns {
		detail := PatternDetail{Pattern: p}
		for _, id := range p.ObservationIDs {
			if observation := byID[id]; observation != nil {
				detail.Observations = append(detail.Observations, observation)
			}
		}
		result = append(result, detail)
	}
	return result, nil
}

// validateSpans rejects speaker spans that fall outside the content or
// carry an unknown role. Overlaps are tolerated (a later intake step may
// produce nested attributions); out-of-range offsets are not, because
// grounding and highlighting both index Content by these numbers.
func validateSpans(spans []domain.Span, content string) error {
	n := len([]rune(content))
	for i, s := range spans {
		if !s.Role.Valid() {
			return fmt.Errorf("spans[%d]: invalid role %q", i, s.Role)
		}
		if s.Start < 0 || s.End > n || s.End <= s.Start {
			return fmt.Errorf("spans[%d]: offsets %d-%d out of range (content is %d runes)", i, s.Start, s.End, n)
		}
	}
	return nil
}

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}
