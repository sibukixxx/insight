package service

import (
	"context"
	"testing"

	"insight-lab/internal/domain"
	"insight-lab/internal/llm"
	"insight-lab/internal/repository/sqlite"
)

// meteredLLM reports a fixed token usage for every call it forwards.
type meteredLLM struct{ inner *fakeLLM }

func (m meteredLLM) Generate(ctx context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	resp, err := m.inner.Generate(ctx, req)
	if resp != nil {
		resp.Usage = llm.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}
	}
	return resp, err
}

func TestPipelineRunAggregatesTokenUsageAcrossModelCalls(t *testing.T) {
	fake := newFakeLLM()
	pipeline, _, project := newTestPipelineWith(t, meteredLLM{fake}, interviewTestDocuments("proj_1"))
	metrics, err := pipeline.Run(context.Background(), testAnalysisID, project.ID, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	calls := 0
	for _, n := range fake.calls {
		calls += n
	}
	want := LLMUsage{Calls: calls, PromptTokens: 10 * calls, CompletionTokens: 5 * calls, TotalTokens: 15 * calls}
	if metrics.Usage == nil || *metrics.Usage != want {
		t.Fatalf("usage = %+v, want %+v", metrics.Usage, want)
	}
}

func TestDeterministicRunRecordsNoModelUsage(t *testing.T) {
	docs := []*domain.Document{
		datasetDoc("doc_jan", "2026-01", "サンプル市", "ASSIGNED", "2", nil),
		datasetDoc("doc_feb", "2026-02", "サンプル市", "ASSIGNED", "5", nil),
	}
	pipeline, _, project := newTestPipelineWith(t, nil, docs)
	metrics, err := pipeline.Run(context.Background(), testAnalysisID, project.ID, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if metrics.Usage != nil {
		t.Fatalf("a run without a model must not report usage: %+v", metrics.Usage)
	}
}

func TestPipelineRunTagsEveryObservationWithItsAnalysis(t *testing.T) {
	pipeline, db, project := newTestPipeline(t)
	if _, err := pipeline.Run(context.Background(), testAnalysisID, project.ID, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	observations, err := sqlite.NewObservationRepository(db).ListByProject(context.Background(), project.ID)
	if err != nil || len(observations) == 0 {
		t.Fatalf("observations = %v, %v", observations, err)
	}
	for _, o := range observations {
		if o.AnalysisID != testAnalysisID {
			t.Fatalf("observation %s analysisId = %q, want %q", o.ID, o.AnalysisID, testAnalysisID)
		}
	}
}
