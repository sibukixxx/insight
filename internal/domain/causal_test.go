package domain

import "testing"

func TestExpectationBasisValidShouldAcceptProvenanceAndLegacyValues(t *testing.T) {
	valid := []ExpectationBasis{
		ExpectationPrior, ExpectationLiterature, ExpectationDomainKnowledge,
		ExpectationModelProposedPostHoc, ExpectationHumanPostHoc, ExpectationDerivedFromPriorRun,
		ExpectationOther, ExpectationUnknown,
		ExpectationSourceBacked, ExpectationModelProposed,
	}
	for _, b := range valid {
		if !b.Valid() {
			t.Errorf("expected %q to be valid", b)
		}
	}
	invalid := []ExpectationBasis{"", "prior", "GUESS"}
	for _, b := range invalid {
		if b.Valid() {
			t.Errorf("expected %q to be invalid", b)
		}
	}
}

func TestExpectationBasisIsPostHocShouldBeTrueOnlyForPostObservationProvenance(t *testing.T) {
	postHoc := []ExpectationBasis{ExpectationModelProposedPostHoc, ExpectationHumanPostHoc, ExpectationModelProposed}
	for _, b := range postHoc {
		if !b.IsPostHoc() {
			t.Errorf("expected %q to be post hoc", b)
		}
	}
	notPostHoc := []ExpectationBasis{
		ExpectationPrior, ExpectationLiterature, ExpectationDomainKnowledge, ExpectationDerivedFromPriorRun,
		ExpectationUnknown, ExpectationOther, ExpectationSourceBacked, "",
	}
	for _, b := range notPostHoc {
		if b.IsPostHoc() {
			t.Errorf("expected %q not to be flagged post hoc", b)
		}
	}
}

func TestExpectationBasisObservationTimingShouldStayUnknownWhenProvenanceIsMissing(t *testing.T) {
	cases := map[ExpectationBasis]ObservationTiming{
		ExpectationPrior:                TimingPreObservation,
		ExpectationLiterature:           TimingPreObservation,
		ExpectationDomainKnowledge:      TimingPreObservation,
		ExpectationDerivedFromPriorRun:  TimingPreObservation,
		ExpectationModelProposedPostHoc: TimingPostObservation,
		ExpectationHumanPostHoc:         TimingPostObservation,
		ExpectationModelProposed:        TimingPostObservation,
		ExpectationSourceBacked:         TimingUnknown,
		ExpectationUnknown:              TimingUnknown,
		ExpectationOther:                TimingUnknown,
		"":                              TimingUnknown,
	}
	for basis, want := range cases {
		if got := basis.ObservationTiming(); got != want {
			t.Errorf("%q: got timing %q, want %q", basis, got, want)
		}
	}
}

func TestExpectationBasisNormalizeShouldMapLegacyModelProposedToPostHoc(t *testing.T) {
	if got := ExpectationModelProposed.Normalize(); got != ExpectationModelProposedPostHoc {
		t.Fatalf("legacy MODEL_PROPOSED normalized to %q", got)
	}
	if got := ExpectationBasis("").Normalize(); got != ExpectationUnknown {
		t.Fatalf("empty basis normalized to %q, want UNKNOWN", got)
	}
	if got := ExpectationSourceBacked.Normalize(); got != ExpectationSourceBacked {
		t.Fatalf("legacy SOURCE_BACKED must be kept as is, got %q", got)
	}
	if got := ExpectationBasis("GUESS").Normalize(); got != ExpectationUnknown {
		t.Fatalf("unrecognised basis normalized to %q, want UNKNOWN", got)
	}
}

func TestExpectationBasisNormalizeShouldNeverPromotePostHocToPrior(t *testing.T) {
	for _, b := range []ExpectationBasis{ExpectationModelProposedPostHoc, ExpectationHumanPostHoc, ExpectationModelProposed} {
		if got := b.Normalize(); got == ExpectationPrior || !got.IsPostHoc() {
			t.Errorf("%q normalized to %q; post hoc provenance must stay post hoc", b, got)
		}
	}
}

func TestExpectationBasisRequiresSourceReferenceShouldBeTrueForLiteratureAndDomainKnowledge(t *testing.T) {
	if !ExpectationLiterature.RequiresSourceReference() || !ExpectationDomainKnowledge.RequiresSourceReference() {
		t.Fatal("LITERATURE and DOMAIN_KNOWLEDGE must be traceable to a source or justification")
	}
	for _, b := range []ExpectationBasis{ExpectationPrior, ExpectationModelProposedPostHoc, ExpectationUnknown, ExpectationOther} {
		if b.RequiresSourceReference() {
			t.Errorf("%q should not require a source reference", b)
		}
	}
}

func TestExpectationBasisPermitsStrongValidationClaimShouldRejectPostHocUnknownAndLegacy(t *testing.T) {
	permitted := []ExpectationBasis{ExpectationPrior, ExpectationLiterature, ExpectationDomainKnowledge, ExpectationDerivedFromPriorRun}
	for _, b := range permitted {
		if !b.PermitsStrongValidationClaim() {
			t.Errorf("expected %q to permit a strong validation claim", b)
		}
	}
	rejected := []ExpectationBasis{
		ExpectationModelProposedPostHoc, ExpectationHumanPostHoc, ExpectationUnknown, ExpectationOther,
		ExpectationSourceBacked, ExpectationModelProposed, "",
	}
	for _, b := range rejected {
		if b.PermitsStrongValidationClaim() {
			t.Errorf("expected %q to be barred from a strong validation claim", b)
		}
	}
}
