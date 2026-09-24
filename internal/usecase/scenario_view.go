package usecase

import (
	"context"
	"time"

	"insight-lab/internal/domain"
)

// IterationScenario is one scenario as seen from one research iteration: the
// frozen scenario definition plus its evidence state at that iteration.
// Status is never a probability.
type IterationScenario struct {
	domain.Scenario
	Kind            string                `json:"kind"` // always "SCENARIO": future-facing, never observed fact
	ScenarioSetID   string                `json:"scenarioSetId"`
	SetVersion      int                   `json:"setVersion"`
	BaselineAsOf    time.Time             `json:"baselineAsOf"`
	NonExhaustive   bool                  `json:"nonExhaustive"`
	Status          domain.ScenarioStatus `json:"status"`
	StatusReasons   []string              `json:"statusReasons,omitempty"`
	LastEvaluatedAt *time.Time            `json:"lastEvaluatedAt,omitempty"`
}

// ResearchIterationView adds read-only scenario state to an iteration. The
// stored iteration payload is unchanged (backwards compatible).
type ResearchIterationView struct {
	domain.ResearchIteration
	Scenarios     []IterationScenario   `json:"scenarios,omitempty"`
	ScenarioDelta *domain.ScenarioDelta `json:"scenarioDelta,omitempty"`
}

type ResearchRunView struct {
	domain.ResearchRun
	Iterations []ResearchIterationView `json:"iterations"`
}

// GetResearchRunView returns the run with scenarios attached per iteration:
// the evaluation recorded for that iteration, or the scenario set drafted
// from it (all UNTESTED) when it has not been evaluated.
func (a *Application) GetResearchRunView(ctx context.Context, runID string) (*ResearchRunView, error) {
	run, err := a.repos.Research.GetResearchRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	view := &ResearchRunView{ResearchRun: *run}
	var sets []*domain.ScenarioSet
	var evals []*domain.ScenarioEvaluation
	if a.repos.Scenarios != nil {
		if sets, err = a.repos.Scenarios.ListScenarioSets(ctx, runID); err != nil {
			return nil, err
		}
		if evals, err = a.repos.Scenarios.ListScenarioEvaluations(ctx, runID); err != nil {
			return nil, err
		}
	}
	setByID := map[string]*domain.ScenarioSet{}
	for _, s := range sets {
		setByID[s.ID] = s
	}
	for _, it := range run.Iterations {
		iv := ResearchIterationView{ResearchIteration: it}
		var evaluation *domain.ScenarioEvaluation
		for _, e := range evals {
			if e.IterationID == it.ID {
				evaluation = e
			}
		}
		switch {
		case evaluation != nil && setByID[evaluation.ScenarioSetID] != nil:
			iv.Scenarios = iterationScenarios(setByID[evaluation.ScenarioSetID], evaluation)
			delta := evaluation.Delta
			iv.ScenarioDelta = &delta
		default:
			for _, s := range sets {
				if s.FromIterationID == it.ID {
					iv.Scenarios = iterationScenarios(s, nil)
				}
			}
		}
		view.Iterations = append(view.Iterations, iv)
	}
	return view, nil
}

func iterationScenarios(set *domain.ScenarioSet, eval *domain.ScenarioEvaluation) []IterationScenario {
	states := map[string]domain.ScenarioState{}
	if eval != nil {
		for _, st := range eval.States {
			states[st.ScenarioID] = st
		}
	}
	out := make([]IterationScenario, 0, len(set.Scenarios))
	for _, sc := range set.Scenarios {
		item := IterationScenario{Scenario: sc, Kind: "SCENARIO", ScenarioSetID: set.ID, SetVersion: set.Version,
			BaselineAsOf: set.Baseline.AsOf, NonExhaustive: set.NonExhaustive, Status: domain.ScenarioUntested}
		if st, ok := states[sc.ID]; ok {
			item.Status, item.StatusReasons = st.Status, st.Reasons
			t := st.LastEvaluatedAt
			item.LastEvaluatedAt = &t
		}
		out = append(out, item)
	}
	return out
}
