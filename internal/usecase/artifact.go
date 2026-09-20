package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/service"
)

// ResearchArtifactSchema and ResearchArtifactVersion identify the shape of
// ResearchArtifact for a downstream consumer (Issue #17). A consumer must
// check these before parsing rather than assume the shape it was built
// against; bumping ResearchArtifactVersion is a breaking change, adding an
// omitempty field to the current shape is not.
const (
	ResearchArtifactSchema  = "insight-lab.research-artifact"
	ResearchArtifactVersion = "1"
)

// ResearchArtifact is the versioned, machine-consumable snapshot of a
// ResearchRun's latest iteration. It exists so a downstream system never has
// to parse report.md to learn what a research run found: every value here is
// either copied verbatim from append-only domain history or derived
// deterministically from it.
//
// ResearchStage (below) and per-insight Expectation provenance
// (ArtifactInsight.ExpectationBasis) are already exported. Stage transition
// history (Issue #22) is the remaining field the domain does not yet track
// per iteration; it is intentionally absent rather than guessed and can be
// added additively once that wiring exists, without a schema version bump.
type ResearchArtifact struct {
	Promotion          domain.PromotionAssessment `json:"promotion"`
	PromotionGateInput domain.PromotionGateInput  `json:"promotionGateInput"`
	ArtifactSchema     string                     `json:"artifactSchema"`
	SchemaVersion      string                     `json:"schemaVersion"`

	ProjectID         string `json:"projectId"`
	ResearchRunID     string `json:"researchRunId"`
	IterationID       string `json:"iterationId"`
	IterationSequence int    `json:"iterationSequence"`
	IterationCount    int    `json:"iterationCount"`
	ResearchQuestion  string `json:"researchQuestion"`

	// ResearchStage is the latest iteration's own recorded stage (Issue #37),
	// copied verbatim so a downstream consumer never has to re-derive it from
	// iteration history the way domain.ResearchRun.CurrentStage does.
	ResearchStage        domain.ResearchStage `json:"researchStage,omitempty"`
	SemanticAnalysisMode domain.AnalysisMode  `json:"semanticAnalysisMode,omitempty"`

	// ExecutionMode, ModelVersion, PromptFingerprint and RuleVersion come from
	// the latest analysis's RunProvenance, so a no-model (deterministic-only)
	// run can never be mistaken for a model-backed one.
	//
	// The Go field is named ExecutionMode (Issue #38) because that is what
	// this value actually is: whether a model executed, not the semantic
	// input-reading mode (DISCOVERY / DATASET_ANALYSIS / RESEARCH_REVIEW)
	// Issue #18 will add. The JSON key stays "analysisMode" because it is
	// already part of the versioned v1 wire contract (Issue #17); renaming
	// the wire field would be a breaking change. When #18 lands, it must
	// introduce its own field rather than repurpose this one.
	ExecutionMode     service.ExecutionMode `json:"analysisMode,omitempty"`
	ModelVersion      string                `json:"modelVersion,omitempty"`
	PromptFingerprint string                `json:"promptFingerprint,omitempty"`
	RuleVersion       string                `json:"ruleVersion,omitempty"`

	InputReferences              []string                              `json:"inputReferences,omitempty"`
	AcquisitionManifests         []service.DatasetProvenance           `json:"acquisitionManifests,omitempty"`
	DatasetHashes                []string                              `json:"datasetHashes,omitempty"`
	DatasetCompatibilityWarnings []service.DatasetCompatibilityWarning `json:"datasetCompatibilityWarnings,omitempty"`
	DeterministicNotes           []string                              `json:"deterministicNotes,omitempty"`

	Insights []ArtifactInsight `json:"insights"`

	ResearchGaps         []domain.ResearchGap                  `json:"researchGaps,omitempty"`
	NextDataRequirements []domain.DataRequirement              `json:"nextDataRequirements,omitempty"`
	Expectations         []domain.Expectation                  `json:"expectations,omitempty"`
	HypothesisStates     []domain.HypothesisState              `json:"hypothesisStates,omitempty"`
	HypothesisHistory    []domain.HypothesisChange             `json:"hypothesisHistory,omitempty"`
	ValidationEvidence   []domain.ValidationEvidenceProvenance `json:"validationEvidence,omitempty"`
	WhatWeCannotConclude []string                              `json:"whatWeCannotConclude,omitempty"`
	AddedEvidence        []string                              `json:"addedEvidence,omitempty"`
	AddedEvidenceLinks   []domain.AddedEvidenceLink            `json:"addedEvidenceLinks,omitempty"`
	InputSnapshot        domain.InputSetSnapshot               `json:"inputSnapshot,omitempty"`
	Claims               []domain.ResearchClaim                `json:"claims,omitempty"`
	InsightDelta         *domain.InsightDelta                  `json:"insightDelta,omitempty"`

	Readiness          domain.ReadinessAssessment `json:"decisionReadiness"`
	EffectiveReadiness domain.DecisionReadiness   `json:"effectiveDecisionReadiness"`
	StopDecision       *domain.StopDecision       `json:"stopDecision,omitempty"`
	HumanOverrides     []domain.HumanOverride     `json:"humanOverrides,omitempty"`
	HumanEvaluation    *domain.HumanEvaluation    `json:"humanEvaluation,omitempty"`

	CreatedAt  time.Time `json:"iterationCreatedAt"`
	ExportedAt time.Time `json:"exportedAt"`
}

