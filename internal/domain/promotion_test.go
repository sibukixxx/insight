package domain

import (
	"errors"
	"testing"
)

func TestContributionTypeValid(t *testing.T) {
	for _, c := range []ContributionType{
		ContributionNovelMismatch, ContributionCorrection, ContributionRefinement,
		ContributionReplication, ContributionDefinitionAudit, ContributionCounterEvidence,
		ContributionInconclusiveButDecisionRelevant, ContributionOther,
	} {
		if !c.Valid() {
			t.Errorf("expected %q to be valid", c)
		}
	}
	for _, c := range []ContributionType{"", "SURPRISING", "novel_mismatch"} {
		if c.Valid() {
			t.Errorf("expected %q to be invalid", c)
		}
	}
}

func TestContributionTypePermitsPublicationValueShouldNotRequireMismatch(t *testing.T) {
	for _, c := range []ContributionType{
		ContributionNovelMismatch, ContributionCorrection, ContributionRefinement,
		ContributionReplication, ContributionDefinitionAudit, ContributionCounterEvidence,
		ContributionInconclusiveButDecisionRelevant,
	} {
		if !c.PermitsPublicationValue() {
			t.Errorf("expected %q to permit publication value", c)
		}
	}
}

func TestContributionTypePermitsPublicationValueShouldRejectOtherAndInvalid(t *testing.T) {
	for _, c := range []ContributionType{ContributionOther, "", "UNKNOWN"} {
		if c.PermitsPublicationValue() {
			t.Errorf("expected %q not to permit publication value on its own", c)
		}
	}
}

