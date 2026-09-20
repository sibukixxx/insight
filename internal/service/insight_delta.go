package service

import (
	"fmt"
	"sort"

	"insight-lab/internal/domain"
)

func CompareResearchIterations(previous, current domain.ResearchIteration) domain.InsightDelta {
	d := domain.InsightDelta{
		FromIterationID: previous.ID,
		ToIterationID: current.ID,
		Input: compareInputSnapshots(previous.InputSnapshot, current.InputSnapshot),
	}
	d.Result.InsightIDsAdded, d.Result.InsightIDsRemoved = setDiff(previous.InsightIDs, current.InsightIDs)
	prevGaps := map[string]domain.ResearchGap{}
	for _, g := range previous.ResearchGaps {
		prevGaps[g.ID] = g
	}
	for _, g := range current.ResearchGaps {
		if _, ok := prevGaps[g.ID]; !ok {
			d.Result.ResearchGapIDsCreated = append(d.Result.ResearchGapIDsCreated, g.ID)
		}
		if old, ok := prevGaps[g.ID]; ok && !old.Resolved && g.Resolved {
			d.Result.ResearchGapIDsResolved = append(d.Result.ResearchGapIDsResolved, g.ID)
		}
	}
	prevReq := map[string]bool{}
	for _, req := range previous.DataRequirements {
		prevReq[req.GapID] = true
	}
	for _, req := range current.DataRequirements {
		if !prevReq[req.GapID] {
			d.Result.DataRequirementGapIDsAdded = append(d.Result.DataRequirementGapIDsAdded, req.GapID)
		}
	}
	d.Result.HypothesisChanges = append([]domain.HypothesisChange(nil), current.HypothesisChanges...)
	d.Result.ConclusionsAdded, d.Result.ConclusionsRemoved = setDiff(previous.WhatWeCannotConclude, current.WhatWeCannotConclude)
	if hasInputDelta(d.Input) {
		d.Explanation = append(d.Explanation, "research inputs changed between iterations; resulting output changes are tracked but are not treated as causal effects of the added inputs")
	}
	if len(d.Result.HypothesisChanges) > 0 {
		d.Explanation = append(d.Explanation, fmt.Sprintf("%d hypothesis state changes were recorded", len(d.Result.HypothesisChanges)))
	}
	sort.Strings(d.Result.ResearchGapIDsCreated)
	sort.Strings(d.Result.ResearchGapIDsResolved)
	sort.Strings(d.Result.DataRequirementGapIDsAdded)
	return d
}

func compareInputSnapshots(a, b domain.InputSetSnapshot) domain.InputDelta {
	var d domain.InputDelta
	d.ArtifactReferences.Added, d.ArtifactReferences.Removed = setDiff(a.ArtifactReferences, b.ArtifactReferences)
	d.EvidenceReferences.Added, d.EvidenceReferences.Removed = setDiff(a.EvidenceReferences, b.EvidenceReferences)
	d.Variables.Added, d.Variables.Removed = setDiff(a.Variables, b.Variables)
	d.Dimensions.Added, d.Dimensions.Removed = setDiff(a.Dimensions, b.Dimensions)
	d.Filters.Added, d.Filters.Removed = setDiff(a.Filters, b.Filters)
	d.Contexts.Added, d.Contexts.Removed = setDiff(a.Contexts, b.Contexts)
	d.Transforms.Added, d.Transforms.Removed = setDiff(a.Transforms, b.Transforms)
	return d
}

func setDiff(before, after []string) (added, removed []string) {
	b := map[string]bool{}
	a := map[string]bool{}
	for _, v := range before {
		if v != "" {
			b[v] = true
		}
	}
	for _, v := range after {
		if v != "" {
			a[v] = true
		}
	}
	for v := range a {
		if !b[v] {
			added = append(added, v)
		}
	}
	for v := range b {
		if !a[v] {
			removed = append(removed, v)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

func hasInputDelta(d domain.InputDelta) bool {
	all := []domain.SetDelta{d.ArtifactReferences, d.EvidenceReferences, d.Variables, d.Dimensions, d.Filters, d.Contexts, d.Transforms}
	for _, x := range all {
		if len(x.Added) > 0 || len(x.Removed) > 0 {
			return true
		}
	}
	return false
}
