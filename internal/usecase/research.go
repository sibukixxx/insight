package usecase

import (
	"context"
	"fmt"
	"strings"

	"insight-lab/internal/domain"
	"insight-lab/internal/service"
)

type CreateResearchRunInput struct {
	ProjectID       string
	Question        string
	InputReferences []string
	AnalysisMode    domain.AnalysisMode
	InputSnapshot   domain.InputSetSnapshot
	Claims          []domain.ResearchClaim
}

type AppendResearchIterationInput struct {
	RunID              string
	Question           string
	InputReferences    []string
	AddedEvidence      []string
	AddedEvidenceLinks []domain.AddedEvidenceLink
	InputSnapshot      domain.InputSetSnapshot
	AnalysisMode       domain.AnalysisMode
	Claims             []domain.ResearchClaim
}

func (a *Application) CreateResearchRun(ctx context.Context, in CreateResearchRunInput) (*domain.ResearchRun, error) {
	if a.repos.Research == nil {
		return nil, fmt.Errorf("research repository is not configured")
	}
	if err := a.RequireProject(ctx, in.ProjectID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Question) == "" {
		return nil, fmt.Errorf("question is required")
	}
	insights, err := a.latestInsights(ctx, in.ProjectID)
	if err != nil {
		return nil, err
	}
	now := a.now()
	mode := in.AnalysisMode.Normalize()
	if err := domain.ValidateAnalysisModeInput(mode, in.InputSnapshot.Artifacts, in.Claims); err != nil {
		return nil, err
	}
	iteration := service.BuildResearchIteration(1, in.Question, in.InputReferences, insights, now)
	iteration.AnalysisMode = mode
	if len(in.InputSnapshot.ArtifactReferences) == 0 {
		in.InputSnapshot.ArtifactReferences = append([]string(nil), in.InputReferences...)
	}
	iteration.InputSnapshot = in.InputSnapshot
	iteration.Claims = append([]domain.ResearchClaim(nil), in.Claims...)
	iteration = service.FinalizeResearchIteration(domain.ResearchRun{}, iteration, nil, now)
	run := &domain.ResearchRun{ID: newID("run"), ProjectID: in.ProjectID, Question: strings.TrimSpace(in.Question), Iterations: []domain.ResearchIteration{iteration}, CreatedAt: now}
	if err := a.repos.Research.CreateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("create research run: %w", err)
	}
	return run, nil
}

func (a *Application) AppendResearchIteration(ctx context.Context, in AppendResearchIterationInput) (*domain.ResearchIteration, error) {
	run, err := a.repos.Research.GetResearchRun(ctx, in.RunID)
	if err != nil {
		return nil, err
	}
	insights, err := a.latestInsights(ctx, run.ProjectID)
	if err != nil {
		return nil, err
	}
	question := strings.TrimSpace(in.Question)
	if question == "" {
		question = run.Question
	}
	now := a.now()
	mode := in.AnalysisMode
	if mode == "" {
		if latest, ok := run.LatestIteration(); ok && latest.AnalysisMode != "" {
			mode = latest.AnalysisMode
		}
	}
	mode = mode.Normalize()
	if err := domain.ValidateAnalysisModeInput(mode, in.InputSnapshot.Artifacts, in.Claims); err != nil {
		return nil, err
	}
	iteration := service.BuildResearchIteration(len(run.Iterations)+1, question, in.InputReferences, insights, now)
	iteration.AnalysisMode = mode
	iteration.Claims = append([]domain.ResearchClaim(nil), in.Claims...)
	snapshot := in.InputSnapshot
	if len(snapshot.ArtifactReferences) == 0 {
		snapshot.ArtifactReferences = append([]string(nil), in.InputReferences...)
	}
	if len(snapshot.EvidenceReferences) == 0 {
		snapshot.EvidenceReferences = append([]string(nil), in.AddedEvidence...)
		for _, link := range in.AddedEvidenceLinks {
			if strings.TrimSpace(link.Reference) != "" {
				snapshot.EvidenceReferences = append(snapshot.EvidenceReferences, link.Reference)
			}
		}
	}
	iteration.InputSnapshot = snapshot
	iteration = service.FinalizeResearchIterationWithLinks(*run, iteration, in.AddedEvidence, in.AddedEvidenceLinks, now)
	if err := a.repos.Research.AppendResearchIteration(ctx, run.ID, iteration); err != nil {
		return nil, fmt.Errorf("append research iteration: %w", err)
	}
	return &iteration, nil
}

