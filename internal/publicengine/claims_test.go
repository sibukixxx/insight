package publicengine

import (
	"strings"
	"testing"
)

func TestPublicClaimsValidationRejectsMalformedClaims(t *testing.T) {
	long := strings.Repeat("x", 2001)
	many := make([]ResearchClaim, 51)
	for i := range many {
		many[i] = ResearchClaim{ID: "c" + strings.Repeat("i", i), Statement: "s"}
	}
	for name, claims := range map[string][]ResearchClaim{
		"missing id":        {{Statement: "s"}},
		"missing statement": {{ID: "c1"}},
		"long statement":    {{ID: "c1", Statement: long}},
		"empty reference":   {{ID: "c1", Statement: "s", EvidenceReferences: []string{" "}}},
		"too many claims":   many,
		"reused id":         {{ID: "c1", Statement: "a"}, {ID: "c1", Statement: "b"}},
	} {
		if _, err := toDomainClaims(claims); AsError(err).Code != CodeInvalidRequest {
			t.Errorf("%s: err = %v, want INVALID_REQUEST", name, err)
		}
	}
	got, err := toDomainClaims([]ResearchClaim{{ID: " c1 ", Statement: " Visits rose. ", EvidenceReferences: []string{"e1"}, HypothesisReferences: []string{"h1"}, Assumptions: []string{"a"}}})
	if err != nil || len(got) != 1 || got[0].ID != "c1" || got[0].Statement != "Visits rose." || got[0].HypothesisReferences[0] != "h1" {
		t.Fatalf("got %+v, err %v", got, err)
	}
}
