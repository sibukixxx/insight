package service

import (
	"strings"
	"testing"

	"insight-lab/internal/domain"
)

func TestGeneralResearchHasNoCustomerProfileInstruction(t *testing.T) {
	if got := reasoningProfileInstruction(domain.ReasoningGeneralResearch, "hypothesis_generation"); got != "" {
		t.Fatalf("GENERAL_RESEARCH must not add customer semantics: %q", got)
	}
}

func TestCustomerInsightAddsExplicitSpecialization(t *testing.T) {
	h := strings.ToLower(reasoningProfileInstruction(domain.ReasoningCustomerInsight, "hypothesis_generation"))
	for _, want := range []string{"customer_insight", "statedneed", "latentneed", "jtbd", "counter-evidence"} {
		if !strings.Contains(h, want) {
			t.Fatalf("customer hypothesis instruction missing %q: %s", want, h)
		}
	}
	w := strings.ToLower(reasoningProfileInstruction(domain.ReasoningCustomerInsight, "insight_writeup"))
	for _, want := range []string{"productopportunity", "monetizationangle", "never evidence"} {
		if !strings.Contains(w, want) {
			t.Fatalf("customer synthesis instruction missing %q: %s", want, w)
		}
	}
}

func TestGeneralResearchNeverRunsCustomerNeedQualityRules(t *testing.T) {
	in := QualityInput{
		ReasoningProfile: domain.ReasoningGeneralResearch,
		StatedNeed: "安心したい", LatentNeed: "安心したい",
		Expectation: "baseline", SurprisingFact: "change",
		Patterns: []*domain.Pattern{{Kind: domain.PatternDeviation}},
	}
	flags := AssessQuality(in)
	if hasCode(flags, domain.QualityStatedNeedEcho) || hasCode(flags, domain.QualityGenericTerm) {
		t.Fatalf("generic research got customer-specific flags: %+v", flags)
	}

	in.ReasoningProfile = domain.ReasoningCustomerInsight
	flags = AssessQuality(in)
	if !hasCode(flags, domain.QualityStatedNeedEcho) || !hasCode(flags, domain.QualityGenericTerm) {
		t.Fatalf("customer profile must retain customer-specific checks: %+v", flags)
	}
}
