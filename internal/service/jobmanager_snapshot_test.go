package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/llm"
	"insight-lab/internal/repository/sqlite"
)

type jobHarness struct {
	jobs      *JobManager
	analyses  *sqlite.AnalysisRepository
	documents *sqlite.DocumentRepository
	settings  *SettingsStore
	clients   []Settings
}

// newJobHarness wires a JobManager to a fresh database holding project
// proj_1 with docs. Workers are not started, so tests can change settings
// between enqueue and run.
func newJobHarness(t *testing.T, docs []*domain.Document, settings Settings) *jobHarness {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	if err := sqlite.NewProjectRepository(db).Create(ctx, &domain.Project{ID: "proj_1", Name: "jobs", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	h := &jobHarness{analyses: sqlite.NewAnalysisRepository(db), documents: sqlite.NewDocumentRepository(db), settings: NewSettingsStore(settings)}
	for _, d := range docs {
		d.ProjectID = "proj_1"
	}
	if err := h.documents.CreateBatch(ctx, docs); err != nil {
		t.Fatal(err)
	}
	pipeline := &Pipeline{
		Documents: h.documents, Observations: sqlite.NewObservationRepository(db), Patterns: sqlite.NewPatternRepository(db),
		Insights: sqlite.NewInsightRepository(db), Evidence: sqlite.NewEvidenceRepository(db),
	}
	h.jobs = NewJobManager(h.analyses, pipeline, h.settings, func(s Settings) llm.Client {
		h.clients = append(h.clients, s)
		return newFakeLLM()
	})
	h.jobs.build = testBuild
	return h
}

func (h *jobHarness) runAll(t *testing.T, ids ...string) []*domain.Analysis {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.jobs.Start(ctx, 1)
	out := make([]*domain.Analysis, 0, len(ids))
	for _, id := range ids {
		deadline := time.Now().Add(10 * time.Second)
		for {
			a, err := h.analyses.Get(context.Background(), id)
			if err != nil {
				t.Fatal(err)
			}
			if a.Status == domain.AnalysisCompleted || a.Status == domain.AnalysisFailed {
				if a.Status == domain.AnalysisFailed {
					t.Fatalf("analysis %s failed: %s", id, a.Error)
				}
				out = append(out, a)
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("analysis %s did not finish", id)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	return out
}

func (h *jobHarness) enqueue(t *testing.T, req EnqueueRequest) string {
	t.Helper()
	req.ProjectID = "proj_1"
	a, err := h.jobs.Enqueue(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	return a.ID
}

func datasetJobDocs() []*domain.Document {
	return []*domain.Document{
		datasetDoc("doc_jan", "2026-01", "サンプル市", "ASSIGNED", "2", nil),
		datasetDoc("doc_feb", "2026-02", "サンプル市", "ASSIGNED", "5", nil),
	}
}

func TestRunsWithTheSameBuildSettingsAndDocumentsShareBothFingerprints(t *testing.T) {
	h := newJobHarness(t, datasetJobDocs(), Settings{})
	first := h.enqueue(t, EnqueueRequest{Label: "baseline", Note: "first pass"})
	second := h.enqueue(t, EnqueueRequest{})
	runs := h.runAll(t, first, second)

	if runs[0].ExecutionFingerprint == "" || runs[0].InputFingerprint == "" {
		t.Fatalf("fingerprints were not recorded: %+v", runs[0])
	}
	if runs[0].ExecutionFingerprint != runs[1].ExecutionFingerprint || runs[0].InputFingerprint != runs[1].InputFingerprint {
		t.Fatalf("identical runs must share fingerprints: %+v vs %+v", runs[0], runs[1])
	}
	if runs[0].Label != "baseline" || runs[0].Note != "first pass" {
		t.Fatalf("label/note = %q/%q", runs[0].Label, runs[0].Note)
	}
	var execution ExecutionSnapshot
	if err := json.Unmarshal([]byte(runs[0].ExecutionSnapshot), &execution); err != nil {
		t.Fatal(err)
	}
	if execution.EngineVersion != testBuild.Version || execution.ExecutionMode != ExecutionModeDeterministic || execution.ExecutionFingerprint != runs[0].ExecutionFingerprint {
		t.Fatalf("execution snapshot = %+v", execution)
	}
}

func TestAddingADocumentChangesOnlyTheInputFingerprint(t *testing.T) {
	h := newJobHarness(t, datasetJobDocs(), Settings{})
	before := h.enqueue(t, EnqueueRequest{})
	runsBefore := h.runAll(t, before)

	if err := h.documents.Create(context.Background(), datasetDoc("doc_mar", "2026-03", "サンプル市", "ASSIGNED", "7", nil)); err != nil {
		t.Fatal(err)
	}
	after := h.enqueue(t, EnqueueRequest{})
	runsAfter := h.runAll(t, after)

	if runsBefore[0].InputFingerprint == runsAfter[0].InputFingerprint {
		t.Fatal("adding a document must change the input fingerprint")
	}
	if runsBefore[0].ExecutionFingerprint != runsAfter[0].ExecutionFingerprint {
		t.Fatal("adding a document must not change the execution fingerprint")
	}
}

func TestRunExecutesWithEnqueueTimeSettingsAndRecordsALaterChange(t *testing.T) {
	enqueued := Settings{BaseURL: "https://api.example.com/v1", Model: "model-a", APIKey: "sk-test-secret"}
	h := newJobHarness(t, interviewTestDocuments("proj_1"), enqueued)
	id := h.enqueue(t, EnqueueRequest{SemanticAnalysisMode: domain.AnalysisModeDiscovery})
	h.settings.Update(Settings{Model: "model-b"})
	run := h.runAll(t, id)[0]

	if len(h.clients) != 1 || h.clients[0].Model != "model-a" {
		t.Fatalf("run used settings %+v, want the enqueue-time model-a", h.clients)
	}
	var execution ExecutionSnapshot
	if err := json.Unmarshal([]byte(run.ExecutionSnapshot), &execution); err != nil {
		t.Fatal(err)
	}
	if execution.LLM == nil || execution.LLM.Models[0].Model != "model-a" || !execution.SettingsChangedBeforeStart {
		t.Fatalf("execution snapshot = %+v", execution)
	}
	if run.SemanticAnalysisMode != domain.AnalysisModeDiscovery || execution.SemanticAnalysisMode != domain.AnalysisModeDiscovery {
		t.Fatalf("semantic mode was not recorded: %q / %q", run.SemanticAnalysisMode, execution.SemanticAnalysisMode)
	}
	for _, field := range []string{run.ExecutionSnapshot, run.InputSnapshot, run.Metrics} {
		if strings.Contains(field, "sk-test-secret") || strings.Contains(field, "https://api.example.com") {
			t.Fatalf("persisted run leaks a credential or the full provider URL: %s", field)
		}
	}
}

func TestEnqueueRejectsAnUnknownSemanticAnalysisMode(t *testing.T) {
	h := newJobHarness(t, datasetJobDocs(), Settings{})
	if _, err := h.jobs.Enqueue(context.Background(), EnqueueRequest{ProjectID: "proj_1", SemanticAnalysisMode: "GUESS"}); err == nil {
		t.Fatal("an unknown semantic analysis mode must be rejected")
	}
}
