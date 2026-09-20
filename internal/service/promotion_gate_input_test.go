package service

import (
	"testing"

	"insight-lab/internal/domain"
)

func satisfiedArtifactFacts() PromotionArtifactFacts {
	return PromotionArtifactFacts{
		Stage:                            domain.StageValidation,
		ProvenanceMode:                   string(ExecutionModeDeterministic),
		DatasetHashesPresent:             true,
		GroundedObservations:             4,
		TotalObservationCandidates:       4,
		AllInsightsHaveExpectationBasis:  true,
		AllInsightsHaveEvidence:          true,
		CounterEvidenceSearched:          true,
		AnyLimitationsDisclosed:          true,
		ResearchGapsDisclosed:            true,
		UnresolvableConclusionsDisclosed: true,
		CompatibilityWarningsPresent:     false,
	}
}

func TestBuildPublicationChecklistDerivesMechanicalItemsFromArtifactFacts(t *testing.T) {
	got := BuildPublicationChecklist(satisfiedArtifactFacts(), domain.PublicationChecklist{})

	want := domain.PublicationChecklist{
		SourceProvenanceComplete:              true,
		DeterministicCalculationsReproducible: true,
		ObservationGrounded:                   true,
		ExpectationProvenanceVisible:          true,
		ResearchStageVisible:                  true,
		ClaimEvidenceMappingComplete:          true,
		CounterEvidenceSearched:               true,
		LimitationsPresent:                    true,
		ResearchGapsDisclosed:                 true,
		UnresolvableConclusionsDisclosed:      true,
		NoHiddenPopulationUnitPeriodMismatch:  true,
	}
	if got != want {
		t.Fatalf("mechanical checklist mismatch:\ngot  %+v\nwant %+v", got, want)
	}
}

func TestBuildPublicationChecklistNeverInfersJudgmentItemsFromArtifactFacts(t *testing.T) {
	got := BuildPublicationChecklist(satisfiedArtifactFacts(), domain.PublicationChecklist{})

	if got.CompetingHypothesisConsidered || got.IndependentValidationStatusAccurate || got.DecisionReadinessHonestlyStated {
		t.Fatalf("judgment items must never be inferred from mechanical facts alone, got %+v", got)
	}
}

func TestBuildPublicationChecklistPassesThroughHumanJudgmentItems(t *testing.T) {
	human := domain.PublicationChecklist{
		CompetingHypothesisConsidered:       true,
		IndependentValidationStatusAccurate: true,
		DecisionReadinessHonestlyStated:     true,
	}
	got := BuildPublicationChecklist(PromotionArtifactFacts{}, human)

	if !got.CompetingHypothesisConsidered || !got.IndependentValidationStatusAccurate || !got.DecisionReadinessHonestlyStated {
		t.Fatalf("an explicit human judgment must be carried through, got %+v", got)
	}
}

func TestBuildPublicationChecklistReportsCompatibilityWarningsAsAHiddenMismatchRisk(t *testing.T) {
	facts := satisfiedArtifactFacts()
	facts.CompatibilityWarningsPresent = true

	got := BuildPublicationChecklist(facts, domain.PublicationChecklist{})

	if got.NoHiddenPopulationUnitPeriodMismatch {
		t.Fatalf("an unresolved dataset compatibility warning must block this checklist item")
	}
}
