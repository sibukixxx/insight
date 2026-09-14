package service

import (
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
)

func TestBuildResearchIterationPromotesMissingEvidenceWithoutCausalUpgrade(t *testing.T) {
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	insights := []*domain.Insight{{
		ID: "hyp-1", SurprisingFact: "treated unit rose", HypothesisSetID: "set-1",
		MissingEvidence: []string{"comparison trend before treatment"},
		CausalStatus: domain.CausalHypothesis, IdentificationStatus: domain.IdentificationNotIdentified,
	}}

	got := BuildResearchIteration(1, "Did treatment cause the increase?", []string{"synthetic.csv"}, insights, now)
	if len(got.ResearchGaps) != 1 || len(got.DataRequirements) != 1 {
		t.Fatalf("expected structured gap and requirement: %+v", got)
	}
	if got.DataRequirements[0].Need != "comparison trend before treatment" {
		t.Fatalf("missing evidence must be preserved: %+v", got.DataRequirements[0])
	}
	if len(got.WhatWeCannotConclude) != 1 || !strings.Contains(got.WhatWeCannotConclude[0], "not identified") {
		t.Fatalf("unresolved identification must be explicit: %+v", got.WhatWeCannotConclude)
	}
	if insights[0].CausalStatus != domain.CausalHypothesis || insights[0].IdentificationStatus != domain.IdentificationNotIdentified {
		t.Fatal("research projection must not upgrade causal state")
	}
}

func TestBuildResearchIterationDeduplicatesHypothesisSets(t *testing.T) {
	got := BuildResearchIteration(2, "q", nil, []*domain.Insight{
		{ID: "h1", HypothesisSetID: "set-a"},
		{ID: "h2", HypothesisSetID: "set-a"},
		{ID: "h3", HypothesisSetID: "set-b"},
	}, time.Now())
	if len(got.HypothesisSetIDs) != 2 || got.HypothesisSetIDs[0] != "set-a" || got.HypothesisSetIDs[1] != "set-b" {
		t.Fatalf("unexpected sets: %+v", got.HypothesisSetIDs)
	}
}
