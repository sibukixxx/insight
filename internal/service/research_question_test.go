package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/llm"
)

func TestResearchFocusInstructionIsOptionalAndQuestionConditioned(t *testing.T) {
	if got := researchFocusInstruction("   "); got != "" {
		t.Fatalf("empty question must not change discovery prompt: %q", got)
	}
	got := researchFocusInstruction("Why did the observed rate change after 2025?")
	for _, want := range []string{"Why did the observed rate change after 2025?", "Do not assume its premise is true", "falsify"} {
		if !strings.Contains(got, want) {
			t.Fatalf("research focus instruction missing %q: %s", want, got)
		}
	}
}

func TestResearchQuestionChangesInputFingerprintNotDocumentIdentity(t *testing.T) {
	docs := []*domain.Document{{
		ID: "d1", ProjectID: "p1", Source: domain.SourceDataset,
		Content: "The recorded outcome was 42.", Metadata: map[string]string{"period": "2025"},
	}}
	at := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	a := BuildInputSnapshotForQuestion(docs, "What changed?", at)
	b := BuildInputSnapshotForQuestion(docs, "Why did it change?", at)
	c := BuildInputSnapshotForQuestion(docs, "  What changed?  ", at)

	if a.DocumentSetHash != b.DocumentSetHash {
		t.Fatalf("question must not change evidence identity: %s vs %s", a.DocumentSetHash, b.DocumentSetHash)
	}
	if a.InputFingerprint == b.InputFingerprint {
		t.Fatal("same evidence under different research questions must have different input fingerprints")
	}
	if a.InputFingerprint != c.InputFingerprint {
		t.Fatal("research question whitespace must be normalized before fingerprinting")
	}
	if a.ResearchQuestion != "What changed?" {
		t.Fatalf("question not normalized in snapshot: %q", a.ResearchQuestion)
	}
}

func TestCorePromptDoesNotAssumeCustomerOrCommercialDomain(t *testing.T) {
	core := strings.ToLower(basePrompt + observationExtractionPrompt + traceDetectionPrompt + patternDetectionPrompt + hypothesisPrompt + evidenceRetrievalPrompt + insightWriteupPrompt)
	for _, forbidden := range []string{
		"you are a customer-research analyst",
		"unspoken needs that drive behavior",
		"product-improvement direction",
		"concrete new product or service",
	} {
		if strings.Contains(core, forbidden) {
			t.Fatalf("core prompt still contains customer/commercial assumption %q", forbidden)
		}
	}
	for _, required := range []string{"evidence-grounded research analyst", "alternative explanations", "missing evidence", "falsification"} {
		if !strings.Contains(core, required) {
			t.Fatalf("generic research invariant missing from prompts: %q", required)
		}
	}
}

func TestGenericQualitySkipsLegacyNeedVocabularyWithoutStatedNeed(t *testing.T) {
	flags := AssessQuality(QualityInput{
		StatedNeed: "", LatentNeed: "efficiency",
		Expectation: "baseline remains stable", SurprisingFact: "the observed value changed",
		Patterns: []*domain.Pattern{{Kind: domain.PatternDeviation}},
	})
	for _, flag := range flags {
		if flag.Code == domain.QualityGenericTerm || flag.Code == domain.QualityStatedNeedEcho {
			t.Fatalf("generic hypothesis was judged with legacy customer-needs rule: %+v", flags)
		}
	}
}


func TestRunComparisonExplainsResearchQuestionInputChange(t *testing.T) {
	at := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	docs := []*domain.Document{{ID: "d1", ProjectID: "p1", Source: domain.SourceDocument, Content: "Same evidence."}}
	aSnap := BuildInputSnapshotForQuestion(docs, "What changed?", at)
	bSnap := BuildInputSnapshotForQuestion(docs, "Why did it change?", at)
	aJSON, _ := json.Marshal(aSnap)
	bJSON, _ := json.Marshal(bSnap)

	from := &domain.Analysis{ID: "a", Status: domain.AnalysisCompleted, InputSnapshot: string(aJSON), InputFingerprint: aSnap.InputFingerprint, ExecutionSnapshot: "{}", ExecutionFingerprint: "same", CreatedAt: at}
	to := &domain.Analysis{ID: "b", Status: domain.AnalysisCompleted, InputSnapshot: string(bJSON), InputFingerprint: bSnap.InputFingerprint, ExecutionSnapshot: "{}", ExecutionFingerprint: "same", CreatedAt: at}

	got := CompareAnalysisRuns(RunComparisonInput{Analysis: from}, RunComparisonInput{Analysis: to}, []*domain.Analysis{from, to})
	if got.Input.State != AxisChanged || got.Input.ResearchQuestion == nil {
		t.Fatalf("question change not exposed as input diff: %+v", got.Input)
	}
	if got.Input.ResearchQuestion.From != "What changed?" || got.Input.ResearchQuestion.To != "Why did it change?" {
		t.Fatalf("unexpected question diff: %+v", got.Input.ResearchQuestion)
	}
	if got.Attribution != AttributionInputChange {
		t.Fatalf("attribution = %s, want %s", got.Attribution, AttributionInputChange)
	}
}


type promptCaptureClient struct {
	prompts []string
}

func (c *promptCaptureClient) Generate(_ context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	c.prompts = append(c.prompts, req.SystemPrompt)
	return &llm.GenerateResponse{Content: json.RawMessage(`{}`)}, nil
}

func TestEverySemanticLLMStageReceivesResearchQuestion(t *testing.T) {
	capture := &promptCaptureClient{}
	p := &Pipeline{LLM: capture, ResearchQuestion: "Which explanation best accounts for the recorded change?"}
	for _, step := range pipelineLLMSteps() {
		if _, err := p.generate(context.Background(), step, nil); err != nil {
			t.Fatalf("stage %s: %v", step.Name, err)
		}
	}
	if len(capture.prompts) != len(pipelineLLMSteps()) {
		t.Fatalf("captured %d prompts, want %d", len(capture.prompts), len(pipelineLLMSteps()))
	}
	for i, prompt := range capture.prompts {
		if !strings.Contains(prompt, p.ResearchQuestion) || !strings.Contains(prompt, "Do not assume its premise is true") {
			t.Fatalf("stage %s did not receive guarded research focus: %s", pipelineLLMSteps()[i].Name, prompt)
		}
	}
}
