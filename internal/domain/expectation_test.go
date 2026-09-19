package domain

import (
	"errors"
	"testing"
	"time"
)

var expectationCreatedAt = time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)

func priorExpectation() Expectation {
	return Expectation{
		ID: "exp-prior", Statement: "corporate registrations rise after the subsidy starts",
		Provenance: ExpectationPrior, AuthorType: AuthorHuman, ResearchIterationID: "iter-1",
		FalsificationCriteria: []string{"registrations flat or falling in the treated period"},
		CreatedAt:             expectationCreatedAt,
	}
}

func postHocExpectation() Expectation {
	return Expectation{
		ID: "exp-posthoc", Statement: "registrations rise only in municipalities with a startup desk",
		Provenance: ExpectationModelProposedPostHoc, AuthorType: AuthorModel, ResearchIterationID: "iter-1",
		ObservedDataAvailableAtCreation: true,
		FalsificationCriteria:           []string{"increase also appears where no desk exists"},
		CreatedAt:                       expectationCreatedAt,
	}
}

func TestAuthorTypeValid(t *testing.T) {
	for _, a := range []AuthorType{AuthorHuman, AuthorModel, AuthorImported} {
		if !a.Valid() {
			t.Errorf("expected %q to be valid", a)
		}
	}
	for _, a := range []AuthorType{"", "human", "LLM"} {
		if a.Valid() {
			t.Errorf("expected %q to be invalid", a)
		}
	}
}

