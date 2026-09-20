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

func TestResearchRunCurrentStageReflectsTheLatestIterationOnly(t *testing.T) {
	run := ResearchRun{Iterations: []ResearchIteration{
		{ID: "iter-1", Sequence: 1, Stage: StageValidation},
		{ID: "iter-2", Sequence: 2, Stage: StageExploratory},
	}}
	if got := run.CurrentStage(); got != StageExploratory {
		t.Fatalf("current stage must come from the latest iteration, got %s", got)
	}
	if got := (ResearchRun{}).CurrentStage(); got != "" {
		t.Fatalf("a run with no iterations has no current stage, got %q", got)
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
		ResearchGaps:         []ResearchGap{{ID: "gap-control", Category: ResearchGapComparison, Need: "comparison trend", WhyItMatters: "the treated increase alone cannot identify the intervention effect", Resolvable: true}},
		WhatWeCannotConclude: []string{"The observed post-policy increase does not by itself establish that the policy caused the increase."},
	}
	if len(iteration.ResearchGaps) == 0 || len(iteration.WhatWeCannotConclude) == 0 {
		t.Fatal("association-only research must retain unresolved identification boundaries")
	}
}

func TestDecisionReadinessValuesDoNotCollideWithExistingStatuses(t *testing.T) {
	existing := map[string]bool{}
	for _, v := range []string{string(CausalObservedAssociation), string(CausalHypothesis), string(CausallySupported), string(CausalNotIdentified),
		string(ValidationUntested), string(ValidationPlausible), string(ValidationPartiallySupported), string(ValidationSupported), string(ValidationInsufficientEvidence), string(ValidationContradicted),
		string(IdentificationIdentified), string(IdentificationNotIdentified), string(IdentificationUnknown)} {
		existing[v] = true
	}
	for _, state := range AllDecisionReadiness() {
		if !state.Valid() {
			t.Errorf("%s must be valid", state)
		}
		if existing[string(state)] {
			t.Errorf("readiness %q collides with an existing status value", state)
		}
	}
	if DecisionReadiness("READY").Valid() {
		t.Error("unknown readiness must be invalid")
	}
}

func TestApplyHumanStopKeepsHistoryAndUnresolvedGaps(t *testing.T) {
	at := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	iteration := ResearchIteration{
		ID: "iter-2", Sequence: 2,
		ResearchGaps: []ResearchGap{{ID: "gap-1", Need: "comparison trend"}, {ID: "gap-2", Need: "population", Resolved: true}},
		Readiness:    ReadinessAssessment{State: ReadinessValidationRequired},
	}

	stopped, err := iteration.ApplyHumanOverride(HumanOverride{StopReason: StopHumanChoice, Note: "budget exhausted", RecordedAt: at})
	if err != nil {
		t.Fatal(err)
	}
	if iteration.Stop != nil || len(iteration.HumanOverrides) != 0 {
		t.Fatal("original iteration must not be mutated")
	}
	if stopped.Stop == nil || stopped.Stop.Reason != StopHumanChoice || stopped.Stop.Source != StopSourceHuman {
		t.Fatalf("human stop not recorded: %+v", stopped.Stop)
	}
	if len(stopped.Stop.UnresolvedGapIDs) != 1 || stopped.Stop.UnresolvedGapIDs[0] != "gap-1" {
		t.Fatalf("unresolved gaps must survive a stop: %+v", stopped.Stop.UnresolvedGapIDs)
	}
	if len(stopped.ResearchGaps) != 2 {
		t.Fatal("stop must not delete research gaps")
	}
	if len(stopped.HumanOverrides) != 1 || stopped.HumanOverrides[0].Note != "budget exhausted" {
		t.Fatalf("override history missing: %+v", stopped.HumanOverrides)
	}
	if stopped.Stop.ReadinessAtStop != ReadinessValidationRequired {
		t.Fatalf("stop must record readiness at the time of stopping: %+v", stopped.Stop)
	}
}

func TestHumanOverrideRejectsSystemOnlyStopReasons(t *testing.T) {
	iteration := ResearchIteration{ID: "iter-1"}
	if _, err := iteration.ApplyHumanOverride(HumanOverride{StopReason: StopHypothesesDistinguished}); err == nil {
		t.Fatal("humans may only stop with HUMAN_STOPPED or EXTERNAL_BUDGET_BOUNDARY")
	}
	if _, err := iteration.ApplyHumanOverride(HumanOverride{Readiness: DecisionReadiness("READY")}); err == nil {
		t.Fatal("invalid readiness override must be rejected")
	}
	if _, err := iteration.ApplyHumanOverride(HumanOverride{}); err == nil {
		t.Fatal("an override must change readiness or stop the loop")
	}
}

func TestEffectiveReadinessPrefersLatestHumanOverride(t *testing.T) {
	iteration := ResearchIteration{ID: "iter-1", Readiness: ReadinessAssessment{State: ReadinessEvidenceConverging}}
	if iteration.EffectiveReadiness() != ReadinessEvidenceConverging {
		t.Fatal("computed readiness must apply when no override exists")
	}
	first, err := iteration.ApplyHumanOverride(HumanOverride{Readiness: ReadinessValidationRequired, Note: "counter evidence not yet reviewed"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := first.ApplyHumanOverride(HumanOverride{Readiness: ReadinessInconclusive, Note: "reviewed; still conflicting"})
	if err != nil {
		t.Fatal(err)
	}
	if second.EffectiveReadiness() != ReadinessInconclusive || second.Readiness.State != ReadinessEvidenceConverging {
		t.Fatalf("latest override must win without erasing the computed assessment: %+v", second)
	}
	if len(second.HumanOverrides) != 2 {
		t.Fatalf("override history must accumulate: %+v", second.HumanOverrides)
	}
}

func TestResearchRunStoppedReflectsLatestIteration(t *testing.T) {
	run := ResearchRun{Iterations: []ResearchIteration{{ID: "iter-1", Stop: &StopDecision{Reason: StopHumanChoice}}, {ID: "iter-2"}}}
	if run.Stopped() {
		t.Fatal("an earlier stop followed by a new iteration means research continues")
	}
	if latest, ok := run.LatestIteration(); !ok || latest.ID != "iter-2" {
		t.Fatalf("latest iteration lookup failed: %+v %v", latest, ok)
	}
	run.Iterations[1].Stop = &StopDecision{Reason: StopNoFeasibleDataSource, Source: StopSourceSystem}
	if !run.Stopped() {
		t.Fatal("run with a stopped latest iteration must report stopped")
	}
	if _, ok := (ResearchRun{}).LatestIteration(); ok {
		t.Fatal("empty run has no latest iteration")
	}
}
