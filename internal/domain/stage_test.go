package domain

import (
	"errors"
	"testing"
)

func TestResearchStageValid(t *testing.T) {
	for _, s := range []ResearchStage{StageDiscovery, StageExploratory, StageValidation, StageSynthesis} {
		if !s.Valid() {
			t.Errorf("expected %q to be valid", s)
		}
	}
	for _, s := range []ResearchStage{"", "discovery", "PUBLISHED"} {
		if s.Valid() {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func TestResearchStageFindingKindShouldSeparateExploratoryFromValidation(t *testing.T) {
	cases := map[ResearchStage]FindingKind{
		StageDiscovery:   FindingExploratory,
		StageExploratory: FindingExploratory,
		StageValidation:  FindingValidationResult,
		StageSynthesis:   FindingSynthesis,
		"":               FindingExploratory,
	}
	for stage, want := range cases {
		if got := stage.FindingKind(); got != want {
			t.Errorf("%q: got %q, want %q", stage, got, want)
		}
	}
}

func frozenPrior(t *testing.T) Expectation {
	t.Helper()
	frozen, err := priorExpectation().Freeze(expectationCreatedAt)
	if err != nil {
		t.Fatalf("freeze failed: %v", err)
	}
	return frozen
}

func TestResearchStageTransitionShouldAllowDiscoveryToExploratoryWhenObservationsExist(t *testing.T) {
	if err := StageDiscovery.Transition(StageExploratory, StageTransitionInput{ObservationCount: 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err := StageDiscovery.Transition(StageExploratory, StageTransitionInput{})
	if !errors.Is(err, ErrStageRequiresObservations) {
		t.Fatalf("got %v, want ErrStageRequiresObservations", err)
	}
}

func TestResearchStageTransitionShouldAllowExploratoryToValidationWhenTargetsAreFrozenAndIndependentEvidenceIsIdentified(t *testing.T) {
	in := StageTransitionInput{
		Expectations:               []Expectation{frozenPrior(t)},
		IndependentEvidencePlanned: true,
	}
	if err := StageExploratory.Transition(StageValidation, in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResearchStageTransitionShouldRejectExploratoryToValidationWithoutFrozenTarget(t *testing.T) {
	in := StageTransitionInput{Expectations: []Expectation{priorExpectation()}, IndependentEvidencePlanned: true}
	err := StageExploratory.Transition(StageValidation, in)
	if !errors.Is(err, ErrStageRequiresFrozenTarget) {
		t.Fatalf("got %v, want ErrStageRequiresFrozenTarget", err)
	}
	if err := StageExploratory.Transition(StageValidation, StageTransitionInput{IndependentEvidencePlanned: true}); !errors.Is(err, ErrStageRequiresFrozenTarget) {
		t.Fatalf("no expectations: got %v, want ErrStageRequiresFrozenTarget", err)
	}
}

func TestResearchStageTransitionShouldRejectExploratoryToValidationWithoutIndependentEvidence(t *testing.T) {
	in := StageTransitionInput{Expectations: []Expectation{frozenPrior(t)}}
	err := StageExploratory.Transition(StageValidation, in)
	if !errors.Is(err, ErrStageRequiresIndependentEvidence) {
		t.Fatalf("got %v, want ErrStageRequiresIndependentEvidence", err)
	}
}

func TestResearchStageTransitionShouldRejectValidationWhenFrozenTargetIsInvalid(t *testing.T) {
	broken := frozenPrior(t)
	broken.ObservedDataAvailableAtCreation = true // PRIOR label contradicts creation context
	in := StageTransitionInput{Expectations: []Expectation{broken}, IndependentEvidencePlanned: true}

	err := StageExploratory.Transition(StageValidation, in)
	if !errors.Is(err, ErrExpectationPriorAfterObservation) {
		t.Fatalf("got %v, want ErrExpectationPriorAfterObservation", err)
	}
}

func TestResearchStageTransitionShouldAllowValidationToSynthesisOnlyAfterValidationRuns(t *testing.T) {
	if err := StageValidation.Transition(StageSynthesis, StageTransitionInput{CompletedValidationCount: 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err := StageValidation.Transition(StageSynthesis, StageTransitionInput{})
	if !errors.Is(err, ErrStageRequiresValidationResults) {
		t.Fatalf("got %v, want ErrStageRequiresValidationResults", err)
	}
}

func TestResearchStageTransitionShouldAlwaysAllowSteppingBack(t *testing.T) {
	back := []struct{ from, to ResearchStage }{
		{StageExploratory, StageDiscovery},
		{StageValidation, StageExploratory},
		{StageSynthesis, StageExploratory},
		{StageSynthesis, StageValidation},
	}
	for _, tr := range back {
		if err := tr.from.Transition(tr.to, StageTransitionInput{}); err != nil {
			t.Errorf("%s -> %s: unexpected error %v", tr.from, tr.to, err)
		}
	}
}

func TestResearchStageTransitionShouldRejectSkipsSelfAndInvalidStages(t *testing.T) {
	rejected := []struct{ from, to ResearchStage }{
		{StageDiscovery, StageValidation},
		{StageDiscovery, StageSynthesis},
		{StageExploratory, StageSynthesis},
		{StageExploratory, StageExploratory},
		{StageDiscovery, "PUBLISHED"},
		{"", StageExploratory},
	}
	for _, tr := range rejected {
		err := tr.from.Transition(tr.to, StageTransitionInput{ObservationCount: 5, CompletedValidationCount: 5, IndependentEvidencePlanned: true, Expectations: []Expectation{frozenPrior(t)}})
		if !errors.Is(err, ErrStageTransitionNotAllowed) {
			t.Errorf("%q -> %q: got %v, want ErrStageTransitionNotAllowed", tr.from, tr.to, err)
		}
	}
}
