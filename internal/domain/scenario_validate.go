package domain

import "fmt"

// Validate enforces the #66 guardrails: baseline/as-of and horizon are
// required, every scenario has at least one expectation that only a future
// observation can test, assumptions are typed, post-hoc provenance is never
// passed off as prior, and probability exists only with a basis.
func (s ScenarioSet) Validate() error {
	if blankStr(s.ResearchRunID) || blankStr(s.Question) {
		return scenarioErr("researchRunId and question are required")
	}
	if s.Baseline.AsOf.IsZero() || blankStr(s.Baseline.Description) {
		return scenarioErr("baseline requires asOf and description")
	}
	switch s.Origin {
	case ScenarioOriginScaffold, ScenarioOriginHuman, ScenarioOriginModel, ScenarioOriginImported:
	default:
		return scenarioErr("unknown origin %q", s.Origin)
	}
	if len(s.Scenarios) == 0 {
		return scenarioErr("at least one scenario is required")
	}
	ids := map[string]bool{}
	expectationIDs := map[string]bool{}
	for i, sc := range s.Scenarios {
		if blankStr(sc.ID) || blankStr(sc.Title) {
			return scenarioErr("scenarios[%d] requires id and title", i)
		}
		if ids[sc.ID] {
			return scenarioErr("duplicate scenario id %q", sc.ID)
		}
		ids[sc.ID] = true
		if err := sc.validate(s.Baseline, expectationIDs); err != nil {
			return scenarioErr("scenario %q: %v", sc.ID, err)
		}
	}
	for i, r := range s.Relations {
		if len(r.ScenarioIDs) < 2 {
			return scenarioErr("relations[%d] needs at least two scenario ids", i)
		}
		switch r.Kind {
		case RelationMutuallyExclusive, RelationOverlapping, RelationNested, RelationConflicting:
		default:
			return scenarioErr("relations[%d] has unknown kind %q", i, r.Kind)
		}
		for _, id := range r.ScenarioIDs {
			if !ids[id] {
				return scenarioErr("relations[%d] references unknown scenario %q", i, id)
			}
		}
	}
	return nil
}

func (sc Scenario) validate(baseline ScenarioBaseline, seen map[string]bool) error {
	if blankStr(sc.Horizon.Label) || sc.Horizon.End.IsZero() {
		return fmt.Errorf("horizon requires label and end")
	}
	if !sc.Horizon.End.After(baseline.AsOf) {
		return fmt.Errorf("horizon end must be after baseline asOf")
	}
	for _, a := range sc.Assumptions {
		if blankStr(a.ID) || blankStr(a.Statement) || !a.Kind.Valid() || a.AsOf.IsZero() {
			return fmt.Errorf("assumption %q requires id, statement, known kind and asOf", a.ID)
		}
		if a.Kind == AssumptionObservedBaseline && len(a.EvidenceRefs) == 0 {
			return fmt.Errorf("assumption %q: OBSERVED_BASELINE must cite observed evidence", a.ID)
		}
		if a.Kind != AssumptionObservedBaseline && len(a.EvidenceRefs) > 0 {
			return fmt.Errorf("assumption %q: only OBSERVED_BASELINE may cite evidence as observed fact; use sourceRefs", a.ID)
		}
	}
	future := 0
	for _, e := range sc.Expectations {
		if err := e.validate(); err != nil {
			return fmt.Errorf("expectation %q: %v", e.ID, err)
		}
		if seen[e.ID] {
			return fmt.Errorf("duplicate expectation id %q", e.ID)
		}
		seen[e.ID] = true
		if e.ObservationWindow.End.After(baseline.AsOf) {
			future++
		}
	}
	if future == 0 {
		return fmt.Errorf("at least one expectation must be testable by an observation after baseline asOf")
	}
	if p := sc.Probability; p != nil {
		if p.Value < 0 || p.Value > 1 || blankStr(p.Basis) || len(p.SourceRefs) == 0 {
			return fmt.Errorf("probability requires a value in [0,1], a basis and source references; the engine never invents one")
		}
	}
	return nil
}

func (e ScenarioExpectation) validate() error {
	if blankStr(e.ID) || blankStr(e.Indicator) || blankStr(e.Statement) {
		return fmt.Errorf("id, indicator and statement are required")
	}
	if blankStr(e.FalsificationCondition) {
		return fmt.Errorf("falsificationCondition is required")
	}
	if e.ObservationWindow.Start.IsZero() || e.ObservationWindow.End.IsZero() || e.ObservationWindow.End.Before(e.ObservationWindow.Start) {
		return fmt.Errorf("observationWindow requires start <= end")
	}
	switch e.Direction {
	case "", "INCREASE", "DECREASE", "NO_CHANGE":
	case "RANGE":
		if e.Range == nil || (e.Range.Min == nil && e.Range.Max == nil) {
			return fmt.Errorf("RANGE direction requires range min or max")
		}
	default:
		return fmt.Errorf("unknown direction %q", e.Direction)
	}
	if !e.Provenance.Valid() {
		return fmt.Errorf("provenance %q is not a known ExpectationBasis", e.Provenance)
	}
	if e.Provenance.RequiresSourceReference() && len(e.SourceRefs) == 0 {
		return fmt.Errorf("%s provenance requires sourceRefs", e.Provenance)
	}
	if e.Provenance.ObservationTiming() == TimingPreObservation && e.ObservedDataAvailableAtCreation {
		return fmt.Errorf("pre-observation provenance %s but observed data was available at creation (post-hoc leakage)", e.Provenance)
	}
	return nil
}
