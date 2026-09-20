//go:build golden

package goldenset

import (
	"insight-lab/internal/domain"
	"insight-lab/internal/service"
	"testing"
	"time"
)

func TestGoldenPromotionRequiresSeparatePublicationAndHumanReview(t *testing.T) {
	checklist := domain.PublicationChecklist{
		SourceProvenanceComplete: true, DeterministicCalculationsReproducible: true,
		ObservationGrounded: true, ExpectationProvenanceVisible: true, ResearchStageVisible: true,
		ClaimEvidenceMappingComplete: true, CompetingHypothesisConsidered: true, CounterEvidenceSearched: true,
		LimitationsPresent: true, ResearchGapsDisclosed: true, UnresolvableConclusionsDisclosed: true,
		NoHiddenPopulationUnitPeriodMismatch: true, IndependentValidationStatusAccurate: true, DecisionReadinessHonestlyStated: true,
	}
	for _, contribution := range []domain.ContributionType{domain.ContributionCorrection, domain.ContributionReplication, domain.ContributionInconclusiveButDecisionRelevant} {
		in := domain.PromotionGateInput{Contribution: contribution, Checklist: checklist, HumanReviewCompleted: true}
		result := service.AssessPromotion(domain.PromotionDraft, in, time.Time{})
		if result.State != domain.PromotionPublicationReady {
			t.Fatalf("honest non-novel finding must reach READY, never auto-publish: %+v", result)
		}
		in.HumanReviewCompleted = false
		if result := service.AssessPromotion(domain.PromotionDraft, in, time.Time{}); result.State != domain.PromotionHumanReviewRequired {
			t.Fatalf("human approval bypassed: %+v", result)
		}
		if err := domain.PromotionPublicationReady.Transition(domain.PromotionPublished, in); err == nil {
			t.Fatal("publication accepted missing human review")
		}
	}
}
