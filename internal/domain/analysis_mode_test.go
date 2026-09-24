package domain

import "testing"

func TestAnalysisModeAxesAreExplicit(t *testing.T) {
	for _, mode := range []AnalysisMode{AnalysisModeDiscovery, AnalysisModeDatasetAnalysis, AnalysisModeResearchReview} {
		if !mode.Valid() {
			t.Fatalf("expected semantic mode %q to be valid", mode)
		}
	}
	if AnalysisMode("").Normalize() != AnalysisModeDiscovery {
		t.Fatal("empty semantic mode must preserve existing behavior as DISCOVERY")
	}
	if AnalysisMode("MODEL_BACKED").Valid() {
		t.Fatal("execution mode must not be accepted as semantic analysis mode")
	}
}

func TestResearchClaimIsNotEvidence(t *testing.T) {
	claim := ResearchClaim{
		ID:                 "claim-1",
		Statement:          "The policy caused the increase",
		SourceReference:    "external-report.pdf",
		EvidenceReferences: []string{"official-table.csv"},
	}
	if err := claim.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAnalysisModeInput(AnalysisModeResearchReview, []InputArtifact{{
		Reference: "external-report.pdf", Kind: ArtifactResearchReport,
	}}, []ResearchClaim{claim}); err != nil {
		t.Fatal(err)
	}
}
