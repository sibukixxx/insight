package publicengine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestPublicContractAcceptsGenericEvidenceSource(t *testing.T) {
	e := newTestEngine(t)
	subjectID := createTestSubject(t, e)
	_, body, err := e.AddEvidence(context.Background(), subjectID, AddEvidenceRequest{
		ContractVersion: "1", IdempotencyKey: "generic-source",
		Documents: []EvidenceDocument{{
			ExternalRef: "paper-1", Source: "paper", Title: "Study",
			Content: "The measured outcome changed after the recorded event.",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var receipt EvidenceReceipt
	if err := json.Unmarshal(body, &receipt); err != nil {
		t.Fatal(err)
	}
	if len(receipt.Documents) != 1 || receipt.Documents[0].Status != "CREATED" {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
}

func TestPublicAnalysisRejectsOverlongResearchQuestion(t *testing.T) {
	e := newTestEngine(t)
	subjectID := createTestSubject(t, e)
	_, _, err := e.StartAnalysis(context.Background(), subjectID, StartAnalysisRequest{
		ContractVersion: "1", IdempotencyKey: "long-analysis-question",
		ResearchQuestion: strings.Repeat("q", maxQuestionLength+1),
	})
	if got := AsError(err); got.Code != CodeInvalidRequest {
		t.Fatalf("code = %s (%s), want %s", got.Code, got.Message, CodeInvalidRequest)
	}
}
