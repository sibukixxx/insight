package usecase

import (
	"context"
	"errors"
	"fmt"

	"insight-lab/internal/domain"
)

// ErrScenarioStorageUnavailable means scenario persistence is not wired.
var ErrScenarioStorageUnavailable = errors.New("scenario repository is not configured")

// ErrScenarioInvalid re-exports the domain validation sentinel for transports.
var ErrScenarioInvalid = domain.ErrScenarioInvalid

// ScenarioAnalysis is the exportable scenario view of one research run: every
// set version and evaluation (append-only history) plus the current state of
// the latest set version. It never names a preferred scenario.
type ScenarioAnalysis struct {
	ResearchRunID     string                       `json:"researchRunId"`
	CurrentSet        *domain.ScenarioSet          `json:"currentSet,omitempty"`
	CurrentEvaluation *domain.ScenarioEvaluation   `json:"currentEvaluation,omitempty"`
	Sets              []*domain.ScenarioSet        `json:"sets"`
	Evaluations       []*domain.ScenarioEvaluation `json:"evaluations"`
}

type ScaffoldScenariosInput struct {
	RunID       string
	IterationID string // empty = latest iteration
	Baseline    domain.ScenarioBaseline
	Horizon     domain.ScenarioHorizon
}

type ScaffoldScenariosResult struct {
	Set     *domain.ScenarioSet `json:"scenarioSet"`
	Skipped []string            `json:"skipped,omitempty"`
}

func (a *Application) scenarioRun(ctx context.Context, runID string) (*domain.ResearchRun, error) {
	if a.repos.Scenarios == nil {
		return nil, ErrScenarioStorageUnavailable
	}
	return a.repos.Research.GetResearchRun(ctx, runID)
}

// ScaffoldScenarios drafts and stores a new scenario set version from the
// competing hypotheses of one iteration.
func (a *Application) ScaffoldScenarios(ctx context.Context, in ScaffoldScenariosInput) (*ScaffoldScenariosResult, error) {
	run, err := a.scenarioRun(ctx, in.RunID)
	if err != nil {
		return nil, err
	}
	iteration, ok := run.LatestIteration()
	if in.IterationID != "" {
		found, err := a.repos.Research.GetResearchIteration(ctx, run.ID, in.IterationID)
		if err != nil {
			return nil, err
		}
		iteration, ok = *found, true
	}
	if !ok {
		return nil, fmt.Errorf("%w: research run has no iteration", ErrScenarioInvalid)
	}
	var insights []*domain.Insight
	for _, id := range iteration.InsightIDs {
		insight, err := a.repos.Insights.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		insights = append(insights, insight)
	}
	set, skipped, err := domain.ScaffoldScenarioSet(domain.ScaffoldInput{ResearchRunID: run.ID, Question: iteration.Question,
		IterationID: iteration.ID, Insights: insights, Baseline: in.Baseline, Horizon: in.Horizon, Now: a.now()})
	if err != nil {
		return nil, err
	}
	stored, err := a.storeScenarioSet(ctx, set)
	if err != nil {
		return nil, err
	}
	return &ScaffoldScenariosResult{Set: stored, Skipped: skipped}, nil
}

// CreateScenarioSet stores a curated scenario set as the next version.
func (a *Application) CreateScenarioSet(ctx context.Context, set domain.ScenarioSet) (*domain.ScenarioSet, error) {
	if _, err := a.scenarioRun(ctx, set.ResearchRunID); err != nil {
		return nil, err
	}
	set.CreatedAt = a.now()
	return a.storeScenarioSet(ctx, set)
}

func (a *Application) storeScenarioSet(ctx context.Context, set domain.ScenarioSet) (*domain.ScenarioSet, error) {
	existing, err := a.repos.Scenarios.ListScenarioSets(ctx, set.ResearchRunID)
	if err != nil {
		return nil, err
	}
	set.ID, set.Version = newID("scnset"), len(existing)+1
	if err := set.Validate(); err != nil {
		return nil, err
	}
	if err := a.repos.Scenarios.CreateScenarioSet(ctx, &set); err != nil {
		return nil, err
	}
	return &set, nil
}

type EvaluateScenariosInput struct {
	RunID         string
	ScenarioSetID string
	IterationID   string
	Input         domain.NewObservationInput
}

// EvaluateScenarios appends an evaluation of a set version, carrying forward
// the previous evaluation of the same version.
func (a *Application) EvaluateScenarios(ctx context.Context, in EvaluateScenariosInput) (*domain.ScenarioEvaluation, error) {
	if _, err := a.scenarioRun(ctx, in.RunID); err != nil {
		return nil, err
	}
	set, err := a.repos.Scenarios.GetScenarioSet(ctx, in.ScenarioSetID)
	if err != nil {
		return nil, err
	}
	if set.ResearchRunID != in.RunID {
		return nil, ErrNotFound
	}
	if in.IterationID != "" {
		if _, err := a.repos.Research.GetResearchIteration(ctx, in.RunID, in.IterationID); err != nil {
			return nil, err
		}
	}
	all, err := a.repos.Scenarios.ListScenarioEvaluations(ctx, in.RunID)
	if err != nil {
		return nil, err
	}
	var prev *domain.ScenarioEvaluation
	for _, e := range all {
		if e.ScenarioSetID == set.ID {
			prev = e
		}
	}
	eval, err := domain.EvaluateScenarios(*set, prev, in.Input, newID("scneval"), in.IterationID, a.now())
	if err != nil {
		return nil, err
	}
	if err := a.repos.Scenarios.CreateScenarioEvaluation(ctx, &eval); err != nil {
		return nil, err
	}
	return &eval, nil
}

// GetScenarioAnalysis returns the run's scenario history. A run without
// scenarios returns empty lists, never synthesized scenarios.
func (a *Application) GetScenarioAnalysis(ctx context.Context, runID string) (*ScenarioAnalysis, error) {
	if _, err := a.scenarioRun(ctx, runID); err != nil {
		return nil, err
	}
	sets, err := a.repos.Scenarios.ListScenarioSets(ctx, runID)
	if err != nil {
		return nil, err
	}
	evals, err := a.repos.Scenarios.ListScenarioEvaluations(ctx, runID)
	if err != nil {
		return nil, err
	}
	out := &ScenarioAnalysis{ResearchRunID: runID, Sets: sets, Evaluations: evals}
	if len(sets) > 0 {
		out.CurrentSet = sets[len(sets)-1]
		for _, e := range evals {
			if e.ScenarioSetID == out.CurrentSet.ID {
				out.CurrentEvaluation = e
			}
		}
	}
	return out, nil
}
