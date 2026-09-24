package domain

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func loadJapanEU(t *testing.T) ScenarioSet {
	t.Helper()
	data, err := os.ReadFile("testdata/scenario_japan_eu.json")
	if err != nil {
		t.Fatal(err)
	}
	var s ScenarioSet
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func at(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func TestScenarioSetValidateAcceptsJapanEUFixtureWithFiveBranchesFromOneBaseline(t *testing.T) {
	s := loadJapanEU(t)
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(s.Scenarios) < 3 {
		t.Fatalf("expected >=3 branches, got %d", len(s.Scenarios))
	}
}

func TestScenarioSetValidateRejectsGuardrailViolations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ScenarioSet)
		want   string
	}{
		{"missing horizon", func(s *ScenarioSet) { s.Scenarios[0].Horizon = ScenarioHorizon{} }, "horizon requires"},
		{"missing baseline asOf", func(s *ScenarioSet) { s.Baseline.AsOf = time.Time{} }, "baseline requires"},
		{"post-hoc leakage as prior", func(s *ScenarioSet) { s.Scenarios[0].Expectations[0].ObservedDataAvailableAtCreation = true }, "post-hoc leakage"},
		{"invented probability", func(s *ScenarioSet) { s.Scenarios[0].Probability = &ScenarioProbability{Value: 0.7} }, "never invents"},
		{"no future-testable expectation", func(s *ScenarioSet) {
			for i := range s.Scenarios[1].Expectations {
				s.Scenarios[1].Expectations[i].ObservationWindow = TimeWindow{Start: at("2025-01-01T00:00:00Z"), End: at("2026-01-01T00:00:00Z")}
			}
		}, "after baseline asOf"},
		{"assumption cites evidence as fact", func(s *ScenarioSet) { s.Scenarios[2].Assumptions[0].EvidenceRefs = []string{"x"} }, "only OBSERVED_BASELINE"},
		{"missing falsification", func(s *ScenarioSet) { s.Scenarios[0].Expectations[0].FalsificationCondition = "" }, "falsificationCondition"},
		{"relation to unknown scenario", func(s *ScenarioSet) { s.Relations[0].ScenarioIDs = []string{"S3", "S9"} }, "unknown scenario"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := loadJapanEU(t)
			tt.mutate(&s)
			err := s.Validate()
			if !errors.Is(err, ErrScenarioInvalid) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want %q, got %v", tt.want, err)
			}
		})
	}
}

