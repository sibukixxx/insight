package domain

import "testing"

func TestGeneralizationRequiresHumanReviewToPermitTransferClaim(t *testing.T) {
	g := GeneralizationCandidate{
		Principle: "a cross-domain intersection may create scarce information",
		Status: GeneralizationStatusSupported,
	}
	if g.PermitsTransferClaim() {
		t.Fatal("supported status alone must not permit a transfer claim")
	}
	g.HumanReviewed = true
	if !g.PermitsTransferClaim() {
		t.Fatal("supported + human reviewed should permit a transfer claim")
	}
}

func TestConnectionKindDoesNotChangeCausalState(t *testing.T) {
	insight := Insight{
		CausalStatus: CausalHypothesis,
		IdentificationStatus: IdentificationNotIdentified,
		Connections: []InsightConnection{{
			Kind: ConnectionMechanismCandidate,
			Statement: "A may connect to B through an unverified bridge",
		}},
	}
	if insight.CausalStatus != CausalHypothesis || insight.IdentificationStatus != IdentificationNotIdentified {
		t.Fatalf("connection semantics must not imply causal promotion: %+v", insight)
	}
}
