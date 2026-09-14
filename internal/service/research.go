package service

import (
	"fmt"
	"strings"

	"insight-lab/internal/domain"
)

// ResearchGapFromMissingEvidence promotes an already-recorded evidence gap into
// a structured research object. It does not invent evidence or claim that the
// gap has been resolved.
func ResearchGapFromMissingEvidence(id string, category domain.ResearchGapCategory, need, why string, hypothesisIDs []string) domain.ResearchGap {
	return domain.ResearchGap{
		ID: id, Category: category, Need: strings.TrimSpace(need), WhyItMatters: strings.TrimSpace(why),
		AffectedHypothesisIDs: append([]string(nil), hypothesisIDs...), Resolvable: true,
	}
}

// PlanDataRequirement converts a gap into a provider-neutral collection plan.
// SourceCategory is descriptive only; no external provider is selected or
// invoked here.
func PlanDataRequirement(gap domain.ResearchGap, dimensions []string, period, sourceCategory string) (domain.DataRequirement, error) {
	if strings.TrimSpace(gap.ID) == "" || strings.TrimSpace(gap.Need) == "" {
		return domain.DataRequirement{}, fmt.Errorf("research gap id and need are required")
	}
	return domain.DataRequirement{
		GapID: gap.ID,
		Need: gap.Need,
		Reason: gap.WhyItMatters,
		AffectedHypothesisIDs: append([]string(nil), gap.AffectedHypothesisIDs...),
		RequiredDimensions: append([]string(nil), dimensions...),
		RequiredPeriod: strings.TrimSpace(period),
		SuggestedSourceCategory: strings.TrimSpace(sourceCategory),
	}, nil
}
