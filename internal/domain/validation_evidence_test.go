package domain

import (
	"testing"
	"time"
)

func TestValidationEvidenceProvenanceRejectsSameIterationEvidence(t *testing.T) {
	expectation := Expectation{ID: "exp-1", FrozenForValidation: true}
	v := ValidationEvidenceProvenance{
		ExpectationID: "exp-1",
		EvidenceReference: "dataset-b",
		SourceIterationID: "it-2",
		IndependenceRationale: "held out from exploratory iteration",
		RecordedAt: time.Now(),
	}
	if err := v.Validate("it-2", []Expectation{expectation}); err == nil {
		t.Fatal("same-iteration evidence must not be accepted as independent")
	}
}

func TestValidationEvidenceProvenanceRequiresFrozenExpectation(t *testing.T) {
	v := ValidationEvidenceProvenance{
		ExpectationID: "exp-1",
		EvidenceReference: "dataset-b",
		SourceIterationID: "it-1",
		IndependenceRationale: "collected before validation",
	}
	if err := v.Validate("it-2", []Expectation{{ID: "exp-1"}}); err == nil {
		t.Fatal("validation evidence must reference a frozen expectation")
	}
}

func TestValidationEvidenceProvenanceAcceptsIndependentFrozenTarget(t *testing.T) {
	v := ValidationEvidenceProvenance{
		ExpectationID: "exp-1",
		EvidenceReference: "dataset-b",
		SourceIterationID: "it-1",
		DatasetReference: "holdout.csv",
		IndependenceRationale: "not used to generate the expectation",
	}
	if err := v.Validate("it-2", []Expectation{{ID: "exp-1", FrozenForValidation: true}}); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