func TestExpectationValidateShouldPassForWellFormedPriorExpectation(t *testing.T) {
	if err := priorExpectation().Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExpectationValidateShouldRejectPriorProvenanceWhenObservedDataWasAvailable(t *testing.T) {
	e := priorExpectation()
	e.ObservedDataAvailableAtCreation = true

	err := e.Validate()
	if !errors.Is(err, ErrExpectationPriorAfterObservation) {
		t.Fatalf("got %v, want ErrExpectationPriorAfterObservation", err)
	}
}

func TestExpectationValidateShouldRejectPostHocProvenanceWithoutObservedData(t *testing.T) {
	e := postHocExpectation()
	e.ObservedDataAvailableAtCreation = false

	err := e.Validate()
	if !errors.Is(err, ErrExpectationPostHocWithoutObservation) {
		t.Fatalf("got %v, want ErrExpectationPostHocWithoutObservation", err)
	}
}

func TestExpectationValidateShouldRequireSourceForLiterature(t *testing.T) {
	e := priorExpectation()
	e.Provenance = ExpectationLiterature

	if err := e.Validate(); !errors.Is(err, ErrExpectationSourceRequired) {
		t.Fatalf("got %v, want ErrExpectationSourceRequired", err)
	}
	e.SourceReferences = []string{"doi:10.1000/example"}
	if err := e.Validate(); err != nil {
		t.Fatalf("literature expectation with a citation should validate: %v", err)
	}
}

func TestExpectationValidateShouldAcceptJustificationForDomainKnowledge(t *testing.T) {
	e := priorExpectation()
	e.Provenance = ExpectationDomainKnowledge
	e.Justification = "regional bank loan officers report registration spikes after subsidy launches"

	if err := e.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExpectationValidateShouldRejectMissingStatementProvenanceAuthorAndTimestamp(t *testing.T) {
	cases := map[string]struct {
		mutate func(*Expectation)
		want   error
	}{
		"statement":  {func(e *Expectation) { e.Statement = "  " }, ErrExpectationStatementRequired},
		"provenance": {func(e *Expectation) { e.Provenance = "GUESS" }, ErrExpectationProvenanceInvalid},
		"author":     {func(e *Expectation) { e.AuthorType = "" }, ErrExpectationAuthorInvalid},
		"createdAt":  {func(e *Expectation) { e.CreatedAt = time.Time{} }, ErrExpectationCreatedAtRequired},
	}
	for name, tc := range cases {
		e := priorExpectation()
		tc.mutate(&e)
		if err := e.Validate(); !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", name, err, tc.want)
		}
	}
}

func TestExpectationValidateShouldTreatEmptyProvenanceAsUnknownNotError(t *testing.T) {
	e := priorExpectation()
	e.Provenance = ""

	if err := e.Validate(); err != nil {
		t.Fatalf("missing provenance must be kept as UNKNOWN, not rejected: %v", err)
	}
	if got := e.EffectiveProvenance(); got != ExpectationUnknown {
		t.Fatalf("effective provenance %q, want UNKNOWN", got)
	}
}

func TestExpectationEffectiveProvenanceShouldNormalizeLegacyModelProposed(t *testing.T) {
	e := postHocExpectation()
	e.Provenance = ExpectationModelProposed

	if got := e.EffectiveProvenance(); got != ExpectationModelProposedPostHoc {
		t.Fatalf("got %q, want MODEL_PROPOSED_POST_HOC", got)
	}
}

func TestExpectationCreatedBeforeObservationShouldBeFalseForPostHocOrObservedData(t *testing.T) {
	if !priorExpectation().CreatedBeforeObservation() {
		t.Fatal("prior expectation without observed data must count as pre-observation")
	}
	if postHocExpectation().CreatedBeforeObservation() {
		t.Fatal("post hoc expectation must never be presented as pre-observation")
	}
	e := priorExpectation()
	e.ObservedDataAvailableAtCreation = true
	if e.CreatedBeforeObservation() {
		t.Fatal("expectation created with observed data available must not count as pre-observation")
	}
	unknown := priorExpectation()
	unknown.Provenance = ExpectationUnknown
	if unknown.CreatedBeforeObservation() {
		t.Fatal("unknown provenance must not be presented as pre-observation")
	}
}

func TestExpectationFreezeShouldReturnFrozenCopyAndLeaveOriginalUntouched(t *testing.T) {
	original := priorExpectation()
	frozenAt := expectationCreatedAt.Add(time.Hour)

	frozen, err := original.Freeze(frozenAt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if original.FrozenForValidation || original.FrozenAt != nil {
		t.Fatal("Freeze mutated the original expectation")
	}
	if !frozen.FrozenForValidation || frozen.FrozenAt == nil || !frozen.FrozenAt.Equal(frozenAt) {
		t.Fatalf("frozen copy not marked: %+v", frozen)
	}
}

func TestExpectationFreezeShouldRejectMissingCriteriaUnknownProvenanceAndDoubleFreeze(t *testing.T) {
	noCriteria := priorExpectation()
	noCriteria.FalsificationCriteria = nil
	if _, err := noCriteria.Freeze(expectationCreatedAt); !errors.Is(err, ErrExpectationCriteriaRequired) {
		t.Errorf("no criteria: got %v, want ErrExpectationCriteriaRequired", err)
	}

	unknown := priorExpectation()
	unknown.Provenance = ExpectationUnknown
	if _, err := unknown.Freeze(expectationCreatedAt); !errors.Is(err, ErrExpectationProvenanceUnknown) {
		t.Errorf("unknown provenance: got %v, want ErrExpectationProvenanceUnknown", err)
	}

	frozen, err := priorExpectation().Freeze(expectationCreatedAt)
	if err != nil {
		t.Fatalf("first freeze failed: %v", err)
	}
	if _, err := frozen.Freeze(expectationCreatedAt.Add(time.Minute)); !errors.Is(err, ErrExpectationAlreadyFrozen) {
		t.Errorf("double freeze: got %v, want ErrExpectationAlreadyFrozen", err)
	}
}

func TestExpectationFreezeShouldAllowPostHocExpectationToBeFixedForLaterValidation(t *testing.T) {
	frozen, err := postHocExpectation().Freeze(expectationCreatedAt)
	if err != nil {
		t.Fatalf("post hoc expectations may be frozen so the next iteration can test them: %v", err)
	}
	if frozen.EffectiveProvenance() != ExpectationModelProposedPostHoc {
		t.Fatalf("freezing must not change provenance, got %q", frozen.EffectiveProvenance())
	}
}

func TestExpectationDeriveForValidationShouldCreatePriorRunLineageWithoutObservedData(t *testing.T) {
	source := postHocExpectation()
	derivedAt := expectationCreatedAt.Add(24 * time.Hour)

	derived := source.DeriveForValidation("exp-2", "iter-2", derivedAt)

	if derived.Provenance != ExpectationDerivedFromPriorRun {
		t.Fatalf("provenance %q, want DERIVED_FROM_PRIOR_RUN", derived.Provenance)
	}
	if derived.DerivedFromExpectationID != source.ID || derived.ResearchIterationID != "iter-2" || derived.ID != "exp-2" {
		t.Fatalf("lineage not recorded: %+v", derived)
	}
	if derived.ObservedDataAvailableAtCreation || derived.FrozenForValidation || derived.FrozenAt != nil {
		t.Fatalf("derived expectation must start unfrozen and before observing the new data: %+v", derived)
	}
	if derived.Statement != source.Statement || len(derived.FalsificationCriteria) != len(source.FalsificationCriteria) || !derived.CreatedAt.Equal(derivedAt) {
		t.Fatalf("statement/criteria/createdAt not carried over: %+v", derived)
	}
	if err := derived.Validate(); err != nil {
		t.Fatalf("derived expectation should validate: %v", err)
	}
	if source.Provenance != ExpectationModelProposedPostHoc {
		t.Fatal("source expectation must remain post hoc")
	}
}

func TestExpectationIsIndependentEvidenceShouldRejectSameIterationWhenDataWasObservedAtCreation(t *testing.T) {
	e := postHocExpectation()
	if e.IsIndependentEvidence("iter-1") {
		t.Fatal("same-run exploratory evidence must not count as independent validation evidence")
	}
	if !e.IsIndependentEvidence("iter-2") {
		t.Fatal("evidence from a later iteration is independent of the generating run")
	}
	if e.IsIndependentEvidence("") {
		t.Fatal("evidence without an iteration reference cannot be shown to be independent")
	}
}

func TestExpectationIsIndependentEvidenceShouldAcceptSameIterationWhenFixedBeforeObservation(t *testing.T) {
	e := priorExpectation()
	if !e.IsIndependentEvidence("iter-1") {
		t.Fatal("an expectation fixed before observing the data is independent of that data")
	}
}

func TestExpectationPermitsStrongValidationClaimShouldRequireFrozenPreObservationKnownProvenance(t *testing.T) {
	unfrozen := priorExpectation()
	if unfrozen.PermitsStrongValidationClaim() {
		t.Fatal("unfrozen expectation must not support a strong validation claim")
	}

	frozen, _ := unfrozen.Freeze(expectationCreatedAt)
	if !frozen.PermitsStrongValidationClaim() {
		t.Fatal("frozen prior expectation should support a strong validation claim")
	}

	frozenPostHoc, _ := postHocExpectation().Freeze(expectationCreatedAt)
	if frozenPostHoc.PermitsStrongValidationClaim() {
		t.Fatal("post hoc expectation must not support a strong validation claim even when frozen")
	}

	derived := postHocExpectation().DeriveForValidation("exp-2", "iter-2", expectationCreatedAt)
	frozenDerived, _ := derived.Freeze(expectationCreatedAt)
	if !frozenDerived.PermitsStrongValidationClaim() {
		t.Fatal("an expectation carried into a new run and frozen may support a strong claim")
	}
}
