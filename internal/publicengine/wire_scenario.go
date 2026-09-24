package publicengine

import "insight-lab/internal/domain"

// Scenario wire types (#66). Nested scenario objects are the domain types
// themselves; drift_test.go holds their JSON fields to schema.json exactly.

type ScenarioSetDraft struct {
	Question           string                    `json:"question"`
	Baseline           domain.ScenarioBaseline   `json:"baseline"`
	NonExhaustive      bool                      `json:"nonExhaustive"`
	SharedEvidenceRefs []string                  `json:"sharedEvidenceRefs,omitempty"`
	Scenarios          []domain.Scenario         `json:"scenarios"`
	Relations          []domain.ScenarioRelation `json:"relations,omitempty"`
	Origin             string                    `json:"origin"`
	FromIterationID    string                    `json:"fromIterationId,omitempty"`
}

type CreateScenarioSetRequest struct {
	ContractVersion string           `json:"contractVersion"`
	IdempotencyKey  string           `json:"idempotencyKey"`
	ScenarioSet     ScenarioSetDraft `json:"scenarioSet"`
}

type ScaffoldScenarioSetRequest struct {
	ContractVersion string                  `json:"contractVersion"`
	IdempotencyKey  string                  `json:"idempotencyKey"`
	IterationID     string                  `json:"iterationId,omitempty"`
	Baseline        domain.ScenarioBaseline `json:"baseline"`
	Horizon         domain.ScenarioHorizon  `json:"horizon"`
}

type EvaluateScenariosRequest struct {
	ContractVersion  string                        `json:"contractVersion"`
	IdempotencyKey   string                        `json:"idempotencyKey"`
	IterationID      string                        `json:"iterationId,omitempty"`
	Observations     []domain.IndicatorObservation `json:"observations,omitempty"`
	AssumptionChecks []domain.AssumptionCheck      `json:"assumptionChecks,omitempty"`
}

type ScenarioSetResult struct {
	ContractVersion string             `json:"contractVersion"`
	ResearchRunID   string             `json:"researchRunId"`
	ScenarioSet     domain.ScenarioSet `json:"scenarioSet"`
	Skipped         []string           `json:"skipped,omitempty"`
}

type ScenarioEvaluationResult struct {
	ContractVersion string                    `json:"contractVersion"`
	ResearchRunID   string                    `json:"researchRunId"`
	Evaluation      domain.ScenarioEvaluation `json:"evaluation"`
}

type ScenarioAnalysis struct {
	ContractVersion   string                       `json:"contractVersion"`
	ResearchRunID     string                       `json:"researchRunId"`
	CurrentSet        *domain.ScenarioSet          `json:"currentSet,omitempty"`
	CurrentEvaluation *domain.ScenarioEvaluation   `json:"currentEvaluation,omitempty"`
	Sets              []*domain.ScenarioSet        `json:"sets"`
	Evaluations       []*domain.ScenarioEvaluation `json:"evaluations"`
}

// scenarioWireTypes extends the drift check map.
var scenarioWireTypes = map[string]any{
	"ScenarioSetDraft": ScenarioSetDraft{}, "CreateScenarioSetRequest": CreateScenarioSetRequest{},
	"ScaffoldScenarioSetRequest": ScaffoldScenarioSetRequest{}, "EvaluateScenariosRequest": EvaluateScenariosRequest{},
	"ScenarioSetResult": ScenarioSetResult{}, "ScenarioEvaluationResult": ScenarioEvaluationResult{},
	"ScenarioAnalysis": ScenarioAnalysis{},
	"ScenarioBaseline": domain.ScenarioBaseline{}, "ScenarioHorizon": domain.ScenarioHorizon{}, "TimeWindow": domain.TimeWindow{},
	"Assumption": domain.Assumption{}, "ValueRange": domain.ValueRange{}, "ScenarioExpectation": domain.ScenarioExpectation{},
	"ScenarioProbability": domain.ScenarioProbability{}, "Scenario": domain.Scenario{}, "ScenarioRelation": domain.ScenarioRelation{},
	"ScenarioSet": domain.ScenarioSet{}, "IndicatorObservation": domain.IndicatorObservation{}, "AssumptionCheck": domain.AssumptionCheck{},
	"ScenarioState": domain.ScenarioState{}, "FiredFalsification": domain.FiredFalsification{},
	"ScenarioStatusChange": domain.ScenarioStatusChange{}, "ScenarioDelta": domain.ScenarioDelta{},
	"ScenarioEvaluation": domain.ScenarioEvaluation{},
}
