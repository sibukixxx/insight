package service

import (
	"time"

	"insight-lab/internal/domain"
)

// promotionForwardOrder is the linear promotion path AssessPromotion walks,
// mirroring the one-step-forward rule enforced by
// domain.PromotionState.Transition.
var promotionForwardOrder = []domain.PromotionState{
	domain.PromotionDraft,
	domain.PromotionResearchComplete,
	domain.PromotionHumanReviewRequired,
	domain.PromotionPublicationReady,
}

// AssessPromotion reports the furthest PromotionState reachable from current
// given the gate evidence in, and why it cannot go further. It never applies
// the transition itself; the caller decides whether and when to persist a
// new state, keeping the human review step out of the system's hands.
func AssessPromotion(current domain.PromotionState, in domain.PromotionGateInput, now time.Time) domain.PromotionAssessment {
	assessment := domain.PromotionAssessment{State: current, AssessedAt: now}
	if current == domain.PromotionRejectedForPublication || current == domain.PromotionPublished {
		return assessment
	}
	start := -1
	for i, s := range promotionForwardOrder {
		if s == current {
			start = i
			break
		}
	}
	if start < 0 {
		assessment.Reasons = []string{"unknown promotion state"}
		return assessment
	}
	for i := start; i+1 < len(promotionForwardOrder); i++ {
		next := promotionForwardOrder[i+1]
		if err := assessment.State.Transition(next, in); err != nil {
			assessment.Reasons = []string{err.Error()}
			return assessment
		}
		assessment.State = next
	}
	return assessment
}