func TestPromotionStateValid(t *testing.T) {
	for _, s := range []PromotionState{
		PromotionDraft, PromotionResearchComplete, PromotionHumanReviewRequired,
		PromotionPublicationReady, PromotionPublished, PromotionRejectedForPublication,
	} {
		if !s.Valid() {
			t.Errorf("expected %q to be valid", s)
		}
	}
	for _, s := range []PromotionState{"", "APPROVED"} {
		if s.Valid() {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func completeChecklist() PublicationChecklist {
	return PublicationChecklist{
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
	}
}

func TestPublicationChecklistSatisfiedShouldRequireEveryItem(t *testing.T) {
	if !completeChecklist().Satisfied() {
		t.Fatal("expected a fully-checked checklist to be satisfied")
	}
	broken := completeChecklist()
	broken.LimitationsPresent = false
	if broken.Satisfied() {
		t.Fatal("expected a checklist missing one item to be unsatisfied")
	}
}

func TestPublicationChecklistUnmetItemsShouldNameMissingChecks(t *testing.T) {
	checklist := completeChecklist()
	checklist.CounterEvidenceSearched = false
	checklist.LimitationsPresent = false

	got := checklist.UnmetItems()
	want := []string{"counter_evidence_searched", "limitations_present"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestPromotionStateTransitionShouldAllowDraftToResearchComplete(t *testing.T) {
	if err := PromotionDraft.Transition(PromotionResearchComplete, PromotionGateInput{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPromotionStateTransitionShouldRequireContributionThatPermitsPublicationValue(t *testing.T) {
	in := PromotionGateInput{Contribution: ContributionOther}
	err := PromotionResearchComplete.Transition(PromotionHumanReviewRequired, in)
	if !errors.Is(err, ErrPromotionContributionRequired) {
		t.Fatalf("got %v, want ErrPromotionContributionRequired", err)
	}

	in.Contribution = ContributionReplication
	if err := PromotionResearchComplete.Transition(PromotionHumanReviewRequired, in); err != nil {
		t.Fatalf("unexpected error for REPLICATION without mismatch: %v", err)
	}
}

func TestPromotionStateTransitionShouldRejectInvalidContribution(t *testing.T) {
	in := PromotionGateInput{Contribution: "SURPRISING"}
	err := PromotionResearchComplete.Transition(PromotionHumanReviewRequired, in)
	if !errors.Is(err, ErrPromotionContributionInvalid) {
		t.Fatalf("got %v, want ErrPromotionContributionInvalid", err)
	}
}

func TestPromotionStateTransitionToPublicationReadyShouldRequireCompleteChecklist(t *testing.T) {
	in := PromotionGateInput{
		Contribution:         ContributionNovelMismatch,
		Checklist:            completeChecklist(),
		HumanReviewCompleted: true,
	}
	in.Checklist.ObservationGrounded = false

	err := PromotionHumanReviewRequired.Transition(PromotionPublicationReady, in)
	if !errors.Is(err, ErrPromotionChecklistIncomplete) {
		t.Fatalf("got %v, want ErrPromotionChecklistIncomplete", err)
	}
}

func TestPromotionStateTransitionToPublicationReadyShouldRequireHumanReview(t *testing.T) {
	in := PromotionGateInput{
		Contribution: ContributionNovelMismatch,
		Checklist:    completeChecklist(),
	}
	err := PromotionHumanReviewRequired.Transition(PromotionPublicationReady, in)
	if !errors.Is(err, ErrPromotionHumanReviewRequired) {
		t.Fatalf("got %v, want ErrPromotionHumanReviewRequired", err)
	}
}

func TestPromotionStateTransitionToPublicationReadyShouldBlockStrongClaimWithUnresolvedCriticalGap(t *testing.T) {
	in := PromotionGateInput{
		Contribution:             ContributionNovelMismatch,
		Checklist:                completeChecklist(),
		HumanReviewCompleted:     true,
		MakesStrongClaim:         true,
		HasUnresolvedCriticalGap: true,
	}
	err := PromotionHumanReviewRequired.Transition(PromotionPublicationReady, in)
	if !errors.Is(err, ErrPromotionUnresolvedCriticalGap) {
		t.Fatalf("got %v, want ErrPromotionUnresolvedCriticalGap", err)
	}

	in.MakesStrongClaim = false
	if err := PromotionHumanReviewRequired.Transition(PromotionPublicationReady, in); err != nil {
		t.Fatalf("expected inconclusive/weak claim with unresolved gap to be publishable, got %v", err)
	}
}

func TestPromotionStateTransitionToPublicationReadyShouldSucceedWhenEverythingIsSatisfied(t *testing.T) {
	in := PromotionGateInput{
		Contribution:         ContributionCorrection,
		Checklist:            completeChecklist(),
		HumanReviewCompleted: true,
	}
	if err := PromotionHumanReviewRequired.Transition(PromotionPublicationReady, in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPromotionStateTransitionShouldAllowPublicationReadyToPublished(t *testing.T) {
	if err := PromotionPublicationReady.Transition(PromotionPublished, PromotionGateInput{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPromotionStateTransitionShouldAllowRejectionFromAnyNonTerminalState(t *testing.T) {
	for _, s := range []PromotionState{
		PromotionDraft, PromotionResearchComplete, PromotionHumanReviewRequired, PromotionPublicationReady,
	} {
		if err := s.Transition(PromotionRejectedForPublication, PromotionGateInput{}); err != nil {
			t.Errorf("%q -> REJECTED_FOR_PUBLICATION: unexpected error %v", s, err)
		}
	}
}

func TestPromotionStateTransitionShouldRejectSkipsAndInvalidStates(t *testing.T) {
	in := PromotionGateInput{
		Contribution:         ContributionNovelMismatch,
		Checklist:            completeChecklist(),
		HumanReviewCompleted: true,
	}
	rejected := []struct{ from, to PromotionState }{
		{PromotionDraft, PromotionPublicationReady},
		{PromotionDraft, PromotionPublished},
		{PromotionResearchComplete, PromotionPublished},
		{PromotionPublished, PromotionDraft},
		{PromotionDraft, PromotionDraft},
		{PromotionDraft, "APPROVED"},
		{"", PromotionResearchComplete},
	}
	for _, tr := range rejected {
		err := tr.from.Transition(tr.to, in)
		if !errors.Is(err, ErrPromotionTransitionNotAllowed) {
			t.Errorf("%q -> %q: got %v, want ErrPromotionTransitionNotAllowed", tr.from, tr.to, err)
		}
	}
}
