package service

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
)

func TestBuildResearchIterationPromotesMissingEvidenceWithoutCausalUpgrade(t *testing.T) {
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	insights := []*domain.Insight{{
		ID: "hyp-1", SurprisingFact: "treated unit rose", HypothesisSetID: "set-1",
		MissingEvidence: []string{"comparison trend before treatment"},
		CausalStatus:    domain.CausalHypothesis, IdentificationStatus: domain.IdentificationNotIdentified,
	}}

	got := BuildResearchIteration(1, "Did treatment cause the increase?", []string{"synthetic.csv"}, insights, now)
	if len(got.ResearchGaps) != 1 || len(got.DataRequirements) != 1 {
		t.Fatalf("expected structured gap and requirement: %+v", got)
	}
	if got.ResearchGaps[0].Resolved {
		t.Fatal("new missing-evidence gap must remain unresolved")
	}
	if got.DataRequirements[0].Need != "comparison trend before treatment" {
		t.Fatalf("missing evidence must be preserved: %+v", got.DataRequirements[0])
	}
	if len(got.WhatWeCannotConclude) != 1 || !strings.Contains(got.WhatWeCannotConclude[0], "not identified") {
		t.Fatalf("unresolved identification must be explicit: %+v", got.WhatWeCannotConclude)
	}
	if insights[0].CausalStatus != domain.CausalHypothesis || insights[0].IdentificationStatus != domain.IdentificationNotIdentified {
		t.Fatal("research projection must not upgrade causal state")
	}
}

