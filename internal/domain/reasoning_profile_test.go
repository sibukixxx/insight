package domain

import "testing"

func TestReasoningProfileNormalizeAndValidate(t *testing.T) {
	if got := ReasoningProfile("").Normalize(); got != ReasoningGeneralResearch {
		t.Fatalf("empty profile = %q, want %q", got, ReasoningGeneralResearch)
	}
	for _, p := range []ReasoningProfile{ReasoningGeneralResearch, ReasoningCustomerInsight, ""} {
		if !p.Valid() {
			t.Fatalf("expected %q to be valid", p)
		}
	}
	if ReasoningProfile("SALES_ONLY").Valid() {
		t.Fatal("unknown profile must be invalid")
	}
}
