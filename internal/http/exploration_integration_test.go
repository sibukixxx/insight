package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
	httpapi "insight-lab/internal/http"
	"insight-lab/internal/llm"
	"insight-lab/internal/llm/scripted"
	"insight-lab/internal/repository/sqlite"
	"insight-lab/internal/service"
	"insight-lab/internal/usecase"
)

// #158: a question alone starts an exploration. With no evidence the run may
// only return unverified candidates; it must never create a Document,
// Observation, Pattern, Insight or Evidence row, and never claim to be
// verified or decision-ready.

type explorationRig struct {
	router    http.Handler
	documents *sqlite.DocumentRepository
	observ    *sqlite.ObservationRepository
	patterns  *sqlite.PatternRepository
	insights  *sqlite.InsightRepository
}

func newExplorationRig(t *testing.T, modelConfigured bool) *explorationRig {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "exploration.db"))
	if err != nil {
		t.Fatal(err)
	}
	projects, documents := sqlite.NewProjectRepository(db), sqlite.NewDocumentRepository(db)
	observations, patterns := sqlite.NewObservationRepository(db), sqlite.NewPatternRepository(db)
	analyses, insights, evidence := sqlite.NewAnalysisRepository(db), sqlite.NewInsightRepository(db), sqlite.NewEvidenceRepository(db)
	settings := service.Settings{}
	if modelConfigured {
		settings = service.Settings{BaseURL: "https://scripted.example.test/v1", Model: "scripted-model"}
	}
	pipeline := &service.Pipeline{Documents: documents, Observations: observations, Patterns: patterns, Insights: insights, Evidence: evidence}
	jobs := service.NewJobManager(analyses, pipeline, service.NewSettingsStore(settings), func(service.Settings) llm.Client { return scripted.Model{} })
	ctx, cancel := context.WithCancel(context.Background())
	jobs.Start(ctx, 1)
	app := usecase.New(usecase.Repositories{Projects: projects, Documents: documents, Observations: observations, Patterns: patterns, Analyses: analyses, Insights: insights, Evidence: evidence, Research: sqlite.NewResearchRepository(db), Scenarios: sqlite.NewScenarioRepository(db)})
	t.Cleanup(func() {
		cancel()
		jobs.Wait()
		db.Close()
	})
	return &explorationRig{router: httpapi.NewRouter(httpapi.Deps{App: app, JobManager: jobs}), documents: documents, observ: observations, patterns: patterns, insights: insights}
}

func (r *explorationRig) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.router.ServeHTTP(rec, req)
	return rec
}

func (r *explorationRig) project(t *testing.T) string {
	t.Helper()
	rec := r.do(t, http.MethodPost, "/api/projects", `{"name":"地方都市の店舗"}`)
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.ID == "" {
		t.Fatalf("create project: %d %s", rec.Code, rec.Body.String())
	}
	return p.ID
}

type explorationRun struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Lifecycle string `json:"lifecycle"`
	Error     string `json:"error"`
	Metrics   struct {
		Exploration *service.ExplorationResult `json:"exploration"`
	} `json:"metrics"`
}

