package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
	httpapi "insight-lab/internal/http"
	"insight-lab/internal/repository/sqlite"
	"insight-lab/internal/service"
	"insight-lab/internal/usecase"
)

// Boundary checks from the acceptance matrix (#156,
// docs/testing/acceptance-matrix.md). They drive the real Reference router
// with files already in the repository as the oracle, so they run in both
// the delivery and the demo build without embedding any sample data.

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// examples/demo-ja/README.md promises that the numeric raw-aggregation
// example is accepted neither as a Documents CSV nor as an analysis CSV.
// Every import entry point must refuse it and store nothing.
func TestImportHTTPRefusesNumericRawAggregationCSVForEveryKindAndStoresNothing(t *testing.T) {
	router := newImportTestRouter(t)
	numeric := readRepoFile(t, "examples/demo-ja/03-numeric-raw-aggregation.csv")

	for _, kind := range []string{"documents", "analysis"} {
		req := httptest.NewRequest(http.MethodPost, "/api/projects/p1/documents/import/preview?kind="+kind, strings.NewReader(numeric))
		req.Header.Set("Content-Type", "text/csv")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusBadRequest {
			t.Errorf("preview kind=%s: %d %s", kind, resp.Code, resp.Body.String())
		}
	}
	for _, target := range []string{"/api/projects/p1/documents/import", "/api/projects/p1/documents/import/analysis"} {
		body, contentType := multipartUpload(t, nil, "file", "03-numeric-raw-aggregation.csv", numeric)
		req := httptest.NewRequest(http.MethodPost, target, body)
		req.Header.Set("Content-Type", contentType)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusBadRequest {
			t.Errorf("POST %s: %d %s", target, resp.Code, resp.Body.String())
		}
	}
	if docs := listDocumentsHTTP(t, router); len(docs) != 0 {
		t.Fatalf("a refused numeric CSV stored %d documents", len(docs))
	}
}

// Without a configured model, the pinned official-population sample (a
// Documents CSV whose rows carry no countable dataset metadata) cannot be
// analyzed. The run must fail with guidance instead of reporting success,
// and it must not leave observations, patterns or insights behind.
func TestAnalysisHTTPFailsWithGuidanceAndStoresNoFindingsWhenNoModelIsConfigured(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "no_model.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	projects, documents, analyses := sqlite.NewProjectRepository(db), sqlite.NewDocumentRepository(db), sqlite.NewAnalysisRepository(db)
	observations, patterns := sqlite.NewObservationRepository(db), sqlite.NewPatternRepository(db)
	insights, evidence := sqlite.NewInsightRepository(db), sqlite.NewEvidenceRepository(db)
	if err := projects.Create(ctx, &domain.Project{ID: "p1", Name: "no model", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	pipeline := &service.Pipeline{Documents: documents, Observations: observations, Patterns: patterns, Insights: insights, Evidence: evidence}
	settings := service.NewSettingsStore(service.Settings{})
	jobs := service.NewJobManager(analyses, pipeline, settings, service.DefaultLLMClientFactory)
	runCtx, cancel := context.WithCancel(ctx)
	jobs.Start(runCtx, 1)
	t.Cleanup(func() {
		cancel()
		jobs.Wait()
		db.Close()
	})
	app := usecase.New(usecase.Repositories{Projects: projects, Documents: documents, Observations: observations, Patterns: patterns, Analyses: analyses, Insights: insights, Evidence: evidence, Research: sqlite.NewResearchRepository(db)})
	router := httpapi.NewRouter(httpapi.Deps{App: app, JobManager: jobs, Settings: settings})

	sample := readRepoFile(t, "internal/sampledata/scenarios/ja-official-population/input.csv")
	body, contentType := multipartUpload(t, nil, "file", "ja-official-population.csv", sample)
	req := httptest.NewRequest(http.MethodPost, "/api/projects/p1/documents/import", body)
	req.Header.Set("Content-Type", contentType)
	imp := httptest.NewRecorder()
	router.ServeHTTP(imp, req)
	var imported struct{ Imported, Skipped int }
	_ = json.Unmarshal(imp.Body.Bytes(), &imported)
	if imp.Code != http.StatusOK || imported.Imported != 8 || imported.Skipped != 0 {
		t.Fatalf("import: %d %s", imp.Code, imp.Body.String())
	}

	created := post(router, "/api/projects/p1/analysis", `{}`)
	if created.Code != http.StatusAccepted {
		t.Fatalf("create analysis: %d %s", created.Code, created.Body.String())
	}
	var run struct {
		ID, Status, Error, FailureCode string
		Metrics                        json.RawMessage
	}
	_ = json.Unmarshal(created.Body.Bytes(), &run)
	deadline := time.Now().Add(20 * time.Second)
	for run.Status != "completed" && run.Status != "failed" {
		if time.Now().After(deadline) {
			t.Fatalf("analysis did not finish: %+v", run)
		}
		time.Sleep(20 * time.Millisecond)
		if err := json.Unmarshal(get(t, router, "/api/analysis/"+run.ID).Body.Bytes(), &run); err != nil {
			t.Fatal(err)
		}
	}
	if run.Status != "failed" || run.FailureCode != string(domain.FailureError) {
		t.Fatalf("a run without a model and without countable data must fail, got %+v", run)
	}
	if !strings.Contains(run.Error, "not configured") || !strings.Contains(run.Error, "Settings") {
		t.Errorf("failure does not tell the user what to do: %q", run.Error)
	}
	if len(run.Metrics) != 0 {
		t.Errorf("a failed run must not carry result metrics: %s", run.Metrics)
	}
	for _, path := range []string{"/api/projects/p1/insights", "/api/projects/p1/patterns"} {
		rec := get(t, router, path)
		var items []json.RawMessage
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &items) != nil || len(items) != 0 {
			t.Errorf("GET %s after a failed run: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	if obs, err := observations.ListByProject(ctx, "p1"); err != nil || len(obs) != 0 {
		t.Errorf("a failed run stored %d observations (%v)", len(obs), err)
	}
}
