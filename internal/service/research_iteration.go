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
			iteration.InsightIDs = append(iteration.InsightIDs, insight.ID)
			iteration.HypothesisStates = append(iteration.HypothesisStates, domain.HypothesisState{
				HypothesisID: insight.ID, ComparisonKey: hypothesisComparisonKey(insight), ValidationStatus: insight.ValidationStatus,
				IdentificationStatus: insight.IdentificationStatus,
			})
		}
		if strings.TrimSpace(insight.SurprisingFact) != "" {
			iteration.SurpriseIDs = append(iteration.SurpriseIDs, insight.ID)
		}
		if insight.HypothesisSetID != "" && !seenSets[insight.HypothesisSetID] {
			seenSets[insight.HypothesisSetID] = true
			iteration.HypothesisSetIDs = append(iteration.HypothesisSetIDs, insight.HypothesisSetID)
		}
		for _, missing := range insight.MissingEvidence {
			missing = strings.TrimSpace(missing)
			if missing == "" {
				continue
			}
			gap := ResearchGapFromMissingEvidence(
				newID("gap"), ClassifyResearchGap(missing), missing,
				"This evidence is missing from the current hypothesis evaluation.", []string{insight.ID},
			)
			iteration.ResearchGaps = append(iteration.ResearchGaps, gap)
			req, err := PlanDataRequirementForGap(gap)
			if err == nil {
				iteration.DataRequirements = append(iteration.DataRequirements, req)
			}
		}
		if insight.IdentificationStatus == domain.IdentificationNotIdentified {
			iteration.WhatWeCannotConclude = append(iteration.WhatWeCannotConclude,
				"Causality is not identified for hypothesis "+insight.ID+" from the current evidence.")
		}
	}

	sort.Strings(iteration.HypothesisSetIDs)
	return iteration
}

// CompareHypothesisStates describes evidence/validation history. It never
// changes causal or identification status and does not express probability.
func CompareHypothesisStates(previous []domain.HypothesisState, current []domain.HypothesisState) []domain.HypothesisChange {
	old := make(map[string]domain.HypothesisState, len(previous))
	for _, state := range previous {
		old[stateKey(state)] = state
	}
	changes := make([]domain.HypothesisChange, 0, len(current))
	for _, state := range current {
		prior, exists := old[stateKey(state)]
		evolution := domain.HypothesisCreated
		reason := "hypothesis first appears in this iteration"
		if exists {
			evolution, reason = validationEvolution(prior.ValidationStatus, state.ValidationStatus)
		}
		changes = append(changes, domain.HypothesisChange{HypothesisID: state.HypothesisID, Evolution: evolution, Reason: reason})
	}
	return changes
}

func hypothesisComparisonKey(insight *domain.Insight) string {
	return strings.ToLower(strings.Join(strings.Fields(insight.Title), " "))
}

func stateKey(state domain.HypothesisState) string {
	if state.ComparisonKey != "" {
		return state.ComparisonKey
	}
	return state.HypothesisID
}

func validationEvolution(before, after domain.ValidationStatus) (domain.HypothesisEvolution, string) {
	if before == after {
		return domain.HypothesisUnchanged, "validation status is unchanged"
	}
	if after == domain.ValidationContradicted {
		return domain.HypothesisContradicted, "new evaluation is contradicted by evidence"
	}
	rank := map[domain.ValidationStatus]int{
		domain.ValidationUntested: 0, domain.ValidationInsufficientEvidence: 1,
		domain.ValidationPlausible: 2, domain.ValidationPartiallySupported: 3, domain.ValidationSupported: 4,
	}
	if rank[after] > rank[before] {
		return domain.HypothesisStrengthened, "validation status gained evidence support"
	}
	return domain.HypothesisWeakened, "validation status lost evidence support"
}
