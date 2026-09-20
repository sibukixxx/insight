package service

import (
	"testing"
	"time"

	"insight-lab/internal/domain"
)

func fullyReadyPromotionInput() domain.PromotionGateInput {
	return domain.PromotionGateInput{
		Contribution: domain.ContributionReplication,
		Checklist: domain.PublicationChecklist{
			SourceProvenanceComplete:              true,
			DeterministicCalculationsReproducible: true,
			ObservationGrounded:                   true,
			ExpectationProvenanceVisible:          true,
			ResearchStageVisible:                  true,
			ClaimEvidenceMappingComplete:          true,
			CompetingHypothesisConsidered:         true,
			CounterEvidenceSearched:               true,
			LimitationsPresent:                    true,
			ResearchGapsDisclosed:                 true,
			UnresolvableConclusionsDisclosed:      true,
			NoHiddenPopulationUnitPeriodMismatch:  true,
			IndependentValidationStatusAccurate:   true,
			DecisionReadinessHonestlyStated:       true,
		},
		HumanReviewCompleted: true,
	}
}

func TestAssessPromotionShouldAdvanceAllTheWayWhenEveryGateIsSatisfied(t *testing.T) {
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	got := AssessPromotion(domain.PromotionDraft, fullyReadyPromotionInput(), now)
	if got.State != domain.PromotionPublished {
		t.Fatalf("got state %q, want PUBLISHED: reasons %v", got.State, got.Reasons)
	}
	if len(got.Reasons) != 0 {
		t.Fatalf("expected no blocking reason, got %v", got.Reasons)
	}
	if !got.AssessedAt.Equal(now) {
		t.Fatalf("got assessedAt %v, want %v", got.AssessedAt, now)
	}
}

func TestAssessPromotionShouldStopAtFirstUnsatisfiedGateAndExplainWhy(t *testing.T) {
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	in := fullyReadyPromotionInput()
	in.HumanReviewCompleted = false

	got := AssessPromotion(domain.PromotionDraft, in, now)
	if got.State != domain.PromotionHumanReviewRequired {
		t.Fatalf("got state %q, want HUMAN_REVIEW_REQUIRED", got.State)
	}
	if len(got.Reasons) != 1 {
		t.Fatalf("expected exactly one blocking reason, got %v", got.Reasons)
	}
}

func TestAssessPromotionShouldNotAdvanceWithoutAQualifyingContribution(t *testing.T) {
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	got := AssessPromotion(domain.PromotionResearchComplete, domain.PromotionGateInput{Contribution: domain.ContributionOther}, now)
	if got.State != domain.PromotionResearchComplete {
		t.Fatalf("got state %q, want RESEARCH_COMPLETE", got.State)
	}
	if len(got.Reasons) != 1 {
		t.Fatalf("expected exactly one blocking reason, got %v", got.Reasons)
	}
}

func TestAssessPromotionShouldLeaveRejectedForPublicationUnchanged(t *testing.T) {
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	got := AssessPromotion(domain.PromotionRejectedForPublication, fullyReadyPromotionInput(), now)
	if got.State != domain.PromotionRejectedForPublication {
		t.Fatalf("got state %q, want REJECTED_FOR_PUBLICATION", got.State)
	}
	if len(got.Reasons) != 0 {
		t.Fatalf("expected no reasons for a terminal rejected state, got %v", got.Reasons)
	}
}
