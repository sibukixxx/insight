package service

import (
	"sort"
	"strings"
	"time"

	"insight-lab/internal/domain"
)

// BuildResearchIteration projects persisted Insight results into the generic
// Research Loop. It only promotes gaps already present in Insight output; it
// does not invent evidence or upgrade causal identification.
func BuildResearchIteration(sequence int, question string, inputReferences []string, insights []*domain.Insight, now time.Time) domain.ResearchIteration {
	iteration := domain.ResearchIteration{
		ID: newID("rit"), Sequence: sequence, Question: strings.TrimSpace(question),
		InputReferences: append([]string(nil), inputReferences...), CreatedAt: now,
	}

	seenSets := map[string]bool{}
	for _, insight := range insights {
		if insight == nil {
			continue
		}
		if insight.ID != "" {
			iteration.ObservationIDs = append(iteration.ObservationIDs, insight.ID)
		}
		if strings.TrimSpace(insight.SurprisingFact) != "" {
			iteration.SurpriseIDs = append(iteration.SurpriseIDs, insight.ID)
		}
		if insight.HypothesisSetID != "" && !seenSets[insight.HypothesisSetID] {
			seenSets[insight.HypothesisSetID] = true
			iteration.HypothesisSetIDs = append(iteration.HypothesisSetIDs, insight.HypothesisSetID)
		}
		for n, missing := range insight.MissingEvidence {
			missing = strings.TrimSpace(missing)
			if missing == "" {
				continue
			}
			gap := ResearchGapFromMissingEvidence(
				newID("gap"), domain.ResearchGapOther, missing,
				"This evidence is missing from the current hypothesis evaluation.", []string{insight.ID},
			)
			iteration.ResearchGaps = append(iteration.ResearchGaps, gap)
			req, err := PlanDataRequirement(gap, nil, "", "unspecified")
			if err == nil {
				iteration.DataRequirements = append(iteration.DataRequirements, req)
			}
			_ = n
		}
		if insight.IdentificationStatus == domain.IdentificationNotIdentified {
			iteration.WhatWeCannotConclude = append(iteration.WhatWeCannotConclude,
				"Causality is not identified for hypothesis "+insight.ID+" from the current evidence.")
		}
	}

	sort.Strings(iteration.HypothesisSetIDs)
	return iteration
}