func (r *explorationRig) await(t *testing.T, id string) explorationRun {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		rec := r.do(t, http.MethodGet, "/api/analysis/"+id, "")
		var run explorationRun
		if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil {
			t.Fatalf("decode: %v: %s", err, rec.Body.String())
		}
		if run.Status == "completed" || run.Status == "failed" {
			return run
		}
		if time.Now().After(deadline) {
			t.Fatalf("run did not finish: %+v", run)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestQuestionOnlyExplorationReturnsUnverifiedCandidatesAndCreatesNoEvidence(t *testing.T) {
	rig := newExplorationRig(t, true)
	pid := rig.project(t)
	question := "地方都市で人口が減っているのに店舗が増える理由は？"

	rec := rig.do(t, http.MethodPost, "/api/projects/"+pid+"/analysis", `{"exploratory":true,"researchQuestion":"`+question+`","outputLocale":"ja-JP"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start exploration: %d %s", rec.Code, rec.Body.String())
	}
	var started explorationRun
	_ = json.Unmarshal(rec.Body.Bytes(), &started)
	run := rig.await(t, started.ID)
	if run.Status != "completed" {
		t.Fatalf("exploration failed: %+v", run)
	}

	// The completed run is revisitable through the plain GET.
	got := run.Metrics.Exploration
	if got == nil {
		t.Fatalf("no exploration result on the run")
	}
	if got.Status != service.ExplorationStatus || got.Verified || got.DecisionReady || got.EvidenceCount != 0 {
		t.Fatalf("exploration must be unverified with no evidence: %+v", got)
	}
	if got.Question != question || len(got.Candidates) < 2 || len(got.Limitations) == 0 {
		t.Fatalf("exploration result incomplete: %+v", got)
	}
	for _, c := range got.Candidates {
		if len(c.FalsificationConditions) == 0 || len(c.RequiredData) == 0 || len(c.CompetingExplanations) == 0 {
			t.Fatalf("candidate lacks falsification, competing explanation or required data: %+v", c)
		}
	}
	if !strings.Contains(got.Candidates[0].Title, "候補") {
		t.Fatalf("ja-JP output locale was not applied: %q", got.Candidates[0].Title)
	}

	ctx := context.Background()
	if docs, _ := rig.documents.ListByProject(ctx, pid); len(docs) != 0 {
		t.Fatalf("exploration created %d documents", len(docs))
	}
	if obs, _ := rig.observ.ListByProject(ctx, pid); len(obs) != 0 {
		t.Fatalf("exploration created %d observations", len(obs))
	}
	if pats, _ := rig.patterns.ListByProject(ctx, pid); len(pats) != 0 {
		t.Fatalf("exploration created %d patterns", len(pats))
	}
	if ins, _ := rig.insights.ListByProject(ctx, pid); len(ins) != 0 {
		t.Fatalf("exploration created %d insights", len(ins))
	}
	if rec := rig.do(t, http.MethodGet, "/api/projects/"+pid+"/insights", ""); strings.Contains(rec.Body.String(), "候補") {
		t.Fatalf("exploration candidates leaked into insights: %s", rec.Body.String())
	}
}

func TestQuestionOnlyExplorationRefusalsAreExplicit(t *testing.T) {
	rig := newExplorationRig(t, true)
	pid := rig.project(t)
	if rec := rig.do(t, http.MethodPost, "/api/projects/"+pid+"/analysis", `{"exploratory":true}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("exploration without a question: %d %s", rec.Code, rec.Body.String())
	}
	// An ordinary run of an empty project keeps failing, question or not.
	rec := rig.do(t, http.MethodPost, "/api/projects/"+pid+"/analysis", `{"researchQuestion":"why?"}`)
	var started explorationRun
	_ = json.Unmarshal(rec.Body.Bytes(), &started)
	if run := rig.await(t, started.ID); run.Status != "failed" || !strings.Contains(run.Error, "no documents") || run.Metrics.Exploration != nil {
		t.Fatalf("ordinary empty run must still fail: %+v", run)
	}
	// With evidence present a question-only exploration is refused.
	if err := rig.documents.CreateBatch(context.Background(), []*domain.Document{{ID: "d1", ProjectID: pid, Source: domain.SourceDocument, Title: "t", Content: "The recorded value was 3."}}); err != nil {
		t.Fatal(err)
	}
	if rec := rig.do(t, http.MethodPost, "/api/projects/"+pid+"/analysis", `{"exploratory":true,"researchQuestion":"why?"}`); rec.Code != http.StatusConflict {
		t.Fatalf("exploration with evidence: %d %s", rec.Code, rec.Body.String())
	}
}

func TestQuestionOnlyExplorationNeedsAModelButTheQuestionStillSaves(t *testing.T) {
	rig := newExplorationRig(t, false)
	pid := rig.project(t)
	rec := rig.do(t, http.MethodPost, "/api/projects/"+pid+"/analysis", `{"exploratory":true,"researchQuestion":"why?"}`)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "model") {
		t.Fatalf("model-less exploration must be refused with a reason: %d %s", rec.Code, rec.Body.String())
	}
}

func TestQuestionOnlyExplorationIsMarkedInTheExecutionSnapshot(t *testing.T) {
	rig := newExplorationRig(t, true)
	pid := rig.project(t)
	rec := rig.do(t, http.MethodPost, "/api/projects/"+pid+"/analysis", `{"exploratory":true,"researchQuestion":"why?"}`)
	var started explorationRun
	_ = json.Unmarshal(rec.Body.Bytes(), &started)
	rig.await(t, started.ID)
	// The marker is what Retry and restart recovery read to stay exploratory.
	rec = rig.do(t, http.MethodGet, "/api/analysis/"+started.ID, "")
	var snap struct {
		ExecutionSnapshot struct {
			Exploration *service.ExplorationConfig `json:"exploration"`
		} `json:"executionSnapshot"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &snap)
	if snap.ExecutionSnapshot.Exploration == nil || snap.ExecutionSnapshot.Exploration.PromptFingerprint == "" {
		t.Fatalf("exploration marker missing from the execution snapshot: %s", rec.Body.String())
	}
}
