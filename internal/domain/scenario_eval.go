package domain

import (
	"fmt"
	"sort"
	"time"
)

// IndicatorOutcome is what one observation showed for one expectation.
type IndicatorOutcome string

const (
	OutcomeConsistent   IndicatorOutcome = "CONSISTENT"
	OutcomeContradicts  IndicatorOutcome = "CONTRADICTS"
	OutcomeInconclusive IndicatorOutcome = "INCONCLUSIVE"
	OutcomeNotObserved  IndicatorOutcome = "NOT_OBSERVED"
)

// AssumptionCheckResult records whether later evidence kept an assumption.
type AssumptionCheckResult string

const (
	AssumptionHolds       AssumptionCheckResult = "HOLDS"
	AssumptionInvalidated AssumptionCheckResult = "INVALIDATED"
	AssumptionUnknown     AssumptionCheckResult = "UNKNOWN"
)

// IndicatorObservation links a later observation to a frozen expectation.
type IndicatorObservation struct {
	ExpectationID string           `json:"expectationId"`
	Outcome       IndicatorOutcome `json:"outcome"`
	EvidenceRef   string           `json:"evidenceRef,omitempty"`
	ObservedAt    time.Time        `json:"observedAt"`
	Note          string           `json:"note,omitempty"`
}

type AssumptionCheck struct {
	AssumptionID string                `json:"assumptionId"`
	Result       AssumptionCheckResult `json:"result"`
	EvidenceRef  string                `json:"evidenceRef,omitempty"`
	Note         string                `json:"note,omitempty"`
}

type ScenarioState struct {
	ScenarioID      string         `json:"scenarioId"`
	Status          ScenarioStatus `json:"status"`
	Reasons         []string       `json:"reasons,omitempty"`
	LastEvaluatedAt time.Time      `json:"lastEvaluatedAt"`
}

type FiredFalsification struct {
	ScenarioID    string `json:"scenarioId"`
	ExpectationID string `json:"expectationId"`
	Condition     string `json:"condition"`
	EvidenceRef   string `json:"evidenceRef"`
}

type ScenarioStatusChange struct {
	ScenarioID string         `json:"scenarioId"`
	From       ScenarioStatus `json:"from"`
	To         ScenarioStatus `json:"to"`
}

// ScenarioDelta explains what new evidence changed. It ranks nothing.
type ScenarioDelta struct {
	FromEvaluationID       string                 `json:"fromEvaluationId,omitempty"`
	ToEvaluationID         string                 `json:"toEvaluationId"`
	Strengthened           []ScenarioStatusChange `json:"strengthened,omitempty"`
	Weakened               []ScenarioStatusChange `json:"weakened,omitempty"`
	Contradicted           []ScenarioStatusChange `json:"contradicted,omitempty"`
	Unchanged              []string               `json:"unchanged,omitempty"`
	AssumptionsInvalidated []string               `json:"assumptionsInvalidated,omitempty"`
	FalsificationsFired    []FiredFalsification   `json:"falsificationsFired,omitempty"`
	NewlyRequiredEvidence  []string               `json:"newlyRequiredEvidence,omitempty"`
	Explanation            []string               `json:"explanation,omitempty"`
}

// ScenarioEvaluation is an append-only evaluation of one ScenarioSet version
// against the evidence of one research iteration. Earlier evaluations are
// never rewritten; a later one carries forward their observations.
type ScenarioEvaluation struct {
	ID               string                 `json:"id"`
	ScenarioSetID    string                 `json:"scenarioSetId"`
	SetVersion       int                    `json:"setVersion"`
	ResearchRunID    string                 `json:"researchRunId"`
	IterationID      string                 `json:"iterationId,omitempty"`
	Observations     []IndicatorObservation `json:"observations"`
	AssumptionChecks []AssumptionCheck      `json:"assumptionChecks,omitempty"`
	States           []ScenarioState        `json:"states"`
	DataRequirements []DataRequirement      `json:"dataRequirements,omitempty"`
	Delta            ScenarioDelta          `json:"delta"`
	EvaluatedAt      time.Time              `json:"evaluatedAt"`
}

// NewObservationInput is the evidence submitted in one evaluation.
type NewObservationInput struct {
	Observations     []IndicatorObservation
	AssumptionChecks []AssumptionCheck
}

func (in NewObservationInput) validate(set ScenarioSet) error {
	exp, asm := map[string]bool{}, map[string]bool{}
	for _, sc := range set.Scenarios {
		for _, e := range sc.Expectations {
			exp[e.ID] = true
		}
		for _, a := range sc.Assumptions {
			asm[a.ID] = true
		}
	}
	for i, o := range in.Observations {
		if !exp[o.ExpectationID] {
			return scenarioErr("observations[%d] references unknown expectation %q", i, o.ExpectationID)
		}
		switch o.Outcome {
		case OutcomeConsistent, OutcomeContradicts:
			if blankStr(o.EvidenceRef) {
				return scenarioErr("observations[%d]: %s requires an evidenceRef", i, o.Outcome)
			}
		case OutcomeInconclusive, OutcomeNotObserved:
		default:
			return scenarioErr("observations[%d] has unknown outcome %q", i, o.Outcome)
		}
		if o.ObservedAt.IsZero() {
			return scenarioErr("observations[%d] requires observedAt", i)
		}
	}
	for i, c := range in.AssumptionChecks {
		if !asm[c.AssumptionID] {
			return scenarioErr("assumptionChecks[%d] references unknown assumption %q", i, c.AssumptionID)
		}
		switch c.Result {
		case AssumptionHolds, AssumptionUnknown:
		case AssumptionInvalidated:
			if blankStr(c.EvidenceRef) {
				return scenarioErr("assumptionChecks[%d]: INVALIDATED requires an evidenceRef", i)
			}
		default:
			return scenarioErr("assumptionChecks[%d] has unknown result %q", i, c.Result)
		}
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func fmtWindow(w TimeWindow) string {
	return fmt.Sprintf("%s..%s", w.Start.UTC().Format("2006-01-02"), w.End.UTC().Format("2006-01-02"))
}
