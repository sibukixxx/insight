//go:build golden

package goldenset

import (
	"errors"
	"testing"

	"insight-lab/internal/domain"
	"insight-lab/internal/service"
)

type sharedEvalCase struct {
	SchemaVersion string `json:"schemaVersion"`
	CaseID        string `json:"caseId"`
	Domain        string `json:"domain"`
	Input         struct {
		References        []string `json:"references"`
		ContextReferences []string `json:"contextReferences"`
	} `json:"input"`
	Expected struct {
		Properties       []string `json:"properties"`
		RequiredEvidence []string `json:"requiredEvidence"`
		ProhibitedClaims []string `json:"prohibitedClaims"`
		SchemaChecks     []string `json:"schemaChecks"`
		QualityChecks    []string `json:"qualityChecks"`
	} `json:"expected"`
	HumanReview struct {
		Required bool   `json:"required"`
		Result   string `json:"result"`
	} `json:"humanReview"`
	Outcome struct {
		Status  string   `json:"status"`
		Reasons []string `json:"reasons"`
	} `json:"outcome"`
}

func loadSharedEvalCase(t *testing.T, name string) sharedEvalCase {
	t.Helper()
	var c sharedEvalCase
	loadFixture(t, name, &c)
	if c.SchemaVersion != "1" || c.CaseID == "" || c.Domain != "insight" {
		t.Fatalf("fixture does not satisfy Shared Eval Contract v1 envelope: %+v", c)
	}
	if len(c.Expected.Properties) == 0 || c.Outcome.Status == "" {
		t.Fatalf("fixture must declare expected properties and outcome: %+v", c)
	}
	return c
}

func TestGoldenNonObviousConnectionCannotPromoteNarrativeCoherence(t *testing.T) {
	c := loadSharedEvalCase(t, "non_obvious_unsupported.json")
	insight := domain.Insight{
		ID: "i-1",
		Connection: domain.InsightConnection{
			Kind:      domain.ConnectionMechanismCandidate,
			Statement: "A appears connected to C through B",
			Sources:   []domain.InsightReference{{Kind: domain.InsightRefObservation, ID: "obs-a"}},
			Targets:   []domain.InsightReference{{Kind: domain.InsightRefObservation, ID: "obs-b"}},
		},
		Mechanism: domain.MechanismCandidate{
			Statement: "B may bridge A and C",
			Steps: []domain.MechanismStep{{
				Statement:       "B transmits the effect",
				MissingEvidence: append([]string(nil), c.Expected.RequiredEvidence...),
			}},
		},
		Generalization: domain.InsightGeneralization{Status: domain.GeneralizationCandidate},
	}
	if len(insight.Mechanism.Steps[0].MissingEvidence) == 0 {
		t.Fatal("unsupported bridge must remain explicit")
	}
	in := domain.PromotionGateInput{
		Contribution:         domain.ContributionNovelMismatch,
		HumanReviewCompleted: false,
	}
	err := domain.PromotionHumanReviewRequired.Transition(domain.PromotionPublicationReady, in)
	if err == nil {
		t.Fatal("unsupported non-obvious narrative must not become PUBLICATION_READY")
	}
	if !errors.Is(err, domain.ErrPromotionChecklistIncomplete) && !errors.Is(err, domain.ErrPromotionHumanReviewRequired) {
		t.Fatalf("unexpected promotion error: %v", err)
	}
}

func TestGoldenInsightDeltaChangeTracksInputAndResultWithoutCausalClaim(t *testing.T) {
	loadSharedEvalCase(t, "insight_delta_change.json")
	before := domain.ResearchIteration{
		ID: "it-1",
		InputSnapshot: domain.InputSetSnapshot{
			Variables:          []string{"population"},
			EvidenceReferences: []string{"population.csv"},
		},
		HypothesisStates: []domain.HypothesisState{{
			HypothesisID: "h1", ComparisonKey: "migration", ValidationStatus: domain.ValidationPlausible,
		}},
		InsightIDs: []string{"h1"},
	}
	after := domain.ResearchIteration{
		ID: "it-2",
		InputSnapshot: domain.InputSetSnapshot{
			Variables:          []string{"population", "income"},
			EvidenceReferences: []string{"population.csv", "income.csv"},
		},
		HypothesisStates: []domain.HypothesisState{{
			HypothesisID: "h1", ComparisonKey: "migration", ValidationStatus: domain.ValidationInsufficientEvidence,
		}},
		InsightIDs: []string{"h1", "h2"},
	}
	after.HypothesisChanges = service.CompareHypothesisStates(before.HypothesisStates, after.HypothesisStates)
	delta := service.CompareResearchIterations(before, after)
	if len(delta.Input.Variables.Added) != 1 || delta.Input.Variables.Added[0] != "income" {
		t.Fatalf("variable delta missing: %+v", delta.Input)
	}
	if len(delta.Result.HypothesisChanges) != 1 {
		t.Fatalf("hypothesis change missing: %+v", delta.Result)
	}
	if len(delta.Explanation) == 0 {
		t.Fatal("delta must carry non-causal interpretation note")
	}
}

func TestGoldenInsightDeltaAllowsNoChange(t *testing.T) {
	loadSharedEvalCase(t, "insight_delta_no_change.json")
	before := domain.ResearchIteration{
		ID:            "it-1",
		InputSnapshot: domain.InputSetSnapshot{Variables: []string{"population"}},
		InsightIDs:    []string{"h1"},
	}
	after := domain.ResearchIteration{
		ID:            "it-2",
		InputSnapshot: domain.InputSetSnapshot{Variables: []string{"population"}},
		InsightIDs:    []string{"h1"},
	}
	delta := service.CompareResearchIterations(before, after)
	if len(delta.Input.Variables.Added) != 0 || len(delta.Input.Variables.Removed) != 0 {
		t.Fatalf("no-change case must remain empty: %+v", delta.Input)
	}
	if len(delta.Result.InsightIDsAdded) != 0 || len(delta.Result.InsightIDsRemoved) != 0 {
		t.Fatalf("no-change result must remain empty: %+v", delta.Result)
	}
}

func TestGoldenResearchReviewClaimStaysSeparateFromEvidence(t *testing.T) {
	c := loadSharedEvalCase(t, "research_review_claim.json")
	claim := domain.ResearchClaim{
		ID:                 "claim-1",
		Statement:          "The policy caused the increase",
		SourceReference:    c.Input.References[0],
		EvidenceReferences: []string{c.Input.References[1]},
	}
	if err := domain.ValidateAnalysisModeInput(domain.AnalysisModeResearchReview, []domain.InputArtifact{
		{Reference: c.Input.References[0], Kind: domain.ArtifactResearchReport},
		{Reference: c.Input.References[1], Kind: domain.ArtifactDataset},
	}, []domain.ResearchClaim{claim}); err != nil {
		t.Fatal(err)
	}
	if claim.SourceReference == claim.EvidenceReferences[0] {
		t.Fatal("claim source must not silently become its underlying primary evidence")
	}
}
