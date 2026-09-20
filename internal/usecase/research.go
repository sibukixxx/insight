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
}

type AppendResearchIterationInput struct {
	RunID             string
	Question          string
	InputReferences   []string
	AddedEvidence     []string
	EvidenceAdditions []domain.EvidenceAddition
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
	iteration := service.BuildResearchIteration(1, in.Question, in.InputReferences, insights, now)
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
	iteration := service.BuildResearchIteration(len(run.Iterations)+1, question, in.InputReferences, insights, now)
	additions := append([]domain.EvidenceAddition(nil), in.EvidenceAdditions...)
	for i := range additions {
		if additions[i].AddedAt.IsZero() {
			additions[i].AddedAt = now
		}
	}
	iteration = service.FinalizeResearchIterationWithEvidence(*run, iteration, in.AddedEvidence, additions, now)
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
	RunID       string
	IterationID string
	TargetStage domain.ResearchStage
	// IndependentEvidencePlanned is retained for source compatibility only.
	// VALIDATION now requires concrete ValidationEvidence provenance.
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
		if len(validationEvidence) == 0 {
			return nil, fmt.Errorf("VALIDATION requires concrete independent-evidence provenance; the legacy boolean is not auditable")
		}
		frozen := map[string]domain.Expectation{}
		for _, expectation := range latest.Expectations {
			if expectation.FrozenForValidation {
				frozen[expectation.ID] = expectation
			}
		}
		covered := map[string]bool{}
		for i := range validationEvidence {
			e, ok := frozen[validationEvidence[i].ExpectationID]
			if !ok {
				return nil, fmt.Errorf("validation evidence references non-frozen expectation %q", validationEvidence[i].ExpectationID)
			}
			validationEvidence[i].RecordedBy = domain.AuthorHuman
			validationEvidence[i].RecordedAt = a.now()
			if err := validationEvidence[i].ValidateAgainst(e); err != nil {
				return nil, err
			}
			covered[e.ID] = true
		}
		for id := range frozen {
			if !covered[id] {
				return nil, fmt.Errorf("frozen expectation %q has no independent validation evidence provenance", id)
			}
		}
	}
	if err := latest.Stage.Transition(in.TargetStage, domain.StageTransitionInput{
		ObservationCount:           len(latest.ObservationIDs),
		Expectations:               latest.Expectations,
		IndependentEvidencePlanned: in.TargetStage != domain.StageValidation || len(validationEvidence) > 0,
		CompletedValidationCount:   completedValidations,
	}); err != nil {
		return nil, err
	}
	updated := latest
	updated.Stage = in.TargetStage
	if in.TargetStage == domain.StageValidation {
		updated.ValidationEvidence = append(append([]domain.ValidationEvidenceProvenance(nil), latest.ValidationEvidence...), validationEvidence...)
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