// ArtifactInsight is the export-facing subset of an Insight and its
// evidence: enough for a downstream consumer to see the hypothesis, its
// validation/identification status, and which evidence supports, contradicts
// or is neutral to it, without re-deriving that split itself.
type ArtifactInsight struct {
	ID                        string                       `json:"id"`
	Title                     string                       `json:"title"`
	Observation               string                       `json:"observation,omitempty"`
	StatedNeed                string                       `json:"statedNeed,omitempty"`
	LatentNeed                string                       `json:"latentNeed,omitempty"`
	Expectation               string                       `json:"expectation,omitempty"`
	ExpectationBasis          domain.ExpectationBasis      `json:"expectationBasis,omitempty"`
	SurprisingFact            string                       `json:"surprisingFact,omitempty"`
	Rationale                 string                       `json:"rationale,omitempty"`
	AlternativeInterpretation string                       `json:"alternativeInterpretation,omitempty"`
	Connection                domain.InsightConnection     `json:"connection,omitempty"`
	Mechanism                 domain.MechanismCandidate    `json:"mechanism,omitempty"`
	Generalization            domain.InsightGeneralization `json:"generalization,omitempty"`
	HypothesisSetID           string                       `json:"hypothesisSetId,omitempty"`
	HypothesisRole            domain.HypothesisRole        `json:"hypothesisRole,omitempty"`
	CausalStatus              domain.CausalStatus          `json:"causalStatus,omitempty"`
	ValidationStatus          domain.ValidationStatus      `json:"validationStatus,omitempty"`
	IdentificationStatus      domain.IdentificationStatus  `json:"identificationStatus,omitempty"`
	CompetingHypotheses       []domain.CompetingHypothesis `json:"competingHypotheses,omitempty"`
	MissingEvidence           []string                     `json:"missingEvidence,omitempty"`
	FalsificationCriteria     []string                     `json:"falsificationCriteria,omitempty"`
	QualityWarnings           []domain.QualityFlag         `json:"qualityWarnings,omitempty"`
	SupportingEvidence        []domain.Evidence            `json:"supportingEvidence,omitempty"`
	CounterEvidence           []domain.Evidence            `json:"counterEvidence,omitempty"`
	NeutralEvidence           []domain.Evidence            `json:"neutralEvidence,omitempty"`
}

// GetResearchArtifact builds the versioned JSON export of runID's latest
// iteration (Issue #17). It is the stable machine-consumption contract:
// downstream systems must consume this instead of parsing report.md.
func (a *Application) GetResearchArtifact(ctx context.Context, runID string) (*ResearchArtifact, error) {
	run, err := a.repos.Research.GetResearchRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	iteration, ok := run.LatestIteration()
	if !ok {
		return nil, fmt.Errorf("research run %s has no iterations", runID)
	}

	insights, err := a.artifactInsights(ctx, iteration.InsightIDs)
	if err != nil {
		return nil, err
	}

	artifact := &ResearchArtifact{
		Promotion: iteration.Promotion, PromotionGateInput: iteration.PromotionGateInput,
		ArtifactSchema: ResearchArtifactSchema, SchemaVersion: ResearchArtifactVersion,
		ProjectID: run.ProjectID, ResearchRunID: run.ID, IterationID: iteration.ID,
		IterationSequence: iteration.Sequence, IterationCount: len(run.Iterations),
		ResearchQuestion: run.Question, ResearchStage: iteration.Stage, SemanticAnalysisMode: iteration.AnalysisMode, InputReferences: iteration.InputReferences,
		Insights:             insights,
		ResearchGaps:         iteration.ResearchGaps,
		NextDataRequirements: iteration.DataRequirements,
		Expectations:         iteration.Expectations,
		HypothesisStates:     iteration.HypothesisStates,
		HypothesisHistory:    iteration.HypothesisChanges,
		ValidationEvidence:   iteration.ValidationEvidence,
		WhatWeCannotConclude: iteration.WhatWeCannotConclude,
		AddedEvidence:        iteration.AddedEvidence,
		AddedEvidenceLinks:   iteration.AddedEvidenceLinks,
		InputSnapshot:        iteration.InputSnapshot,
		Claims:               iteration.Claims,
		InsightDelta:         iteration.Delta,
		Readiness:            iteration.Readiness, EffectiveReadiness: iteration.EffectiveReadiness(),
		StopDecision: iteration.Stop, HumanOverrides: iteration.HumanOverrides,
		CreatedAt: iteration.CreatedAt, ExportedAt: a.now(),
	}

	if evaluation, err := a.repos.Research.GetHumanEvaluation(ctx, run.ID, iteration.ID); err == nil {
		artifact.HumanEvaluation = evaluation
	}

	if metrics, ok := a.iterationMetrics(ctx, run.ProjectID, iteration); ok {
		prov := metrics.Provenance
		artifact.ExecutionMode = prov.Mode
		artifact.ModelVersion = prov.Model
		artifact.PromptFingerprint = prov.PromptFingerprint
		artifact.RuleVersion = prov.RuleVersion
		artifact.DatasetHashes = prov.DatasetHashes
		artifact.AcquisitionManifests = prov.Datasets
		artifact.DatasetCompatibilityWarnings = prov.CompatibilityWarnings
		artifact.DeterministicNotes = prov.Notes
	}

	return artifact, nil
}

