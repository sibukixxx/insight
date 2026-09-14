package service

import (
	"encoding/json"
	"fmt"
	"os"
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
		CausalStatus:    domain.CausalHypothesis, IdentificationStatus: domain.IdentificationNotIdentified,
	}}

	got := BuildResearchIteration(1, "Did treatment cause the increase?", []string{"synthetic.csv"}, insights, now)
	if len(got.ResearchGaps) != 1 || len(got.DataRequirements) != 1 {
		t.Fatalf("expected structured gap and requirement: %+v", got)
	}
	if got.ResearchGaps[0].Resolved {
		t.Fatal("new missing-evidence gap must remain unresolved")
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

func TestGoldenPolicyDogfoodRemainsNotIdentified(t *testing.T) {
	var fixture struct {
		Question               string   `json:"question"`
		Hypotheses             []string `json:"hypotheses"`
		MissingEvidence        []string `json:"missingEvidence"`
		ExpectedIdentification string   `json:"expectedIdentification"`
	}
	raw, err := os.ReadFile("testdata/research_loop_policy.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	insights := make([]*domain.Insight, 0, len(fixture.Hypotheses))
	for i, title := range fixture.Hypotheses {
		insights = append(insights, &domain.Insight{ID: fmt.Sprintf("h%d", i+1), Title: title, SurprisingFact: "treated and comparison outcomes both increased", HypothesisSetID: "set1", MissingEvidence: fixture.MissingEvidence, CausalStatus: domain.CausalHypothesis, ValidationStatus: domain.ValidationInsufficientEvidence, IdentificationStatus: domain.IdentificationNotIdentified})
	}
	got := BuildResearchIteration(1, fixture.Question, []string{"synthetic policy fixture"}, insights, time.Now())
	if len(got.InsightIDs) != 3 || len(got.HypothesisSetIDs) != 1 {
		t.Fatalf("competing hypotheses were not retained: %+v", got)
	}
	if len(got.WhatWeCannotConclude) != 3 {
		t.Fatalf("identification boundary missing: %+v", got.WhatWeCannotConclude)
	}
	for _, state := range got.HypothesisStates {
		if string(state.IdentificationStatus) != fixture.ExpectedIdentification {
			t.Fatalf("causality was promoted: %+v", state)
		}
	}
}

func TestResearchGapClassificationIsDeterministicAndUnknownStaysOther(t *testing.T) {
	for input, want := range map[string]domain.ResearchGapCategory{"control regions": domain.ResearchGapComparison, "pre-period trend": domain.ResearchGapPrePeriod, "exact policy timing": domain.ResearchGapTiming, "population by year": domain.ResearchGapConfounder, "unfamiliar unresolved question": domain.ResearchGapOther} {
		if got := ClassifyResearchGap(input); got != want {
			t.Errorf("ClassifyResearchGap(%q)=%s want %s", input, got, want)
		}
	}
}

func TestCompareHypothesisStatesAuditsEvidenceChangeWithoutCausalPromotion(t *testing.T) {
	previous := []domain.HypothesisState{{HypothesisID: "old-h1", ComparisonKey: "treatment effect", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified}}
	current := []domain.HypothesisState{{HypothesisID: "new-h1", ComparisonKey: "treatment effect", ValidationStatus: domain.ValidationPartiallySupported, IdentificationStatus: domain.IdentificationNotIdentified}}
	changes := CompareHypothesisStates(previous, current)
	if len(changes) != 1 || changes[0].Evolution != domain.HypothesisStrengthened {
		t.Fatalf("unexpected evolution: %+v", changes)
	}
	if current[0].IdentificationStatus != domain.IdentificationNotIdentified {
		t.Fatal("history comparison promoted identification")
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
