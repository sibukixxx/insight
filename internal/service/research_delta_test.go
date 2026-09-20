package service

import (
	"testing"

	"insight-lab/internal/domain"
)

func TestCompareResearchIterationsTracksInputsAndResults(t *testing.T) {
	previous := domain.ResearchIteration{
		ID: "it-1",
		InputSnapshot: domain.ResearchInputSnapshot{Variables: []string{"population"}, EvidenceReferences: []string{"population.csv"}},
		InsightIDs: []string{"h1"},
		HypothesisStates: []domain.HypothesisState{{
			HypothesisID: "h1", ComparisonKey: "housing", ValidationStatus: domain.ValidationPlausible,
			IdentificationStatus: domain.IdentificationNotIdentified,
		}},
	}
	current := domain.ResearchIteration{
		ID: "it-2",
		InputSnapshot: domain.ResearchInputSnapshot{Variables: []string{"population", "housing_price"}, EvidenceReferences: []string{"population.csv", "housing.csv"}},
		InsightIDs: []string{"h1b", "h2"},
		HypothesisStates: []domain.HypothesisState{
			{HypothesisID: "h1b", ComparisonKey: "housing", ValidationStatus: domain.ValidationPartiallySupported, IdentificationStatus: domain.IdentificationNotIdentified},
			{HypothesisID: "h2", ComparisonKey: "income interaction", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified},
		},
	}
	current.HypothesisChanges = CompareHypothesisStates(previous.HypothesisStates, current.HypothesisStates)
	got := CompareResearchIterations(previous, current)
	if got.FromIterationID != "it-1" || got.ToIterationID != "it-2" {
		t.Fatalf("iteration lineage lost: %+v", got)
	}
	var variableAdded, evidenceAdded bool
	for _, change := range got.InputChanges {
		if change.Category == "variable" && change.Kind == domain.DeltaAdded && change.Value == "housing_price" {
			variableAdded = true
		}
		if change.Category == "evidence" && change.Kind == domain.DeltaAdded && change.Value == "housing.csv" {
			evidenceAdded = true
		}
	}
	if !variableAdded || !evidenceAdded {
		t.Fatalf("input delta incomplete: %+v", got.InputChanges)
	}
	if len(got.Result.HypothesisChanges) == 0 {
		t.Fatal("result delta must reuse hypothesis evolution")
	}
}

func TestCompareResearchIterationsAllowsNoChange(t *testing.T) {
	previous := domain.ResearchIteration{ID: "it-1", InputSnapshot: domain.ResearchInputSnapshot{Variables: []string{"x"}}, InsightIDs: []string{"h1"}}
	current := domain.ResearchIteration{ID: "it-2", InputSnapshot: domain.ResearchInputSnapshot{Variables: []string{"x"}}, InsightIDs: []string{"h1"}}
	got := CompareResearchIterations(previous, current)
	if len(got.InputChanges) != 0 || len(got.Result.InsightAdded) != 0 || len(got.Result.InsightRemoved) != 0 {
		t.Fatalf("unchanged iterations must produce an empty delta: %+v", got)
	}
}