func TestEvaluateScenariosContradictsOneBranchWithoutMutatingPriorEvaluation(t *testing.T) {
	s := loadJapanEU(t)
	first, err := EvaluateScenarios(s, nil, NewObservationInput{Observations: []IndicatorObservation{
		{ExpectationID: "S1-E2", Outcome: OutcomeConsistent, EvidenceRef: "comext:2026-09", ObservedAt: at("2026-10-15T00:00:00Z")},
	}}, "ev1", "it2", at("2026-10-20T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, _ := json.Marshal(first)
	second, err := EvaluateScenarios(s, &first, NewObservationInput{Observations: []IndicatorObservation{
		{ExpectationID: "S1-E1", Outcome: OutcomeContradicts, EvidenceRef: "comext:2026-12", ObservedAt: at("2027-01-15T00:00:00Z")},
	}, AssumptionChecks: []AssumptionCheck{{AssumptionID: "S3-A1", Result: AssumptionInvalidated, EvidenceRef: "eurostat:2026Q4"}}}, "ev2", "it3", at("2027-01-20T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(first)
	if string(snapshot) != string(after) {
		t.Fatal("prior evaluation was mutated")
	}
	status := map[string]ScenarioStatus{}
	for _, st := range second.States {
		status[st.ScenarioID] = st.Status
	}
	want := map[string]ScenarioStatus{"S1": ScenarioContradicted, "S2": ScenarioUntested, "S3": ScenarioWeakened, "S4": ScenarioUntested, "S5": ScenarioUntested}
	if !reflect.DeepEqual(status, want) {
		t.Fatalf("status = %v", status)
	}
	if len(second.Delta.Contradicted) != 1 || second.Delta.Contradicted[0].From != ScenarioConsistentSoFar ||
		len(second.Delta.FalsificationsFired) != 1 || !reflect.DeepEqual(second.Delta.AssumptionsInvalidated, []string{"S3-A1"}) {
		t.Fatalf("delta = %+v", second.Delta)
	}
	if len(second.Observations) != 2 {
		t.Fatalf("observations must accumulate, got %d", len(second.Observations))
	}
}

func TestEvaluateScenariosTreatsObservationBeforeFreezeAsInconclusive(t *testing.T) {
	s := loadJapanEU(t)
	ev, err := EvaluateScenarios(s, nil, NewObservationInput{Observations: []IndicatorObservation{
		{ExpectationID: "S1-E1", Outcome: OutcomeConsistent, EvidenceRef: "comext:2026-05", ObservedAt: at("2026-06-15T00:00:00Z")},
	}}, "ev1", "", at("2026-07-02T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if ev.States[0].Status != ScenarioInconclusive {
		t.Fatalf("post-hoc observation confirmed a scenario: %+v", ev.States[0])
	}
}

func TestEvaluateScenariosGeneratesScenarioSpecificDataRequirements(t *testing.T) {
	s := loadJapanEU(t)
	ev, err := EvaluateScenarios(s, nil, NewObservationInput{}, "ev1", "", at("2026-07-02T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ev.DataRequirements) != 6 || ev.DataRequirements[0].GapID != "scenario:S1:S1-E1" || !ev.DataRequirements[0].Priority.CanFalsify {
		t.Fatalf("requirements = %+v", ev.DataRequirements)
	}
	for _, st := range ev.States {
		if st.Status != ScenarioUntested {
			t.Fatalf("no evidence must leave %s UNTESTED, got %s", st.ScenarioID, st.Status)
		}
	}
}

func TestEvaluateScenariosRejectsDecisiveOutcomeWithoutEvidenceRef(t *testing.T) {
	s := loadJapanEU(t)
	_, err := EvaluateScenarios(s, nil, NewObservationInput{Observations: []IndicatorObservation{
		{ExpectationID: "S1-E1", Outcome: OutcomeConsistent, ObservedAt: at("2026-08-01T00:00:00Z")},
	}}, "ev1", "", at("2026-08-02T00:00:00Z"))
	if !errors.Is(err, ErrScenarioInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestScenarioDeltaHasNoWinnerField(t *testing.T) {
	data, _ := json.Marshal(ScenarioDelta{})
	for _, banned := range []string{"winner", "mostLikely", "best", "probability"} {
		if strings.Contains(strings.ToLower(string(data)), strings.ToLower(banned)) {
			t.Fatalf("delta exposes %q: %s", banned, data)
		}
	}
}

func TestScaffoldScenarioSetDraftsOneBranchPerSurvivingHypothesisAndSkipsUntestable(t *testing.T) {
	insights := []*Insight{
		{ID: "i1", Title: "FX pass-through", Rationale: "exporters cut EUR prices", FalsificationCriteria: []string{"EUR price unchanged"},
			CompetingHypotheses: []CompetingHypothesis{{Title: "Freight absorbs FX", FalsificationCriteria: []string{"landed price falls"}}, {Title: "No criteria"}}},
		{ID: "i2", Title: "Contradicted", ValidationStatus: ValidationContradicted, FalsificationCriteria: []string{"x"}},
	}
	set, skipped, err := ScaffoldScenarioSet(ScaffoldInput{ResearchRunID: "r", Question: "q", IterationID: "it1", Insights: insights,
		Baseline: ScenarioBaseline{AsOf: at("2026-06-30T00:00:00Z"), Description: "baseline"},
		Horizon:  ScenarioHorizon{Label: "12m", End: at("2027-06-30T00:00:00Z")}, Now: at("2026-07-01T00:00:00Z")})
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Scenarios) != 2 || len(skipped) != 1 || !set.NonExhaustive || set.Scenarios[0].Probability != nil {
		t.Fatalf("set=%+v skipped=%v", set.Scenarios, skipped)
	}
	if e := set.Scenarios[0].Expectations[0]; e.Provenance != ExpectationDerivedFromPriorRun || e.ObservedDataAvailableAtCreation {
		t.Fatalf("scaffold expectation provenance = %+v", e)
	}
	set.ID, set.Version = "s1", 1
	if err := set.Validate(); err != nil {
		t.Fatalf("scaffold must validate: %v", err)
	}
}

func TestEvaluateScenariosTreatsObservationOutsideWindowAsInconclusive(t *testing.T) {
	s := loadJapanEU(t)
	ev, err := EvaluateScenarios(s, nil, NewObservationInput{Observations: []IndicatorObservation{
		{ExpectationID: "S1-E1", Outcome: OutcomeContradicts, EvidenceRef: "comext:2028-01", ObservedAt: at("2028-01-15T00:00:00Z")},
	}}, "ev1", "", at("2028-02-01T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if ev.States[0].Status != ScenarioInconclusive || len(ev.Delta.FalsificationsFired) != 0 {
		t.Fatalf("out-of-window observation decided a scenario: %+v / %+v", ev.States[0], ev.Delta)
	}
}