// ApplyResearchHumanOverrideInput carries an explicit human decision about an
// iteration's readiness or about stopping the research loop. Only the latest
// iteration of a run may be overridden, because the run's stop state is
// always read from its latest iteration.
type ApplyResearchHumanOverrideInput struct {
	RunID       string
	IterationID string
	Readiness   domain.DecisionReadiness
	StopReason  domain.ResearchStopReason
	Note        string
}

func (a *Application) ApplyResearchHumanOverride(ctx context.Context, in ApplyResearchHumanOverrideInput) (*domain.ResearchIteration, error) {
	run, err := a.repos.Research.GetResearchRun(ctx, in.RunID)
	if err != nil {
		return nil, err
	}
	latest, ok := run.LatestIteration()
	if !ok || latest.ID != in.IterationID {
		return nil, fmt.Errorf("human override can only be applied to the latest research iteration")
	}
	updated, err := latest.ApplyHumanOverride(domain.HumanOverride{
		Readiness: in.Readiness, StopReason: in.StopReason, Note: strings.TrimSpace(in.Note), RecordedAt: a.now(),
	})
	if err != nil {
		return nil, err
	}
	if err := a.repos.Research.UpdateResearchIteration(ctx, run.ID, updated); err != nil {
		return nil, fmt.Errorf("update research iteration: %w", err)
	}
	return &updated, nil
}

// FreezeResearchExpectationInput identifies which expectation on the latest
// iteration a human is fixing for validation.
type FreezeResearchExpectationInput struct {
	RunID         string
	IterationID   string
	ExpectationID string
}

// FreezeResearchExpectation fixes one of the latest iteration's expectations
// so it can be used as a VALIDATION-stage target. Like ApplyResearchHumanOverride,
// only the latest iteration may be changed, and freezing never rewrites the
// expectation's provenance (domain.Expectation.Freeze enforces this).
func (a *Application) FreezeResearchExpectation(ctx context.Context, in FreezeResearchExpectationInput) (*domain.ResearchIteration, error) {
	run, err := a.repos.Research.GetResearchRun(ctx, in.RunID)
	if err != nil {
		return nil, err
	}
	latest, ok := run.LatestIteration()
	if !ok || latest.ID != in.IterationID {
		return nil, fmt.Errorf("expectation can only be frozen on the latest research iteration")
	}
	idx := -1
	for i, e := range latest.Expectations {
		if e.ID == in.ExpectationID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, fmt.Errorf("expectation %q not found on iteration %q", in.ExpectationID, in.IterationID)
	}
	frozen, err := latest.Expectations[idx].Freeze(a.now())
	if err != nil {
		return nil, err
	}
	updated := latest
	updated.Expectations = append([]domain.Expectation(nil), latest.Expectations...)
	updated.Expectations[idx] = frozen
	if err := a.repos.Research.UpdateResearchIteration(ctx, run.ID, updated); err != nil {
		return nil, fmt.Errorf("update research iteration: %w", err)
	}
	return &updated, nil
}

// TransitionResearchStageInput carries an explicit human request to move a
// research iteration's stage. Like ApplyResearchHumanOverride, only the
// latest iteration of a run may transition, because the run's current stage
// is always read from its latest iteration.
type TransitionResearchStageInput struct {
	RunID                      string
	IterationID                string
	TargetStage                domain.ResearchStage
	// Deprecated compatibility flag. New callers should supply ValidationEvidence
	// so the reason for VALIDATION remains auditable after the transition.
	IndependentEvidencePlanned bool
	ValidationEvidence         []domain.ValidationEvidenceProvenance
}

// TransitionResearchStage moves the latest iteration to TargetStage after
// checking domain.ResearchStage.Transition's requirements. Stage never
// advances on its own: forward moves require this explicit call, and a move
// into VALIDATION is rejected unless at least one of the iteration's
// Expectations is frozen for validation (see FreezeResearchExpectation) and
// every frozen one still validates.
func (a *Application) TransitionResearchStage(ctx context.Context, in TransitionResearchStageInput) (*domain.ResearchIteration, error) {
	run, err := a.repos.Research.GetResearchRun(ctx, in.RunID)
	if err != nil {
		return nil, err
	}
	latest, ok := run.LatestIteration()
	if !ok || latest.ID != in.IterationID {
		return nil, fmt.Errorf("stage transition can only be applied to the latest research iteration")
	}
	completedValidations := 0
	for _, iteration := range run.Iterations {
		if iteration.Stage == domain.StageValidation {
			completedValidations++
		}
	}
	validationEvidence := append([]domain.ValidationEvidenceProvenance(nil), in.ValidationEvidence...)
	if in.TargetStage == domain.StageValidation {
		for idx := range validationEvidence {
			if validationEvidence[idx].RecordedAt.IsZero() {
				validationEvidence[idx].RecordedAt = a.now()
			}
			if strings.TrimSpace(validationEvidence[idx].Actor) == "" {
				validationEvidence[idx].Actor = "HUMAN"
			}
			if err := validationEvidence[idx].Validate(latest.ID, latest.Expectations); err != nil {
				return nil, err
			}
		}
	}
	if err := latest.Stage.Transition(in.TargetStage, domain.StageTransitionInput{
		ObservationCount:           len(latest.ObservationIDs),
		Expectations:               latest.Expectations,
		IndependentEvidencePlanned: len(validationEvidence) > 0 || in.IndependentEvidencePlanned,
		CompletedValidationCount:   completedValidations,
	}); err != nil {
		return nil, err
	}
	updated := latest
	updated.Stage = in.TargetStage
	if in.TargetStage == domain.StageValidation && len(validationEvidence) > 0 {
		updated.ValidationEvidence = append([]domain.ValidationEvidenceProvenance(nil), validationEvidence...)
	}
	if err := a.repos.Research.UpdateResearchIteration(ctx, run.ID, updated); err != nil {
		return nil, fmt.Errorf("update research iteration: %w", err)
	}
	return &updated, nil
}

