package service

import (
	"testing"

	"insight-lab/internal/domain"
)

func TestCompareResearchIterationsTracksInputAndInterpretationChange(t *testing.T) {
	before := domain.ResearchIteration{
		ID: "it-1",
		InputSnapshot: domain.InputSetSnapshot{Variables: []string{"population"}, EvidenceReferences: []string{"population.csv"}},
		InsightIDs: []string{"h1"},
		ResearchGaps: []domain.ResearchGap{{ID: "g1", Need: "income"}},
	}
	after := domain.ResearchIteration{
		ID: "it-2",
		InputSnapshot: domain.InputSetSnapshot{Variables: []string{"population", "income"}, EvidenceReferences: []string{"population.csv", "income.csv"}},
		InsightIDs: []string{"h1", "h2"},
		ResearchGaps: []domain.ResearchGap{{ID: "g1", Need: "income", Resolved: true}},
		HypothesisChanges: []domain.HypothesisChange{{HypothesisID: "h1", Evolution: domain.HypothesisWeakened, Reason: "new evidence"}},
	}
	d := CompareResearchIterations(before, after)
	if len(d.Input.Variables.Added) != 1 || d.Input.Variables.Added[0] != "income" {
		t.Fatalf("expected income variable delta: %+v", d.Input)
	}
	if len(d.Result.InsightIDsAdded) != 1 || d.Result.InsightIDsAdded[0] != "h2" {
		t.Fatalf("expected new insight delta: %+v", d.Result)
	}
	if len(d.Result.ResearchGapIDsResolved) != 1 || d.Result.ResearchGapIDsResolved[0] != "g1" {
		t.Fatalf("expected resolved gap: %+v", d.Result)
	}
	if len(d.Explanation) == 0 {
		t.Fatal("delta must explain that temporal input/output change is not causality")
	}
}
