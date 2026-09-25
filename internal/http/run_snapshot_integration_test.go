package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/buildinfo"
	"insight-lab/internal/domain"
	httpapi "insight-lab/internal/http"
	"insight-lab/internal/http/handler"
	"insight-lab/internal/repository/sqlite"
	"insight-lab/internal/service"
	"insight-lab/internal/usecase"
)

func TestHealthReportsEngineBuildIdentity(t *testing.T) {
	router := httpapi.NewRouter(httpapi.Deps{Build: handler.BuildInfo{Engine: buildinfo.Info{Version: "v0.9.0", Commit: "abc123", Dirty: "false"}}})
	rec := get(t, router, "/api/health")
	var body struct {
		Engine buildinfo.Info `json:"engine"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if want := (buildinfo.Info{Version: "v0.9.0", Commit: "abc123", Dirty: "false"}); body.Engine != want {
		t.Fatalf("engine = %+v, want %+v", body.Engine, want)
	}
}

func TestLegacyRunsExposeNoSnapshotsInsteadOfEmptyOnes(t *testing.T) {
	router := newRunScopeRouter(t)
	rec := get(t, router, "/api/projects/p1/analyses")
	for _, key := range []string{"executionSnapshot", "inputSnapshot", "executionFingerprint", "inputFingerprint"} {
		if strings.Contains(rec.Body.String(), key) {
			t.Fatalf("a legacy run must omit %s, got %s", key, rec.Body.String())
		}
	}
}

// newJobRouter serves project p1 through a real JobManager whose settings
// carry an API key, so the enqueue path can be checked end to end.
func newJobRouter(t *testing.T) (http.Handler, *sqlite.AnalysisRepository) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	projects, analyses := sqlite.NewProjectRepository(db), sqlite.NewAnalysisRepository(db)
	if err := projects.Create(ctx, &domain.Project{ID: "p1", Name: "p1", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	documents := sqlite.NewDocumentRepository(db)
	pipeline := &service.Pipeline{Documents: documents, Observations: sqlite.NewObservationRepository(db), Patterns: sqlite.NewPatternRepository(db), Insights: sqlite.NewInsightRepository(db), Evidence: sqlite.NewEvidenceRepository(db)}
	settings := service.NewSettingsStore(service.Settings{BaseURL: "https://api.example.com/v1?key=q", Model: "model-a", APIKey: "sk-http-secret"})
	jobs := service.NewJobManager(analyses, pipeline, settings, service.DefaultLLMClientFactory)
	app := usecase.New(usecase.Repositories{Projects: projects, Documents: documents, Observations: pipeline.Observations, Patterns: pipeline.Patterns, Analyses: analyses, Insights: pipeline.Insights, Evidence: pipeline.Evidence, Research: sqlite.NewResearchRepository(db)})
	return httpapi.NewRouter(httpapi.Deps{App: app, JobManager: jobs, Settings: settings}), analyses
}

func post(router http.Handler, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("content-type", "application/json")
	router.ServeHTTP(rec, req)
	return rec
}

func TestCreateAnalysisRecordsLabelNoteAndExecutionSnapshotWithoutSecrets(t *testing.T) {
	router, _ := newJobRouter(t)
	rec := post(router, "/api/projects/p1/analysis", `{"label":" baseline ","note":"first","semanticAnalysisMode":"DISCOVERY"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create analysis: %d %s", rec.Code, rec.Body.String())
	}
	var run struct {
		Label, Note, SemanticAnalysisMode, ExecutionFingerprint string
		ExecutionSnapshot                                       json.RawMessage
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil {
		t.Fatal(err)
	}
	if run.Label != "baseline" || run.Note != "first" || run.SemanticAnalysisMode != "DISCOVERY" || run.ExecutionFingerprint == "" || len(run.ExecutionSnapshot) == 0 {
		t.Fatalf("created run = %+v", run)
	}
	for _, secret := range []string{"sk-http-secret", "key=q", "https://api.example.com"} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Fatalf("response leaks %q: %s", secret, rec.Body.String())
		}
	}
}

func TestCreateAnalysisRejectsAnUnknownSemanticAnalysisMode(t *testing.T) {
	router, analyses := newJobRouter(t)
	if rec := post(router, "/api/projects/p1/analysis", `{"semanticAnalysisMode":"GUESS"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
	if list, _ := analyses.ListByProject(context.Background(), "p1"); len(list) != 0 {
		t.Fatalf("a rejected request must not create a run, got %d", len(list))
	}
}

func TestCreateAnalysisAcceptsAnEmptyBody(t *testing.T) {
	router, _ := newJobRouter(t)
	if rec := post(router, "/api/projects/p1/analysis", ""); rec.Code != http.StatusAccepted {
		t.Fatalf("status %d, want 202: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateAnalysisRecordsAnExplicitReasoningProfileAndRejectsUnknownOnes(t *testing.T) {
	router, analyses := newJobRouter(t)
	if rec := post(router, "/api/projects/p1/analysis", `{"reasoningProfile":"MARKETING"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown profile: status %d, want 400", rec.Code)
	}
	if list, _ := analyses.ListByProject(context.Background(), "p1"); len(list) != 0 {
		t.Fatalf("a rejected request must not create a run, got %d", len(list))
	}
	rec := post(router, "/api/projects/p1/analysis", `{"reasoningProfile":"CUSTOMER_INSIGHT"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create analysis: %d %s", rec.Code, rec.Body.String())
	}
	var run struct{ ReasoningProfile string }
	if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil || run.ReasoningProfile != "CUSTOMER_INSIGHT" {
		t.Fatalf("created run profile = %q (%v)", run.ReasoningProfile, err)
	}
}
