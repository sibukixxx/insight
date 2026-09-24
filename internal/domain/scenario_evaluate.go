package domain

import (
	"fmt"
	"time"
)

// EvaluateScenarios appends one evaluation. prev is the latest earlier
// evaluation of the same set version (nil for the first). Observations made
// at or before the set was frozen (CreatedAt) cannot test it: they are kept
// for audit but counted as INCONCLUSIVE, so post-hoc evidence never
// "confirms" a scenario written after seeing it.
func EvaluateScenarios(set ScenarioSet, prev *ScenarioEvaluation, in NewObservationInput, id, iterationID string, now time.Time) (ScenarioEvaluation, error) {
	if err := set.Validate(); err != nil {
		return ScenarioEvaluation{}, err
	}
	if err := in.validate(set); err != nil {
		return ScenarioEvaluation{}, err
	}
	var observations []IndicatorObservation
	var checks []AssumptionCheck
	if prev != nil {
		if prev.ScenarioSetID != set.ID || prev.SetVersion != set.Version {
			return ScenarioEvaluation{}, scenarioErr("previous evaluation belongs to another scenario set version")
		}
		observations = append(observations, prev.Observations...)
		checks = append(checks, prev.AssumptionChecks...)
	}
	observations = append(observations, in.Observations...)
	checks = append(checks, in.AssumptionChecks...)

	eval := ScenarioEvaluation{ID: id, ScenarioSetID: set.ID, SetVersion: set.Version, ResearchRunID: set.ResearchRunID,
		IterationID: iterationID, Observations: observations, AssumptionChecks: checks, EvaluatedAt: now}
	invalidated := map[string]AssumptionCheck{}
	for _, c := range checks {
		if c.Result == AssumptionInvalidated {
			invalidated[c.AssumptionID] = c
		} else {
			delete(invalidated, c.AssumptionID) // latest check wins for non-invalidation
		}
	}
	var allFired []FiredFalsification
	for _, sc := range set.Scenarios {
		state, fired := scenarioState(set, sc, observations, invalidated, now)
		eval.States = append(eval.States, state)
		allFired = append(allFired, fired...)
		eval.DataRequirements = append(eval.DataRequirements, scenarioRequirements(sc, observations, set.CreatedAt)...)
	}
	eval.Delta = buildScenarioDelta(prev, eval, in, allFired)
	return eval, nil
}

func scenarioState(set ScenarioSet, sc Scenario, obs []IndicatorObservation, invalidated map[string]AssumptionCheck, now time.Time) (ScenarioState, []FiredFalsification) {
	state := ScenarioState{ScenarioID: sc.ID, LastEvaluatedAt: now}
	var consistent, inconclusive int
	var fired []FiredFalsification
	byID := map[string]ScenarioExpectation{}
	for _, e := range sc.Expectations {
		byID[e.ID] = e
	}
	for _, o := range obs {
		e, ok := byID[o.ExpectationID]
		if !ok {
			continue
		}
		if !o.ObservedAt.After(set.CreatedAt) && (o.Outcome == OutcomeConsistent || o.Outcome == OutcomeContradicts) {
			inconclusive++
			state.Reasons = append(state.Reasons, fmt.Sprintf("%s: observation %s predates the scenario set and cannot test it", e.ID, o.EvidenceRef))
			continue
		}
		switch o.Outcome {
		case OutcomeContradicts:
			fired = append(fired, FiredFalsification{ScenarioID: sc.ID, ExpectationID: e.ID, Condition: e.FalsificationCondition, EvidenceRef: o.EvidenceRef})
			state.Reasons = append(state.Reasons, fmt.Sprintf("%s: falsification condition fired (%s)", e.ID, o.EvidenceRef))
		case OutcomeConsistent:
			consistent++
			state.Reasons = append(state.Reasons, fmt.Sprintf("%s: consistent with %s", e.ID, o.EvidenceRef))
		case OutcomeInconclusive:
			inconclusive++
			state.Reasons = append(state.Reasons, fmt.Sprintf("%s: inconclusive", e.ID))
		}
	}
	var weakened []string
	for _, a := range sc.Assumptions {
		if c, ok := invalidated[a.ID]; ok {
			weakened = append(weakened, a.ID)
			state.Reasons = append(state.Reasons, fmt.Sprintf("assumption %s invalidated (%s)", a.ID, c.EvidenceRef))
		}
	}
	switch {
	case len(fired) > 0:
		state.Status = ScenarioContradicted
	case len(weakened) > 0:
		state.Status = ScenarioWeakened
	case consistent > 0:
		state.Status = ScenarioConsistentSoFar
	case inconclusive > 0:
		state.Status = ScenarioInconclusive
	default:
		state.Status = ScenarioUntested
	}
	return state, fired
}

// scenarioRequirements turns every expectation still lacking a decisive
// observation into a scenario-specific DataRequirement.
func scenarioRequirements(sc Scenario, obs []IndicatorObservation, frozenAt time.Time) []DataRequirement {
	decided := map[string]bool{}
	for _, o := range obs {
		if (o.Outcome == OutcomeConsistent || o.Outcome == OutcomeContradicts) && o.ObservedAt.After(frozenAt) {
			decided[o.ExpectationID] = true
		}
	}
	var out []DataRequirement
	for _, e := range sc.Expectations {
		if decided[e.ExpectationKey()] {
			continue
		}
		out = append(out, DataRequirement{
			GapID:          "scenario:" + sc.ID + ":" + e.ID,
			Need:           fmt.Sprintf("observe %s over %s", e.Indicator, fmtWindow(e.ObservationWindow)),
			Reason:         fmt.Sprintf("tests scenario %q; falsified if: %s", sc.Title, e.FalsificationCondition),
			RequiredPeriod: fmtWindow(e.ObservationWindow),
			Priority:       RequirementPriority{CanFalsify: true, Rationale: []string{"scenario expectation without a decisive observation"}},
		})
	}
	return out
}

// ExpectationKey is the identity used to match observations.
func (e ScenarioExpectation) ExpectationKey() string { return e.ID }