// GetHumanHandoff assembles the final shape handed to a human for a research
// run: strongest surviving hypotheses, contradicted and unresolved
// alternatives, remaining uncertainty, why the loop stopped (if it has), and
// what to research next. It never acquires evidence.
func (a *Application) GetHumanHandoff(ctx context.Context, runID string) (*domain.HumanHandoff, error) {
	run, err := a.repos.Research.GetResearchRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	handoff := service.BuildHumanHandoff(*run)
	return &handoff, nil
}

func (a *Application) latestInsights(ctx context.Context, projectID string) ([]*domain.Insight, error) {
	analysis, err := a.repos.Analyses.LatestByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("latest analysis: %w", err)
	}
	if analysis.Status != domain.AnalysisCompleted {
		return nil, fmt.Errorf("latest analysis has not completed")
	}
	all, err := a.repos.Insights.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Insight, 0, len(all))
	for _, insight := range all {
		if insight.AnalysisID != nil && *insight.AnalysisID == analysis.ID {
			result = append(result, insight)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("latest analysis has no insights")
	}
	return result, nil
}

func (a *Application) GetResearchRun(ctx context.Context, id string) (*domain.ResearchRun, error) {
	return a.repos.Research.GetResearchRun(ctx, id)
}
func (a *Application) ListResearchRuns(ctx context.Context, projectID string) ([]*domain.ResearchRun, error) {
	if err := a.RequireProject(ctx, projectID); err != nil {
		return nil, err
	}
	return a.repos.Research.ListResearchRuns(ctx, projectID)
}
func (a *Application) GetResearchIteration(ctx context.Context, runID, iterationID string) (*domain.ResearchIteration, error) {
	return a.repos.Research.GetResearchIteration(ctx, runID, iterationID)
}

func (a *Application) SaveHumanEvaluation(ctx context.Context, evaluation *domain.HumanEvaluation) error {
	if _, err := a.repos.Research.GetResearchIteration(ctx, evaluation.ResearchRunID, evaluation.IterationID); err != nil {
		return err
	}
	if !evaluation.Novelty.Valid() {
		return fmt.Errorf("invalid human novelty")
	}
	for _, score := range []int{evaluation.ObservationGrounding, evaluation.SurpriseUsefulness, evaluation.HypothesisDiversity, evaluation.CounterEvidenceQuality, evaluation.MissingEvidenceQuality, evaluation.IdentificationHonesty, evaluation.NextDataUsefulness, evaluation.OverallUsefulness} {
		if score < 1 || score > 5 {
			return fmt.Errorf("evaluation scores must be between 1 and 5")
		}
	}
	evaluation.EvaluatedAt = a.now()
	return a.repos.Research.SaveHumanEvaluation(ctx, evaluation)
}

func (a *Application) GetHumanEvaluation(ctx context.Context, runID, iterationID string) (*domain.HumanEvaluation, error) {
	return a.repos.Research.GetHumanEvaluation(ctx, runID, iterationID)
}


func (a *Application) CompareResearchIterations(ctx context.Context, runID, fromIterationID, toIterationID string) (*domain.InsightDelta, error) {
	from, err := a.repos.Research.GetResearchIteration(ctx, runID, fromIterationID)
	if err != nil {
		return nil, err
	}
	to, err := a.repos.Research.GetResearchIteration(ctx, runID, toIterationID)
	if err != nil {
		return nil, err
	}
	delta := service.CompareResearchIterations(*from, *to)
	return &delta, nil
}
