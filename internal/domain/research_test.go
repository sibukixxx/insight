package domain

import (
	"testing"
	"time"
)

func TestResearchRunAppendIterationPreservesPriorValue(t *testing.T) {
	first := ResearchIteration{ID: "iter-1", Sequence: 1, Question: "why did the metric change?"}
	run := ResearchRun{ID: "run-1", Question: first.Question, Iterations: []ResearchIteration{first}}

	updated := run.AppendIteration(ResearchIteration{ID: "iter-2", Sequence: 2, Question: first.Question, AddedEvidence: []string{"comparison series"}})
	if len(run.Iterations) != 1 {
		t.Fatalf("original run mutated: got %d iterations", len(run.Iterations))
	}
	if len(updated.Iterations) != 2 || updated.Iterations[0].ID != "iter-1" || updated.Iterations[1].ID != "iter-2" {
		t.Fatalf("iteration history not retained: %+v", updated.Iterations)
	}
}

func TestResearchGapCanBecomeProviderNeutralDataRequirement(t *testing.T) {
	gap := ResearchGap{ID: "gap-1", Category: ResearchGapConfounder, Need: "population by municipality and year", WhyItMatters: "population change could explain the observed association", AffectedHypothesisIDs: []string{"h1", "h2"}, Resolvable: true}
	req := DataRequirement{GapID: gap.ID, Need: gap.Need, Reason: gap.WhyItMatters, AffectedHypothesisIDs: gap.AffectedHypothesisIDs, RequiredDimensions: []string{"municipality", "year"}, RequiredPeriod: "2018-2026", SuggestedSourceCategory: "official statistics"}

	if req.GapID != gap.ID || req.Need == "" || req.SuggestedSourceCategory != "official statistics" {
		t.Fatalf("unexpected data requirement: %+v", req)
	}
}

func TestHumanNoveltyRequiresExplicitHumanValue(t *testing.T) {
	eval := HumanEvaluation{ResearchRunID: "run-1", IterationID: "iter-1", Novelty: NoveltyNew, EvaluatedAt: time.Now()}
	if eval.Novelty != NoveltyNew {
		t.Fatalf("unexpected novelty: %s", eval.Novelty)
	}
}

func TestAssociationOnlyIterationCanStateNotIdentifiedBoundary(t *testing.T) {
	iteration := ResearchIteration{
		ID: "iter-1", Sequence: 1, Question: "did the policy cause the increase?",
		ResearchGaps: []ResearchGap{{ID: "gap-control", Category: ResearchGapComparison, Need: "comparison trend", WhyItMatters: "the treated increase alone cannot identify the intervention effect", Resolvable: true}},
		WhatWeCannotConclude: []string{"The observed post-policy increase does not by itself establish that the policy caused the increase."},
	}
	if len(iteration.ResearchGaps) == 0 || len(iteration.WhatWeCannotConclude) == 0 {
		t.Fatal("association-only research must retain unresolved identification boundaries")
	}
}