func TestGoldenPolicyDogfoodRemainsNotIdentified(t *testing.T) {
	var fixture struct {
		Question               string   `json:"question"`
		Hypotheses             []string `json:"hypotheses"`
		MissingEvidence        []string `json:"missingEvidence"`
		ExpectedIdentification string   `json:"expectedIdentification"`
	}
	raw, err := os.ReadFile("testdata/research_loop_policy.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	insights := make([]*domain.Insight, 0, len(fixture.Hypotheses))
	for i, title := range fixture.Hypotheses {
		insights = append(insights, &domain.Insight{ID: fmt.Sprintf("h%d", i+1), Title: title, SurprisingFact: "treated and comparison outcomes both increased", HypothesisSetID: "set1", MissingEvidence: fixture.MissingEvidence, CausalStatus: domain.CausalHypothesis, ValidationStatus: domain.ValidationInsufficientEvidence, IdentificationStatus: domain.IdentificationNotIdentified})
	}
	got := BuildResearchIteration(1, fixture.Question, []string{"synthetic policy fixture"}, insights, time.Now())
	if len(got.InsightIDs) != 3 || len(got.HypothesisSetIDs) != 1 {
		t.Fatalf("competing hypotheses were not retained: %+v", got)
	}
	if len(got.WhatWeCannotConclude) != 3 {
		t.Fatalf("identification boundary missing: %+v", got.WhatWeCannotConclude)
	}
	for _, state := range got.HypothesisStates {
		if string(state.IdentificationStatus) != fixture.ExpectedIdentification {
			t.Fatalf("causality was promoted: %+v", state)
		}
	}
}

func TestResearchGapClassificationIsDeterministicAndUnknownStaysOther(t *testing.T) {
	for input, want := range map[string]domain.ResearchGapCategory{"control regions": domain.ResearchGapComparison, "pre-period trend": domain.ResearchGapPrePeriod, "exact policy timing": domain.ResearchGapTiming, "population by year": domain.ResearchGapConfounder, "unfamiliar unresolved question": domain.ResearchGapOther} {
		if got := ClassifyResearchGap(input); got != want {
			t.Errorf("ClassifyResearchGap(%q)=%s want %s", input, got, want)
		}
	}
}

func TestCompareHypothesisStatesAuditsEvidenceChangeWithoutCausalPromotion(t *testing.T) {
	previous := []domain.HypothesisState{{HypothesisID: "old-h1", ComparisonKey: "treatment effect", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified}}
	current := []domain.HypothesisState{{HypothesisID: "new-h1", ComparisonKey: "treatment effect", ValidationStatus: domain.ValidationPartiallySupported, IdentificationStatus: domain.IdentificationNotIdentified}}
	changes := CompareHypothesisStates(previous, current)
	if len(changes) != 1 || changes[0].Evolution != domain.HypothesisStrengthened {
		t.Fatalf("unexpected evolution: %+v", changes)
	}
	if current[0].IdentificationStatus != domain.IdentificationNotIdentified {
		t.Fatal("history comparison promoted identification")
	}
}

func TestBuildResearchIterationDeduplicatesHypothesisSets(t *testing.T) {
	got := BuildResearchIteration(2, "q", nil, []*domain.Insight{
		{ID: "h1", HypothesisSetID: "set-a"},
		{ID: "h2", HypothesisSetID: "set-a"},
		{ID: "h3", HypothesisSetID: "set-b"},
	}, time.Now())
	if len(got.HypothesisSetIDs) != 2 || got.HypothesisSetIDs[0] != "set-a" || got.HypothesisSetIDs[1] != "set-b" {
		t.Fatalf("unexpected sets: %+v", got.HypothesisSetIDs)
	}
}

func TestBuildResearchIterationRanksRequirementsByDiscriminatingPower(t *testing.T) {
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	insights := []*domain.Insight{
		{ID: "h1", Title: "policy effect", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified,
			MissingEvidence: []string{"comparison trend before treatment", "exact policy timing", "an unfamiliar note"}},
		{ID: "h2", Title: "macro trend", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified,
			MissingEvidence: []string{"comparison trend before treatment"}},
	}

	got := BuildResearchIteration(1, "did the policy cause it?", nil, insights, now)

	if len(got.DataRequirements) != 3 {
		t.Fatalf("identical needs across hypotheses must merge into one requirement: %+v", got.DataRequirements)
	}
	first := got.DataRequirements[0]
	if first.Need != "comparison trend before treatment" || first.Priority.Rank != 1 {
		t.Fatalf("comparison evidence must rank first: %+v", first)
	}
	if first.Priority.DiscriminatingPower != domain.DiscriminatingHigh || !first.Priority.CanDistinguishHypotheses || !first.Priority.CanFalsify {
		t.Fatalf("shared identification-critical gap must be marked as discriminating: %+v", first.Priority)
	}
	if len(first.AffectedHypothesisIDs) != 2 {
		t.Fatalf("merged requirement must keep both hypotheses: %+v", first.AffectedHypothesisIDs)
	}
	if first.Priority.Urgency != domain.UrgencyHigh {
		t.Fatalf("not-identified hypotheses make comparison evidence urgent: %+v", first.Priority)
	}
	if got.DataRequirements[1].Need != "exact policy timing" || got.DataRequirements[1].Priority.DiscriminatingPower != domain.DiscriminatingModerate {
		t.Fatalf("timing should rank second with moderate power: %+v", got.DataRequirements[1])
	}
	last := got.DataRequirements[2]
	if last.Priority.Rank != 3 || last.Priority.DiscriminatingPower != domain.DiscriminatingLow || last.Priority.CanFalsify {
		t.Fatalf("unclassified evidence must not be treated as decisive: %+v", last.Priority)
	}
	for _, req := range got.DataRequirements {
		if len(req.Priority.Rationale) == 0 || req.Priority.AcquisitionDifficulty == "" {
			t.Fatalf("every requirement needs a rationale and qualitative difficulty: %+v", req)
		}
	}
	for _, gap := range got.ResearchGaps {
		if gap.FirstSeenIterationID != got.ID {
			t.Fatalf("gap must be linked to the iteration where it first appeared: %+v", gap)
		}
	}
}

func TestPrioritizeResearchIterationRecordsMeasurementDependency(t *testing.T) {
	iteration := domain.ResearchIteration{ID: "it-1", HypothesisStates: []domain.HypothesisState{{HypothesisID: "h1", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified}}}
	measurement := ResearchGapFromMissingEvidence("gap-m", domain.ResearchGapMeasurement, "registration definition change", "why", []string{"h1"})
	comparison := ResearchGapFromMissingEvidence("gap-c", domain.ResearchGapComparison, "comparison region trend", "why", []string{"h1"})
	iteration.ResearchGaps = []domain.ResearchGap{comparison, measurement}
	for _, gap := range iteration.ResearchGaps {
		req, _ := PlanDataRequirementForGap(gap)
		iteration.DataRequirements = append(iteration.DataRequirements, req)
	}

	got := PrioritizeResearchIteration(iteration)

	byGap := map[string]domain.DataRequirement{}
	for _, req := range got.DataRequirements {
		byGap[req.GapID] = req
	}
	if deps := byGap["gap-c"].DependsOnGapIDs; len(deps) != 1 || deps[0] != "gap-m" {
		t.Fatalf("comparison evidence must depend on unresolved measurement definition: %+v", byGap["gap-c"])
	}
	if len(byGap["gap-m"].DependsOnGapIDs) != 0 {
		t.Fatalf("measurement gap has no prerequisite: %+v", byGap["gap-m"])
	}
	if got.ResearchGaps[0].ID != "gap-c" || len(got.ResearchGaps[0].DependsOnGapIDs) != 1 {
		t.Fatalf("dependency must also be visible on the gap itself: %+v", got.ResearchGaps)
	}
	if len(iteration.DataRequirements[0].DependsOnGapIDs) != 0 {
		t.Fatal("input iteration must not be mutated")
	}
}

func TestAssessDecisionReadinessNeverReadyAfterSinglePass(t *testing.T) {
	now := time.Now()
	run := domain.ResearchRun{ID: "run", Iterations: []domain.ResearchIteration{{
		ID: "it-1", Sequence: 1,
		HypothesisStates: []domain.HypothesisState{{HypothesisID: "h1", ValidationStatus: domain.ValidationSupported, IdentificationStatus: domain.IdentificationIdentified}},
	}}}

	got := AssessDecisionReadiness(run, now)

	if got.State != domain.ReadinessExploratoryOnly {
		t.Fatalf("one-pass summary must not become decision-ready, got %s (%v)", got.State, got.Reasons)
	}
	if got.IterationCount != 1 || len(got.Reasons) == 0 {
		t.Fatalf("assessment must explain itself: %+v", got)
	}
}

func TestAssessDecisionReadinessIsInsufficientWhenNothingIsSupported(t *testing.T) {
	run := domain.ResearchRun{Iterations: []domain.ResearchIteration{{ID: "it-1", HypothesisStates: []domain.HypothesisState{
		{HypothesisID: "h1", ValidationStatus: domain.ValidationInsufficientEvidence}, {HypothesisID: "h2", ValidationStatus: domain.ValidationUntested},
	}}}}
	if got := AssessDecisionReadiness(run, time.Now()); got.State != domain.ReadinessEvidenceInsufficient {
		t.Fatalf("got %s", got.State)
	}
	if got := AssessDecisionReadiness(domain.ResearchRun{Iterations: []domain.ResearchIteration{{ID: "it-1"}}}, time.Now()); got.State != domain.ReadinessEvidenceInsufficient {
		t.Fatalf("no hypotheses must be insufficient, got %s", got.State)
	}
}

func TestAssessDecisionReadinessBlockedWhenNoGapIsResolvable(t *testing.T) {
	run := domain.ResearchRun{Iterations: []domain.ResearchIteration{{
		ID:               "it-1",
		HypothesisStates: []domain.HypothesisState{{HypothesisID: "h1", ValidationStatus: domain.ValidationPlausible}},
		ResearchGaps:     []domain.ResearchGap{{ID: "gap-1", Need: "destroyed archive", Resolvable: false}},
	}}}
	got := AssessDecisionReadiness(run, time.Now())
	if got.State != domain.ReadinessBlocked || len(got.UnresolvedGapIDs) != 1 {
		t.Fatalf("got %+v", got)
	}
	stop := DecideStop(run, got, time.Now())
	if stop == nil || stop.Reason != domain.StopNoFeasibleDataSource || stop.Source != domain.StopSourceSystem {
		t.Fatalf("blocked research must stop with an explicit reason: %+v", stop)
	}
	if len(stop.UnresolvedGapIDs) != 1 || stop.UnresolvedGapIDs[0] != "gap-1" {
		t.Fatalf("stop must keep unresolved gaps: %+v", stop)
	}
}

func twoIterationRun(second domain.ResearchIteration) domain.ResearchRun {
	first := domain.ResearchIteration{ID: "it-1", Sequence: 1, HypothesisStates: []domain.HypothesisState{
		{HypothesisID: "h1", ComparisonKey: "policy effect", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified},
		{HypothesisID: "h2", ComparisonKey: "macro trend", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified},
	}}
	second.ID, second.Sequence = "it-2", 2
	second.HypothesisChanges = CompareHypothesisStates(first.HypothesisStates, second.HypothesisStates)
	return domain.ResearchRun{ID: "run", Question: "did the policy cause it?", Iterations: []domain.ResearchIteration{first, second}}
}

func TestAssessDecisionReadinessConvergesWhileHighDiscriminatingGapRemains(t *testing.T) {
	gap := ResearchGapFromMissingEvidence("gap-c", domain.ResearchGapComparison, "comparison trend", "why", []string{"h1", "h2"})
	req, _ := PlanDataRequirementForGap(gap)
	second := PrioritizeResearchIteration(domain.ResearchIteration{
		AddedEvidence: []string{"pre-period series"},
		HypothesisStates: []domain.HypothesisState{
			{HypothesisID: "h1b", ComparisonKey: "policy effect", ValidationStatus: domain.ValidationSupported, IdentificationStatus: domain.IdentificationNotIdentified},
			{HypothesisID: "h2b", ComparisonKey: "macro trend", ValidationStatus: domain.ValidationContradicted, IdentificationStatus: domain.IdentificationNotIdentified},
		},
		ResearchGaps: []domain.ResearchGap{gap}, DataRequirements: []domain.DataRequirement{req},
	})
	run := twoIterationRun(second)

	got := AssessDecisionReadiness(run, time.Now())
	if got.State != domain.ReadinessEvidenceConverging {
		t.Fatalf("got %s (%v)", got.State, got.Reasons)
	}
	if DecideStop(run, got, time.Now()) != nil {
		t.Fatal("converging research must continue while a discriminating gap is open")
	}
}

func TestAssessDecisionReadinessReadyWithLimitationsWhenOneHypothesisSurvives(t *testing.T) {
	second := domain.ResearchIteration{
		AddedEvidence: []string{"comparison series"},
		HypothesisStates: []domain.HypothesisState{
			{HypothesisID: "h1b", ComparisonKey: "policy effect", ValidationStatus: domain.ValidationSupported, IdentificationStatus: domain.IdentificationNotIdentified},
			{HypothesisID: "h2b", ComparisonKey: "macro trend", ValidationStatus: domain.ValidationContradicted, IdentificationStatus: domain.IdentificationNotIdentified},
		},
		WhatWeCannotConclude: []string{"Causality is not identified for hypothesis h1b from the current evidence."},
	}
	run := twoIterationRun(second)

	got := AssessDecisionReadiness(run, time.Now())
	if got.State != domain.ReadinessDecisionReadyWithLimitation {
		t.Fatalf("got %s (%v)", got.State, got.Reasons)
	}
	if !strings.Contains(strings.Join(got.Reasons, " "), "NOT_IDENTIFIED") {
		t.Fatalf("limitations must name unresolved identification: %v", got.Reasons)
	}
	stop := DecideStop(run, got, time.Now())
	if stop == nil || stop.Reason != domain.StopHypothesesDistinguished || stop.ReadinessAtStop != got.State {
		t.Fatalf("unexpected stop: %+v", stop)
	}
}

func TestAssessDecisionReadinessInconclusiveWhenAddedEvidenceDoesNotMoveHypotheses(t *testing.T) {
	second := domain.ResearchIteration{
		AddedEvidence: []string{"comparison series"},
		HypothesisStates: []domain.HypothesisState{
			{HypothesisID: "h1b", ComparisonKey: "policy effect", ValidationStatus: domain.ValidationPartiallySupported, IdentificationStatus: domain.IdentificationNotIdentified},
			{HypothesisID: "h2b", ComparisonKey: "macro trend", ValidationStatus: domain.ValidationPartiallySupported, IdentificationStatus: domain.IdentificationNotIdentified},
		},
	}
	run := twoIterationRun(second)
	run.Iterations[0].HypothesisStates[0].ValidationStatus = domain.ValidationPartiallySupported
	run.Iterations[0].HypothesisStates[1].ValidationStatus = domain.ValidationPartiallySupported
	run.Iterations[1].HypothesisChanges = CompareHypothesisStates(run.Iterations[0].HypothesisStates, run.Iterations[1].HypothesisStates)

	got := AssessDecisionReadiness(run, time.Now())
	if got.State != domain.ReadinessInconclusive {
		t.Fatalf("got %s (%v)", got.State, got.Reasons)
	}
	stop := DecideStop(run, got, time.Now())
	if stop == nil || stop.Reason != domain.StopIdentificationUnresolved {
		t.Fatalf("competing not-identified hypotheses with nothing left to collect must stop on identification: %+v", stop)
	}
}

func TestCarryForwardResearchGapsResolvesOnlyWithAddedEvidence(t *testing.T) {
	previous := domain.ResearchIteration{ID: "it-1", ResearchGaps: []domain.ResearchGap{
		ResearchGapFromMissingEvidence("gap-old-c", domain.ResearchGapComparison, "Comparison trend before treatment", "why", []string{"h1"}),
		ResearchGapFromMissingEvidence("gap-old-t", domain.ResearchGapTiming, "exact policy timing", "why", []string{"h1"}),
	}}
	previous.ResearchGaps[0].FirstSeenIterationID = "it-1"
	current := domain.ResearchIteration{ID: "it-2", ResearchGaps: []domain.ResearchGap{
		ResearchGapFromMissingEvidence("gap-new-c", domain.ResearchGapComparison, "comparison trend before treatment", "why", []string{"h1b"}),
	}}
	current.ResearchGaps[0].FirstSeenIterationID = "it-2"
	req, _ := PlanDataRequirementForGap(current.ResearchGaps[0])
	current.DataRequirements = []domain.DataRequirement{req}

	withoutEvidence := CarryForwardResearchGaps(previous, current, nil)
	if len(withoutEvidence.ResearchGaps) != 2 {
		t.Fatalf("gap that vanished without new evidence must be carried forward: %+v", withoutEvidence.ResearchGaps)
	}
	if withoutEvidence.ResearchGaps[0].ID != "gap-old-c" || withoutEvidence.ResearchGaps[0].FirstSeenIterationID != "it-1" {
		t.Fatalf("matching gap must keep its original identity: %+v", withoutEvidence.ResearchGaps[0])
	}
	if withoutEvidence.DataRequirements[0].GapID != "gap-old-c" {
		t.Fatalf("requirement must follow the stable gap id: %+v", withoutEvidence.DataRequirements[0])
	}
	if withoutEvidence.ResearchGaps[1].Resolved || withoutEvidence.ResearchGaps[1].ID != "gap-old-t" {
		t.Fatalf("timing gap must stay unresolved: %+v", withoutEvidence.ResearchGaps[1])
	}
	if len(withoutEvidence.DataRequirements) != 2 {
		t.Fatalf("carried gap still needs its requirement: %+v", withoutEvidence.DataRequirements)
	}

	withEvidence := CarryForwardResearchGaps(previous, current, []string{"policy chronology"})
	timing := withEvidence.ResearchGaps[1]
	if !timing.Resolved || timing.AddressedInIterationID != "it-2" {
		t.Fatalf("gap absent after added evidence must be marked addressed in this iteration: %+v", timing)
	}
	if len(withEvidence.DataRequirements) != 1 {
		t.Fatalf("resolved gaps need no further requirement: %+v", withEvidence.DataRequirements)
	}
	if len(withEvidence.UnresolvedGapIDs()) != 1 {
		t.Fatalf("comparison gap still open: %v", withEvidence.UnresolvedGapIDs())
	}
}

func TestFinalizeResearchIterationLinksEvidenceAssessesAndStops(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	first := BuildResearchIteration(1, "q", nil, []*domain.Insight{
		{ID: "h1", Title: "policy effect", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified, MissingEvidence: []string{"comparison trend"}},
		{ID: "h2", Title: "macro trend", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified, MissingEvidence: []string{"comparison trend"}},
	}, now)
	first = FinalizeResearchIteration(domain.ResearchRun{}, first, nil, now)
	if first.Readiness.State != domain.ReadinessEvidenceInsufficient || first.Stop != nil {
		t.Fatalf("first pass: %+v", first.Readiness)
	}
	run := domain.ResearchRun{ID: "run", Iterations: []domain.ResearchIteration{first}}

	second := BuildResearchIteration(2, "q", nil, []*domain.Insight{
		{ID: "h1b", Title: "policy effect", ValidationStatus: domain.ValidationSupported, IdentificationStatus: domain.IdentificationNotIdentified},
		{ID: "h2b", Title: "macro trend", ValidationStatus: domain.ValidationContradicted, IdentificationStatus: domain.IdentificationNotIdentified},
	}, now.Add(time.Hour))
	second = FinalizeResearchIteration(run, second, []string{"comparison series"}, now.Add(time.Hour))

	if len(second.AddedEvidence) != 1 || len(second.HypothesisChanges) != 2 {
		t.Fatalf("added evidence and changes must be linked to the new iteration: %+v", second)
	}
	if len(second.ResearchGaps) != 1 || !second.ResearchGaps[0].Resolved || second.ResearchGaps[0].ID != first.ResearchGaps[0].ID {
		t.Fatalf("prior gap must be carried and marked addressed: %+v", second.ResearchGaps)
	}
	if second.Readiness.State != domain.ReadinessDecisionReadyWithLimitation || second.Stop == nil {
		t.Fatalf("second pass: %+v %+v", second.Readiness, second.Stop)
	}
	if second.Readiness.AssessedAt != now.Add(time.Hour) || second.Stop.DecidedAt != now.Add(time.Hour) {
		t.Fatal("timestamps must come from the caller clock")
	}
}

func TestBuildHumanHandoffNeverEndsWithBareAlternatives(t *testing.T) {
	gap := ResearchGapFromMissingEvidence("gap-c", domain.ResearchGapComparison, "comparison trend", "why", []string{"h1b", "h2b"})
	req, _ := PlanDataRequirementForGap(gap)
	second := PrioritizeResearchIteration(domain.ResearchIteration{
		AddedEvidence: []string{"pre-period series"},
		HypothesisStates: []domain.HypothesisState{
			{HypothesisID: "h1b", ComparisonKey: "policy effect", ValidationStatus: domain.ValidationSupported, IdentificationStatus: domain.IdentificationNotIdentified},
			{HypothesisID: "h2b", ComparisonKey: "macro trend", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified},
		},
		ResearchGaps: []domain.ResearchGap{gap}, DataRequirements: []domain.DataRequirement{req},
		WhatWeCannotConclude: []string{"Causality is not identified."},
	})
	run := twoIterationRun(second)
	run.Iterations[1].Readiness = AssessDecisionReadiness(run, time.Now())
	stopped, err := run.Iterations[1].ApplyHumanOverride(domain.HumanOverride{StopReason: domain.StopExternalBudgetBoundary, Note: "sprint ended"})
	if err != nil {
		t.Fatal(err)
	}
	run.Iterations[1] = stopped

	handoff := BuildHumanHandoff(run)

	if handoff.Readiness != domain.ReadinessEvidenceConverging || handoff.IterationCount != 2 {
		t.Fatalf("handoff must carry readiness and history depth: %+v", handoff)
	}
	if len(handoff.StrongestSurvivingHypotheses) != 1 || handoff.StrongestSurvivingHypotheses[0].HypothesisID != "h1b" {
		t.Fatalf("strongest surviving: %+v", handoff.StrongestSurvivingHypotheses)
	}
	if len(handoff.UnresolvedAlternatives) != 1 || handoff.UnresolvedAlternatives[0].HypothesisID != "h2b" {
		t.Fatalf("unresolved alternatives: %+v", handoff.UnresolvedAlternatives)
	}
	if len(handoff.RemainingUncertainty) != 1 || len(handoff.SuggestedNextResearch) != 1 || handoff.SuggestedNextResearch[0].Priority.Rank != 1 {
		t.Fatalf("handoff must say what to research next: %+v", handoff)
	}
	if handoff.Stop == nil || handoff.Stop.Reason != domain.StopExternalBudgetBoundary || len(handoff.HumanOverrides) != 1 {
		t.Fatalf("why the loop stopped must be explicit: %+v", handoff.Stop)
	}
	if len(handoff.EvidenceAddedAcrossIterations) != 1 || len(handoff.WhatWeCannotConclude) != 1 {
		t.Fatalf("evidence history and boundaries missing: %+v", handoff)
	}
}
