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
	"insight-lab/internal/service"
	"insight-lab/internal/usecase"
)

type lifecycleRun struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Lifecycle   string `json:"lifecycle"`
	FailureCode string `json:"failureCode"`
	RetryOf     string `json:"retryOf"`
}

func decodeRun(t *testing.T, rec *httptest.ResponseRecorder) lifecycleRun {
	t.Helper()
	var run lifecycleRun
	if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return run
}

func TestAnalysisHTTPCancelsAQueuedRunAndRetriesItIdempotently(t *testing.T) {
	router, _ := newJobRouter(t) // no workers: runs stay queued
	created := decodeRun(t, post(router, "/api/projects/p1/analysis", `{"label":"first"}`))
	if created.Lifecycle != "QUEUED" {
		t.Fatalf("created lifecycle = %q", created.Lifecycle)
	}

	cancel := post(router, "/api/analysis/"+created.ID+"/cancel", "")
	if cancel.Code != http.StatusAccepted {
		t.Fatalf("cancel: %d %s", cancel.Code, cancel.Body.String())
	}
	if got := decodeRun(t, cancel); got.Status != "failed" || got.Lifecycle != "CANCELLED" || got.FailureCode != "CANCELLED" {
		t.Fatalf("cancelled run = %+v", got)
	}
	if again := post(router, "/api/analysis/"+created.ID+"/cancel", ""); again.Code != http.StatusConflict {
		t.Fatalf("cancel a finished run: %d", again.Code)
	}

	retry := post(router, "/api/analysis/"+created.ID+"/retry", "")
	if retry.Code != http.StatusAccepted {
		t.Fatalf("retry: %d %s", retry.Code, retry.Body.String())
	}
	first := decodeRun(t, retry)
	if first.RetryOf != created.ID || first.Lifecycle != "QUEUED" {
		t.Fatalf("retry run = %+v", first)
	}
	replay := post(router, "/api/analysis/"+created.ID+"/retry", "")
	if replay.Code != http.StatusOK || decodeRun(t, replay).ID != first.ID {
		t.Fatalf("second retry: %d %s", replay.Code, replay.Body.String())
	}
	if queued := post(router, "/api/analysis/"+first.ID+"/retry", ""); queued.Code != http.StatusConflict {
		t.Fatalf("retrying a queued run: %d", queued.Code)
	}
	if missing := post(router, "/api/analysis/ana_missing/cancel", ""); missing.Code != http.StatusNotFound {
		t.Fatalf("cancel unknown: %d", missing.Code)
	}
}

func TestAnalysisEventsSendTheTerminalEventToALateSubscriber(t *testing.T) {
	router, _ := newJobRouter(t)
	created := decodeRun(t, post(router, "/api/projects/p1/analysis", ""))
	post(router, "/api/analysis/"+created.ID+"/cancel", "")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/analysis/"+created.ID+"/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req) // must return on its own: the run already ended
	if ctx.Err() != nil {
		t.Fatal("the event stream of a finished analysis did not end")
	}
	body := rec.Body.String()
	if !strings.HasPrefix(body, "event: error\ndata: ") || !strings.Contains(body, `"code":"CANCELLED"`) {
		t.Fatalf("stream = %q", body)
	}
}

func TestAnalysisEventsOpenWithTheCurrentStatus(t *testing.T) {
	router, _ := newJobRouter(t)
	created := decodeRun(t, post(router, "/api/projects/p1/analysis", ""))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/analysis/"+created.ID+"/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if !strings.HasPrefix(rec.Body.String(), "event: status\ndata: ") || !strings.Contains(rec.Body.String(), `"lifecycle":"QUEUED"`) {
		t.Fatalf("stream = %q", rec.Body.String())
	}
}

func TestCreateAnalysisHTTPAnswers503WhenTheQueueIsFull(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "queue.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	projects, analyses, documents := sqlite.NewProjectRepository(db), sqlite.NewAnalysisRepository(db), sqlite.NewDocumentRepository(db)
	if err := projects.Create(context.Background(), &domain.Project{ID: "p1", Name: "p1", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	jobs := service.NewJobManager(analyses, &service.Pipeline{Documents: documents}, service.NewSettingsStore(service.Settings{}), service.DefaultLLMClientFactory)
	jobs.MaxQueued = 1
	router := httpapi.NewRouter(httpapi.Deps{App: usecase.New(usecase.Repositories{Projects: projects, Documents: documents, Analyses: analyses}), JobManager: jobs})

	if rec := post(router, "/api/projects/p1/analysis", ""); rec.Code != http.StatusAccepted {
		t.Fatalf("first: %d %s", rec.Code, rec.Body.String())
	}
	rec := post(router, "/api/projects/p1/analysis", "")
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("over the bound: %d retry-after=%q %s", rec.Code, rec.Header().Get("Retry-After"), rec.Body.String())
	}
}
