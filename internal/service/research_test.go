package service

import (
	"testing"

	"insight-lab/internal/domain"
)

func TestPlanDataRequirementIsProviderNeutral(t *testing.T) {
	gap := ResearchGapFromMissingEvidence("gap-1", domain.ResearchGapConfounder, "population by year", "population may explain both exposure and outcome", []string{"h1"})
	req, err := PlanDataRequirement(gap, []string{"municipality", "year"}, "2018-2026", "official statistics")
	if err != nil {
		t.Fatal(err)
	}
	if req.Need != gap.Need || req.SuggestedSourceCategory != "official statistics" {
		t.Fatalf("unexpected requirement: %+v", req)
	}
}

func TestPlanDataRequirementRejectsUnidentifiedGap(t *testing.T) {
	_, err := PlanDataRequirement(domain.ResearchGap{Need: "comparison trend"}, nil, "", "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}
