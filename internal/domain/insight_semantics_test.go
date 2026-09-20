package domain

import "testing"

func TestInsightSemanticEnumsAreBounded(t *testing.T) {
	for _, kind := range []InsightReferenceKind{
		InsightRefObservation, InsightRefFinding, InsightRefVariable, InsightRefClaim, InsightRefContext, InsightRefOther,
	} {
		if !kind.Valid() {
			t.Fatalf("expected reference kind %q to be valid", kind)
		}
	}
	if InsightReferenceKind("MAGIC").Valid() {
		t.Fatal("unknown reference kind must not be accepted")
	}

	for _, kind := range []ConnectionKind{
		ConnectionAssociation, ConnectionContrast, ConnectionSequence, ConnectionInteraction,
		ConnectionConstraint, ConnectionMechanismCandidate, ConnectionOther,
	} {
		if !kind.Valid() {
			t.Fatalf("expected connection kind %q to be valid", kind)
		}
	}
	if ConnectionKind("CAUSES").Valid() {
		t.Fatal("connection kind must not smuggle in causal validity")
	}
}

func TestGeneralizationDefaultsToUncertifiedCandidateState(t *testing.T) {
	if !GeneralizationStatus("").Valid() {
		t.Fatal("empty status must remain valid for backward-compatible insights")
	}
	if !GeneralizationCandidate.Valid() || !GeneralizationSupported.Valid() || !GeneralizationRejected.Valid() {
		t.Fatal("known generalization states must be valid")
	}
	if GeneralizationStatus("MODEL_CERTIFIED").Valid() {
		t.Fatal("the model cannot self-certify generalization")
	}
}

func TestMechanismKeepsUnsupportedBridgeExplicit(t *testing.T) {
	m := MechanismCandidate{
		Statement: "A may influence C through B",
		Steps: []MechanismStep{{
			Statement:       "B bridges A and C",
			Assumptions:     []string{"B occurs after A"},
			MissingEvidence: []string{"direct observation of B"},
		}},
	}
	if len(m.Steps) != 1 || len(m.Steps[0].MissingEvidence) != 1 {
		t.Fatalf("unsupported bridge must remain inspectable: %+v", m)
	}
}
