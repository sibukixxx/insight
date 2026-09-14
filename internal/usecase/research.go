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
	RunID           string
	Question        string
	InputReferences []string
	AddedEvidence   []string
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
	iteration := service.BuildResearchIteration(1, in.Question, in.InputReferences, insights, a.now())
	run := &domain.ResearchRun{ID: newID("run"), ProjectID: in.ProjectID, Question: strings.TrimSpace(in.Question), Iterations: []domain.ResearchIteration{iteration}, CreatedAt: a.now()}
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
	iteration := service.BuildResearchIteration(len(run.Iterations)+1, question, in.InputReferences, insights, a.now())
	iteration.AddedEvidence = append([]string(nil), in.AddedEvidence...)
	if len(run.Iterations) > 0 {
		iteration.HypothesisChanges = service.CompareHypothesisStates(run.Iterations[len(run.Iterations)-1].HypothesisStates, iteration.HypothesisStates)
	}
	if err := a.repos.Research.AppendResearchIteration(ctx, run.ID, iteration); err != nil {
		return nil, fmt.Errorf("append research iteration: %w", err)
	}
	return &iteration, nil
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
