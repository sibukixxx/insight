package domain

import "testing"

func TestResolveAnalysisModeFromArtifactClass(t *testing.T) {
	tests := []struct {
		class ArtifactClass
		want  AnalysisMode
	}{
		{ArtifactClassRawEvidence, AnalysisModeDiscovery},
		{ArtifactClassStructuredDataset, AnalysisModeDatasetAnalysis},
		{ArtifactClassResearchArtifact, AnalysisModeResearchReview},
	}
	for _, tc := range tests {
		got, err := ResolveAnalysisMode("", tc.class)
		if err != nil || got != tc.want {
			t.Fatalf("ResolveAnalysisMode(%q) = %q, %v; want %q", tc.class, got, err, tc.want)
		}
	}
}

func TestResolveAnalysisModeRequiresExplicitChoiceForMixedArtifacts(t *testing.T) {
	if _, err := ResolveAnalysisMode("", ArtifactClassMixed); err == nil {
		t.Fatal("mixed artifacts must not silently pick an analysis mode")
	}
	got, err := ResolveAnalysisMode(AnalysisModeResearchReview, ArtifactClassMixed)
	if err != nil || got != AnalysisModeResearchReview {
		t.Fatalf("explicit mode must resolve mixed artifacts: got %q err=%v", got, err)
	}
}

func TestClaimWithoutUnderlyingEvidenceIsNotEvidence(t *testing.T) {
	claim := Claim{ID: "c1", Statement: "the intervention caused the increase", Kind: ClaimInterpretation, SourceReference: "report.pdf"}
	if claim.HasUnderlyingEvidence() {
		t.Fatal("an imported claim without underlying evidence references must not be evidence")
	}
	claim.UnderlyingEvidenceReferences = []string{"dataset:official-1"}
	if !claim.HasUnderlyingEvidence() {
		t.Fatal("underlying evidence references must remain inspectable")
	}
}