// latestRunProvenance decodes the RunProvenance recorded on the project's
// latest analysis. ok is false when there is no completed analysis or it
// predates provenance tracking, so callers leave those fields empty rather
// than exporting zero values that look like a deterministic-only run.
func (a *Application) latestRunProvenance(ctx context.Context, projectID string) (service.RunProvenance, bool) {
	analysis, err := a.repos.Analyses.LatestByProject(ctx, projectID)
	if err != nil || analysis.Metrics == "" {
		return service.RunProvenance{}, false
	}
	var metrics service.Metrics
	if err := json.Unmarshal([]byte(analysis.Metrics), &metrics); err != nil || metrics.Provenance.RuleVersion == "" && metrics.Provenance.Mode == "" {
		return service.RunProvenance{}, false
	}
	return metrics.Provenance, true
}

func (a *Application) artifactInsights(ctx context.Context, insightIDs []string) ([]ArtifactInsight, error) {
	out := make([]ArtifactInsight, 0, len(insightIDs))
	for _, id := range insightIDs {
		detail, err := a.GetInsight(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("resolve artifact insight %s: %w", id, err)
		}
		i := detail.Insight
		artifactInsight := ArtifactInsight{
			ID: i.ID, Title: i.Title, Observation: i.Observation, StatedNeed: i.StatedNeed, LatentNeed: i.LatentNeed,
			Expectation: i.Expectation, ExpectationBasis: i.ExpectationBasis, SurprisingFact: i.SurprisingFact,
			Rationale: i.Rationale, AlternativeInterpretation: i.AlternativeInterpretation,
			Connection: i.Connection, Mechanism: i.Mechanism, Generalization: i.Generalization,
			HypothesisSetID: i.HypothesisSetID, HypothesisRole: i.HypothesisRole, CausalStatus: i.CausalStatus,
			ValidationStatus: i.ValidationStatus, IdentificationStatus: i.IdentificationStatus,
			CompetingHypotheses: i.CompetingHypotheses, MissingEvidence: i.MissingEvidence,
			FalsificationCriteria: i.FalsificationCriteria, QualityWarnings: i.QualityFlags,
		}
		for _, evidence := range detail.Evidence {
			switch evidence.Type {
			case domain.EvidenceSupport:
				artifactInsight.SupportingEvidence = append(artifactInsight.SupportingEvidence, *evidence)
			case domain.EvidenceCounter:
				artifactInsight.CounterEvidence = append(artifactInsight.CounterEvidence, *evidence)
			default:
				artifactInsight.NeutralEvidence = append(artifactInsight.NeutralEvidence, *evidence)
			}
		}
		out = append(out, artifactInsight)
	}
	return out, nil
}

// GetApprovedResearchArtifact returns persisted reviewed bytes, never a regenerated export.
func (a *Application) GetApprovedResearchArtifact(ctx context.Context, runID string) (*domain.ApprovedResearchArtifact, error) {
	run, err := a.repos.Research.GetResearchRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	iteration, ok := run.LatestIteration()
	if !ok || (iteration.Promotion.State != domain.PromotionPublicationReady && iteration.Promotion.State != domain.PromotionPublished) || iteration.ApprovedArtifact == nil {
		return nil, fmt.Errorf("latest iteration has no approved research artifact")
	}
	return iteration.ApprovedArtifact, nil
}

// iterationMetrics binds provenance to the analysis that produced this iteration's
// insights. A later project analysis must never silently change an older report.
func (a *Application) iterationMetrics(ctx context.Context, projectID string, iteration domain.ResearchIteration) (service.Metrics, bool) {
	var analysisID string
	for _, id := range iteration.InsightIDs {
		insight, err := a.repos.Insights.Get(ctx, id)
		if err != nil || insight.ProjectID != projectID || insight.AnalysisID == nil || *insight.AnalysisID == "" {
			return service.Metrics{}, false
		}
		if analysisID != "" && analysisID != *insight.AnalysisID {
			return service.Metrics{}, false
		}
		analysisID = *insight.AnalysisID
	}
	if analysisID == "" {
		return service.Metrics{}, false
	}
	analysis, err := a.repos.Analyses.Get(ctx, analysisID)
	if err != nil || analysis.ProjectID != projectID || analysis.Status != domain.AnalysisCompleted {
		return service.Metrics{}, false
	}
	var metrics service.Metrics
	if json.Unmarshal([]byte(analysis.Metrics), &metrics) != nil {
		return service.Metrics{}, false
	}
	return metrics, true
}
