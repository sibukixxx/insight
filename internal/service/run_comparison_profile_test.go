package service

import (
	"encoding/json"
	"testing"

	"insight-lab/internal/domain"
	"insight-lab/internal/execution"
)

func withProfile(t *testing.T, a *domain.Analysis, requested, resolved execution.Profile) {
	t.Helper()
	var s ExecutionSnapshot
	if err := json.Unmarshal([]byte(a.ExecutionSnapshot), &s); err != nil {
		t.Fatal(err)
	}
	s.ExecutionProfile = &execution.Resolution{Requested: requested, Resolved: resolved, Reason: "test", StrategyVersion: "t/1"}
	raw, _ := json.Marshal(s)
	a.ExecutionSnapshot = string(raw)
}

// The execution profile is recorded but excluded from the execution
// fingerprint by design (#91: it must not change research semantics). A
// comparison keeps the execution state from fingerprints and lists profile
// differences separately as informational, so a profile-only difference is
// visible without being attributed as an instrument change.
func TestCompareListsProfileDifferencesAsInformationalWithoutChangingState(t *testing.T) {
	a := run(t, "a1", "c1", "", "same evidence")
	b := run(t, "a2", "c1", "", "same evidence")
	withProfile(t, a, execution.ProfileAuto, execution.ProfileLight)
	withProfile(t, b, execution.ProfileHeavy, execution.ProfileHeavy)

	c := CompareAnalysisRuns(RunComparisonInput{Analysis: a}, RunComparisonInput{Analysis: b}, nil)
	if c.Execution.State != AxisSame || len(c.Execution.Changes) != 0 {
		t.Fatalf("profile-only difference must not change execution state: %+v", c.Execution)
	}
	got := map[string]FieldChange{}
	for _, ch := range c.Execution.Informational {
		got[ch.Field] = ch
	}
	if got["executionProfile.resolved"].From != "LIGHT" || got["executionProfile.resolved"].To != "HEAVY" || got["executionProfile.requested"].To != "HEAVY" {
		t.Fatalf("informational changes = %+v", c.Execution.Informational)
	}
}
