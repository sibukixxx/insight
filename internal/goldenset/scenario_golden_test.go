//go:build golden

package goldenset

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
)

// Golden cases for #66: false certainty, post-hoc leakage, missing horizon
// and conflicting scenarios, on the Japan -> EU export fixture.

func loadScenarioFixture(t *testing.T) domain.ScenarioSet {
	t.Helper()
	data, err := os.ReadFile("../domain/testdata/scenario_japan_eu.json")
	if err != nil {
		t.Fatal(err)
	}
	var s domain.ScenarioSet
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func ts(v string) time.Time {
	t, _ := time.Parse(time.RFC3339, v)
	return t
}

func TestGoldenScenarioFalseCertaintyRejectsProbabilityWithoutBasis(t *testing.T) {
	s := loadScenarioFixture(t)
	s.Scenarios[0].Probability = &domain.ScenarioProbability{Value: 0.9, Basis: "model judgement"}
	if err := s.Validate(); !errors.Is(err, domain.ErrScenarioInvalid) {
		t.Fatalf("probability without sources accepted: %v", err)
	}
	ev, err := domain.EvaluateScenarios(loadScenarioFixture(t), nil, domain.NewObservationInput{Observations: []domain.IndicatorObservation{
		{ExpectationID: "S1-E1", Outcome: domain.OutcomeConsistent, EvidenceRef: "comext:2026-10", ObservedAt: ts("2026-11-01T00:00:00Z")},
	}}, "ev", "", ts("2026-11-02T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if ev.States[0].Status != domain.ScenarioConsistentSoFar {
		t.Fatalf("one consistent observation must read CONSISTENT_SO_FAR, never confirmed: %s", ev.States[0].Status)
	}
}

func TestGoldenScenarioPostHocLeakageIsRejectedOrInconclusive(t *testing.T) {
	s := loadScenarioFixture(t)
	s.Scenarios[1].Expectations[0].ObservedDataAvailableAtCreation = true
	if err := s.Validate(); err == nil || !strings.Contains(err.Error(), "post-hoc leakage") {
		t.Fatalf("prior-labelled expectation written after seeing data accepted: %v", err)
	}
	ev, err := domain.EvaluateScenarios(loadScenarioFixture(t), nil, domain.NewObservationInput{Observations: []domain.IndicatorObservation{
		{ExpectationID: "S2-E1", Outcome: domain.OutcomeConsistent, EvidenceRef: "old", ObservedAt: ts("2026-05-01T00:00:00Z")},
	}}, "ev", "", ts("2026-07-02T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if ev.States[1].Status == domain.ScenarioConsistentSoFar {
		t.Fatal("observation available before the scenario was written confirmed it")
	}
}

func TestGoldenScenarioMissingHorizonIsRejected(t *testing.T) {
	s := loadScenarioFixture(t)
	s.Scenarios[3].Horizon.End = time.Time{}
	if err := s.Validate(); !errors.Is(err, domain.ErrScenarioInvalid) {
		t.Fatalf("scenario without horizon accepted: %v", err)
	}
}

func TestGoldenConflictingScenariosKeepBothStatesWithoutWinner(t *testing.T) {
	s := loadScenarioFixture(t)
	ev, err := domain.EvaluateScenarios(s, nil, domain.NewObservationInput{Observations: []domain.IndicatorObservation{
		{ExpectationID: "S3-E1", Outcome: domain.OutcomeConsistent, EvidenceRef: "comext:share", ObservedAt: ts("2026-12-01T00:00:00Z")},
		{ExpectationID: "S4-E1", Outcome: domain.OutcomeContradicts, EvidenceRef: "comext:total", ObservedAt: ts("2026-12-01T00:00:00Z")},
	}}, "ev", "", ts("2026-12-02T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if ev.States[2].Status != domain.ScenarioConsistentSoFar || ev.States[3].Status != domain.ScenarioContradicted {
		t.Fatalf("states = %+v", ev.States)
	}
	for _, id := range []string{"S1", "S2", "S5"} {
		found := false
		for _, u := range ev.Delta.Unchanged {
			found = found || u == id
		}
		if !found {
			t.Fatalf("untouched scenario %s must stay reported as unchanged: %+v", id, ev.Delta)
		}
	}
	raw, _ := json.Marshal(ev)
	for _, banned := range []string{"winner", "mostLikely", "\"best"} {
		if strings.Contains(string(raw), banned) {
			t.Fatalf("evaluation names a preferred future: %s", banned)
		}
	}
}
