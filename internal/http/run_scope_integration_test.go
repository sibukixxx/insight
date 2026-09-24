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
	"insight-lab/internal/repository/sqlite"
	"insight-lab/internal/usecase"
)

// newRunScopeRouter serves project p1 with two completed runs (a1, a2) and
// a newer failed run (a3), plus an unrelated project p2.
func newRunScopeRouter(t *testing.T) http.Handler {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "runs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	projects, analyses := sqlite.NewProjectRepository(db), sqlite.NewAnalysisRepository(db)
	insights, patterns := sqlite.NewInsightRepository(db), sqlite.NewPatternRepository(db)
	base := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	for _, id := range []string{"p1", "p2"} {
		if err := projects.Create(ctx, &domain.Project{ID: id, Name: id, CreatedAt: base}); err != nil {
			t.Fatal(err)
		}
	}
	for i, id := range []string{"a1", "a2"} {
		at := base.Add(time.Duration(i) * time.Hour)
		metrics := `{"provenance":{"mode":"deterministic","ruleVersion":"dataset-preanalysis/v1"},"patternCount":1}`
		if err := analyses.Create(ctx, &domain.Analysis{ID: id, ProjectID: "p1", Status: domain.AnalysisCompleted, Metrics: metrics, StartedAt: &at, FinishedAt: &at, CreatedAt: at}); err != nil {
			t.Fatal(err)
		}
		analysisID := id
		if err := insights.Create(ctx, &domain.Insight{ID: "ins_" + id, ProjectID: "p1", AnalysisID: &analysisID, Title: "Insight " + id, CreatedAt: at}); err != nil {
			t.Fatal(err)
		}
		if err := patterns.CreateBatch(ctx, []*domain.Pattern{{ID: "pat_" + id, ProjectID: "p1", AnalysisID: id, Title: "Pattern " + id, CreatedAt: at}}); err != nil {
			t.Fatal(err)
		}
	}
	failedAt := base.Add(5 * time.Hour)
	if err := analyses.Create(ctx, &domain.Analysis{ID: "a3", ProjectID: "p1", Status: domain.AnalysisFailed, Error: "boom", FinishedAt: &failedAt, CreatedAt: failedAt}); err != nil {
		t.Fatal(err)
	}
	app := usecase.New(usecase.Repositories{
		Projects: projects, Documents: sqlite.NewDocumentRepository(db), Observations: sqlite.NewObservationRepository(db),
		Patterns: patterns, Analyses: analyses, Insights: insights, Evidence: sqlite.NewEvidenceRepository(db), Research: sqlite.NewResearchRepository(db),
	})
	return httpapi.NewRouter(httpapi.Deps{App: app})
}

func get(t *testing.T, router http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestRunScopedEndpointsReturnOnlyTheSelectedRun(t *testing.T) {
	router := newRunScopeRouter(t)
	cases := []struct{ path, wantID string }{
		{"/api/projects/p1/insights", "ins_a2"},
		{"/api/projects/p1/insights?analysisId=a1", "ins_a1"},
		{"/api/projects/p1/patterns", "pat_a2"},
		{"/api/projects/p1/patterns?analysisId=a1", "pat_a1"},
	}
	for _, c := range cases {
		rec := get(t, router, c.path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", c.path, rec.Code, rec.Body.String())
		}
		var items []struct {
			ID         string `json:"id"`
			AnalysisID string `json:"analysisId"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || items[0].ID != c.wantID {
			t.Fatalf("%s returned %+v, want only %s", c.path, items, c.wantID)
		}
		if items[0].AnalysisID == "" {
			t.Fatalf("%s: item does not carry analysisId", c.path)
		}
	}
}

func TestRunScopedEndpointsRejectAnotherProjectsRun(t *testing.T) {
	router := newRunScopeRouter(t)
	for _, path := range []string{
		"/api/projects/p2/insights?analysisId=a1",
		"/api/projects/p2/patterns?analysisId=a1",
		"/api/projects/p2/evaluation?analysisId=a1",
		"/api/projects/p2/report.md?analysisId=a1",
	} {
		if rec := get(t, router, path); rec.Code != http.StatusNotFound {
			t.Fatalf("%s: status %d, want 404", path, rec.Code)
		}
	}
}

func TestEvaluationUsesLatestCompletedRunAndRejectsASelectedFailedRun(t *testing.T) {
	router := newRunScopeRouter(t)
	if rec := get(t, router, "/api/projects/p1/evaluation"); rec.Code != http.StatusOK {
		t.Fatalf("evaluation with a newer failed run: %d %s", rec.Code, rec.Body.String())
	}
	if rec := get(t, router, "/api/projects/p1/evaluation?analysisId=a3"); rec.Code != http.StatusConflict {
		t.Fatalf("evaluation of a failed run: %d, want 409", rec.Code)
	}
}

func TestProjectReportHonoursTheSelectedRun(t *testing.T) {
	router := newRunScopeRouter(t)
	rec := get(t, router, "/api/projects/p1/report.md?analysisId=a1")
	if rec.Code != http.StatusOK {
		t.Fatalf("report: %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Analysis ID: `a1`") || !strings.Contains(body, "Insight a1") || strings.Contains(body, "Insight a2") {
		t.Fatalf("report is not bound to a1:\n%s", body)
	}
}

func TestAnalysesListIncludesMetricsOnlyForCompletedRuns(t *testing.T) {
	router := newRunScopeRouter(t)
	rec := get(t, router, "/api/projects/p1/analyses")
	if rec.Code != http.StatusOK {
		t.Fatalf("analyses: %d", rec.Code)
	}
	var runs []struct {
		ID      string          `json:"id"`
		Status  string          `json:"status"`
		Metrics json.RawMessage `json:"metrics"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &runs); err != nil {
		t.Fatal(err)
	}
	if len(runs) != 3 {
		t.Fatalf("expected all 3 runs to be listed and none deleted, got %d", len(runs))
	}
	for _, run := range runs {
		hasMetrics := len(run.Metrics) > 0
		if (run.Status == "completed") != hasMetrics {
			t.Fatalf("run %s (%s) metrics present = %v", run.ID, run.Status, hasMetrics)
		}
	}
}
