package domain

import "testing"

func TestReasoningProfileIsAnIndependentExplicitAxis(t *testing.T) {
	for _, p := range []ReasoningProfile{ReasoningProfileGeneralResearch, ReasoningProfileCustomerInsight} {
		if !p.Valid() {
			t.Fatalf("expected profile %q to be valid", p)
		}
	}
	if ReasoningProfile("").Normalize() != ReasoningProfileGeneralResearch {
		t.Fatal("an omitted profile must keep the domain-neutral GENERAL_RESEARCH behavior")
	}
	for _, other := range []string{"DISCOVERY", "MODEL_BACKED", "STANDARD", "VALIDATION", "customer_insight"} {
		if ReasoningProfile(other).Valid() {
			t.Fatalf("%q from another axis (or wrong case) must not be accepted as a reasoning profile", other)
		}
	}
	if got := ReasoningProfiles(); len(got) != 2 || got[0] != ReasoningProfileGeneralResearch {
		t.Fatalf("supported profiles = %v, want GENERAL_RESEARCH first", got)
	}
}
