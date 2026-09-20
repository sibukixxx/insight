package domain

import (
	"testing"
	"time"
)

func TestValidationEvidenceProvenanceRejectsSameIterationPostHocEvidence(t *testing.T) {
	now := time.Now()
	e := Expectation{
		ID: "exp-1", Statement: "x", Provenance: ExpectationModelProposedPostHoc,
		AuthorType: AuthorModel, ResearchIterationID: "it-1",
		ObservedDataAvailableAtCreation: true,
		FalsificationCriteria: []string{"not x"}, CreatedAt: now,
	}
	v := ValidationEvidenceProvenance{
		ExpectationID: e.ID, EvidenceReference: "same.csv", EvidenceIterationID: "it-1",
		Relation: ValidationEvidencePreObservationSameIteration,
		Rationale: "same pass", RecordedBy: AuthorHuman, RecordedAt: now,
	}
	if err := v.ValidateAgainst(e); err == nil {
		t.Fatal("post-hoc same-iteration evidence must not validate its own expectation")
	}
}

func TestValidationEvidenceProvenanceAcceptsIndependentExternalEvidence(t *testing.T) {
	now := time.Now()
	e := Expectation{
		ID: "exp-1", Statement: "x", Provenance: ExpectationPrior,
		AuthorType: AuthorHuman, ResearchIterationID: "it-1",
		FalsificationCriteria: []string{"not x"}, CreatedAt: now,
	}
	v := ValidationEvidenceProvenance{
		ExpectationID: e.ID, EvidenceReference: "independent.csv",
		Relation: ValidationEvidenceExternalIndependent,
		Rationale: "collected independently after the expectation was fixed",
		RecordedBy: AuthorHuman, RecordedAt: now,
	}
	if err := v.ValidateAgainst(e); err != nil {
		t.Fatalf("independent evidence provenance should validate: %v", err)
	}
}

func TestEvidenceAdditionAddressesOnlyNamedGaps(t *testing.T) {
	e := EvidenceAddition{Reference: "evidence.csv", GapIDs: []string{"g1"}}
	if !e.Addresses("g1") || e.Addresses("g2") {
		t.Fatalf("gap linkage must be exact: %+v", e)
	}
}
