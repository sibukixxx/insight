package service

import (
	"fmt"
	"strings"

	"insight-lab/internal/domain"
)

// ClassifyResearchGap uses a deliberately small deterministic vocabulary.
// Ambiguous prose remains OTHER rather than being confidently invented by an LLM.
func ClassifyResearchGap(need string) domain.ResearchGapCategory {
	v := strings.ToLower(need)
	cases := []struct {
		category domain.ResearchGapCategory
		words    []string
	}{
		{domain.ResearchGapComparison, []string{"comparison", "control group", "control region", "対照"}},
		{domain.ResearchGapPrePeriod, []string{"pre-period", "pre policy", "before treatment", "baseline trend", "導入前"}},
		{domain.ResearchGapTiming, []string{"timing", "implementation date", "policy date", "時期", "タイミング"}},
		{domain.ResearchGapConfounder, []string{"confound", "population", "economic condition", "交絡", "人口", "景気"}},
		{domain.ResearchGapMeasurement, []string{"measurement", "definition", "registration", "schema change", "測定", "定義", "登記"}},
		{domain.ResearchGapSourceQuality, []string{"source quality", "missing source", "provenance", "出典", "品質"}},
		{domain.ResearchGapExternalContext, []string{"external context", "macro", "national trend", "外部", "全国"}},
	}
	for _, candidate := range cases {
		for _, word := range candidate.words {
			if strings.Contains(v, word) {
				return candidate.category
			}
		}
	}
	return domain.ResearchGapOther
}

// ResearchGapFromMissingEvidence promotes an already-recorded evidence gap into
// a structured research object. It does not invent evidence or claim that the
// gap has been resolved.
func ResearchGapFromMissingEvidence(id string, category domain.ResearchGapCategory, need, why string, hypothesisIDs []string) domain.ResearchGap {
	return domain.ResearchGap{
		ID: id, Category: category, Need: strings.TrimSpace(need), WhyItMatters: strings.TrimSpace(why),
		AffectedHypothesisIDs: append([]string(nil), hypothesisIDs...), Resolvable: true,
	}
}

func PlanDataRequirementForGap(gap domain.ResearchGap) (domain.DataRequirement, error) {
	dimensions, period, source := []string(nil), "", "unspecified"
	switch gap.Category {
	case domain.ResearchGapComparison:
		dimensions, period, source = []string{"group", "time"}, "matched analysis period", "comparison dataset"
	case domain.ResearchGapPrePeriod:
		dimensions, period, source = []string{"unit", "time"}, "pre-exposure period", "historical records"
	case domain.ResearchGapTiming:
		dimensions, source = []string{"event", "date"}, "documented event chronology"
	case domain.ResearchGapConfounder:
		dimensions, period, source = []string{"candidate confounder", "unit", "time"}, "analysis period", "official or audited statistics"
	case domain.ResearchGapMeasurement:
		dimensions, source = []string{"measure", "definition", "version"}, "measurement documentation"
	case domain.ResearchGapExternalContext:
		dimensions, period, source = []string{"context", "time"}, "analysis period", "external contextual data"
	case domain.ResearchGapSourceQuality:
		dimensions, source = []string{"source", "version", "provenance"}, "source documentation"
	}
	return PlanDataRequirement(gap, dimensions, period, source)
}

// PlanDataRequirement converts a gap into a provider-neutral collection plan.
// SourceCategory is descriptive only; no external provider is selected or
// invoked here.
func PlanDataRequirement(gap domain.ResearchGap, dimensions []string, period, sourceCategory string) (domain.DataRequirement, error) {
	if strings.TrimSpace(gap.ID) == "" || strings.TrimSpace(gap.Need) == "" {
		return domain.DataRequirement{}, fmt.Errorf("research gap id and need are required")
	}
	return domain.DataRequirement{
		GapID:                   gap.ID,
		Need:                    gap.Need,
		Reason:                  gap.WhyItMatters,
		AffectedHypothesisIDs:   append([]string(nil), gap.AffectedHypothesisIDs...),
		RequiredDimensions:      append([]string(nil), dimensions...),
		RequiredPeriod:          strings.TrimSpace(period),
		SuggestedSourceCategory: strings.TrimSpace(sourceCategory),
	}, nil
}
