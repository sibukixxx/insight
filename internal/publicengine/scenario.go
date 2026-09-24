package publicengine

import (
	"context"
	"net/http"

	"insight-lab/internal/domain"
	"insight-lab/internal/usecase"
)

// GetScenarios returns the scenario history of a public research run. A run
// without scenarios returns empty lists; nothing is synthesized.
func (e *Engine) GetScenarios(ctx context.Context, researchRunID string) (ScenarioAnalysis, error) {
	run, err := e.publicResearchRun(ctx, researchRunID)
	if err != nil {
		return ScenarioAnalysis{}, err
	}
	a, err := e.app.GetScenarioAnalysis(ctx, run.ID)
	if err != nil {
		return ScenarioAnalysis{}, err
	}
	return ScenarioAnalysis{ContractVersion: ContractVersion, ResearchRunID: run.ID, CurrentSet: a.CurrentSet,
		CurrentEvaluation: a.CurrentEvaluation, Sets: a.Sets, Evaluations: a.Evaluations}, nil
}

func (e *Engine) CreateScenarioSet(ctx context.Context, researchRunID string, req CreateScenarioSetRequest) (int, []byte, error) {
	return e.idempotent(ctx, req.ContractVersion, req.IdempotencyKey, "createScenarioSet:"+researchRunID, req, func() (int, any, error) {
		run, err := e.publicResearchRun(ctx, researchRunID)
		if err != nil {
			return 0, nil, err
		}
		d := req.ScenarioSet
		set, err := e.app.CreateScenarioSet(ctx, domain.ScenarioSet{ResearchRunID: run.ID, Question: d.Question, Baseline: d.Baseline,
			NonExhaustive: d.NonExhaustive, SharedEvidenceRefs: d.SharedEvidenceRefs, Scenarios: d.Scenarios, Relations: d.Relations,
			Origin: domain.ScenarioOrigin(d.Origin), FromIterationID: d.FromIterationID})
		if err != nil {
			return 0, nil, err
		}
		return http.StatusCreated, ScenarioSetResult{ContractVersion: ContractVersion, ResearchRunID: run.ID, ScenarioSet: *set}, nil
	})
}

func (e *Engine) ScaffoldScenarioSet(ctx context.Context, researchRunID string, req ScaffoldScenarioSetRequest) (int, []byte, error) {
	return e.idempotent(ctx, req.ContractVersion, req.IdempotencyKey, "scaffoldScenarioSet:"+researchRunID, req, func() (int, any, error) {
		run, err := e.publicResearchRun(ctx, researchRunID)
		if err != nil {
			return 0, nil, err
		}
		res, err := e.app.ScaffoldScenarios(ctx, usecase.ScaffoldScenariosInput{RunID: run.ID, IterationID: req.IterationID,
			Baseline: req.Baseline, Horizon: req.Horizon})
		if err != nil {
			return 0, nil, err
		}
		return http.StatusCreated, ScenarioSetResult{ContractVersion: ContractVersion, ResearchRunID: run.ID, ScenarioSet: *res.Set, Skipped: res.Skipped}, nil
	})
}

func (e *Engine) EvaluateScenarios(ctx context.Context, researchRunID, scenarioSetID string, req EvaluateScenariosRequest) (int, []byte, error) {
	return e.idempotent(ctx, req.ContractVersion, req.IdempotencyKey, "evaluateScenarios:"+researchRunID+":"+scenarioSetID, req, func() (int, any, error) {
		run, err := e.publicResearchRun(ctx, researchRunID)
		if err != nil {
			return 0, nil, err
		}
		eval, err := e.app.EvaluateScenarios(ctx, usecase.EvaluateScenariosInput{RunID: run.ID, ScenarioSetID: scenarioSetID,
			IterationID: req.IterationID, Input: domain.NewObservationInput{Observations: req.Observations, AssumptionChecks: req.AssumptionChecks}})
		if err != nil {
			return 0, nil, err
		}
		return http.StatusCreated, ScenarioEvaluationResult{ContractVersion: ContractVersion, ResearchRunID: run.ID, Evaluation: *eval}, nil
	})
}
