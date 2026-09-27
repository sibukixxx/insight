package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

func newIngestTestRouter(t *testing.T, limits service.IngestLimits) http.Handler {
	t.Helper()
	base := t.TempDir()
	db, err := sqlite.Open(filepath.Join(base, "http_ingest.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	projects := sqlite.NewProjectRepository(db)
	ctx := context.Background()
	for _, id := range []string{"p1", "p2"} {
		if err := projects.Create(ctx, &domain.Project{ID: id, Name: id, CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
	}
	ingest, err := service.NewIngestManager(sqlite.NewIngestRepository(db), filepath.Join(base, "ingest"), limits)
	if err != nil {
		t.Fatal(err)
	}
	workerCtx, cancel := context.WithCancel(ctx)
	ingest.Start(workerCtx)
	t.Cleanup(func() { cancel(); ingest.Wait() })
	app := usecase.New(usecase.Repositories{Projects: projects, Documents: sqlite.NewDocumentRepository(db)})
	return httpapi.NewRouter(httpapi.Deps{App: app, Ingest: ingest})
}

type ingestReceipt struct {
	ID               string `json:"id"`
	State            string `json:"state"`
	FileName         string `json:"fileName"`
	FileSHA256       string `json:"fileSha256"`
	SizeBytes        int64  `json:"sizeBytes"`
	DocumentsCreated int64  `json:"documentsCreated"`
	RowsSkipped      int64  `json:"rowsSkipped"`
	ErrorCount       int64  `json:"errorCount"`
	NextAction       string `json:"nextAction"`
	HasManifest      bool   `json:"hasManifest"`
	Failure          string `json:"failure"`
	Preview          *struct {
		Scope     string           `json:"scope"`
		Documents []map[string]any `json:"documents"`
	} `json:"preview"`
}

func serve(router http.Handler, req *http.Request) *httptest.ResponseRecorder {
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func decodeReceipt(t *testing.T, resp *httptest.ResponseRecorder) ingestReceipt {
	t.Helper()
	var r ingestReceipt
	if err := json.Unmarshal(resp.Body.Bytes(), &r); err != nil {
		t.Fatalf("decode %q: %v", resp.Body.String(), err)
	}
	return r
}

func awaitIngestHTTP(t *testing.T, router http.Handler, id string) ingestReceipt {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		resp := serve(router, httptest.NewRequest(http.MethodGet, "/api/projects/p1/ingests/"+id, nil))
		if resp.Code != http.StatusOK {
			t.Fatalf("get ingest: %d %s", resp.Code, resp.Body.String())
		}
		r := decodeReceipt(t, resp)
		if r.State == "READY" || r.State == "FAILED" || r.State == "CANCELLED" || time.Now().After(deadline) {
			return r
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// generatedCSV streams a documents CSV of about size bytes without ever
// holding it in memory.
type generatedCSV struct {
	size, sent int64
	row        int
	buf        []byte
}

func (g *generatedCSV) Read(p []byte) (int, error) {
	for len(g.buf) < len(p) && g.sent < g.size {
		var line string
		if g.row == 0 {
			line = "id,source,title,content\n"
		} else {
			line = fmt.Sprintf("r%d,dataset,Row %d,%s\n", g.row, g.row, strings.Repeat("v", 2000))
		}
		g.row++
		g.sent += int64(len(line))
		g.buf = append(g.buf, line...)
	}
	if len(g.buf) == 0 {
		return 0, io.EOF
	}
	n := copy(p, g.buf)
	g.buf = g.buf[n:]
	return n, nil
}

func TestIngestHTTPAcceptsCSVLargerThanPreviewLimitAndReachesReady(t *testing.T) {
	if testing.Short() {
		t.Skip("large upload")
	}
	router := newIngestTestRouter(t, service.DefaultIngestLimits())
	size := int64(service.MaxImportPreviewBytes + 4<<20)

	preview := httptest.NewRequest(http.MethodPost, "/api/projects/p1/documents/import/preview?kind=documents", &generatedCSV{size: size})
	preview.Header.Set("Content-Type", "text/csv")
	if resp := serve(router, preview); resp.Code != http.StatusBadRequest || !strings.Contains(resp.Body.String(), "32 MB preview limit") {
		t.Fatalf("synchronous preview must keep its explicit limit: %d %s", resp.Code, resp.Body.String())
	}

	stream := &generatedCSV{size: size}
	req := httptest.NewRequest(http.MethodPost, "/api/projects/p1/ingests?kind=documents&filename=big.csv", stream)
	req.Header.Set("Content-Type", "text/csv")
	resp := serve(router, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("create ingest: %d %s", resp.Code, resp.Body.String())
	}
	accepted := decodeReceipt(t, resp)
	if accepted.SizeBytes != stream.sent || accepted.SizeBytes <= service.MaxImportPreviewBytes || accepted.NextAction != "WAIT" {
		t.Fatalf("accepted receipt = %+v (sent %d)", accepted, stream.sent)
	}
	done := awaitIngestHTTP(t, router, accepted.ID)
	if done.State != "READY" || done.NextAction != "START_ANALYSIS" || done.DocumentsCreated != int64(stream.row-1) {
		t.Fatalf("finished receipt = %+v, want READY with %d documents", done, stream.row-1)
	}
	if done.Preview == nil || done.Preview.Scope != "SAMPLE" || len(done.Preview.Documents) != service.IngestPreviewDocuments {
		t.Fatalf("preview = %+v", done.Preview)
	}
	if docs := listDocumentsHTTP(t, router); int64(len(docs)) != done.DocumentsCreated {
		t.Fatalf("listed documents = %d, want %d", len(docs), done.DocumentsCreated)
	}
}

func TestIngestHTTPMultipartKeepsManifestSanitizesFileNameAndReplaysIdempotently(t *testing.T) {
	router := newIngestTestRouter(t, service.DefaultIngestLimits())
	manifest := `{"sourceName":"e-Stat","datasetId":"000032143614","retrievalMethod":"download","retrievedAt":"2026-09-01T09:00:00Z","schemaId":"estat"}`
	csvData := "id,source,title,content\nr1,dataset,T1,one\nr2,nonsense,T2,two\n"
	post := func() *httptest.ResponseRecorder {
		body, contentType := multipartUpload(t, map[string]string{"manifest": manifest, "kind": "documents"}, "file", `..\..\secret\data.csv`, csvData)
		req := httptest.NewRequest(http.MethodPost, "/api/projects/p1/ingests", body)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Idempotency-Key", "upload-1")
		return serve(router, req)
	}
	first := post()
	if first.Code != http.StatusAccepted {
		t.Fatalf("create: %d %s", first.Code, first.Body.String())
	}
	receipt := decodeReceipt(t, first)
	if receipt.FileName != "data.csv" || !receipt.HasManifest {
		t.Fatalf("receipt = %+v", receipt)
	}
	done := awaitIngestHTTP(t, router, receipt.ID)
	if done.State != "READY" || done.DocumentsCreated != 1 || done.RowsSkipped != 1 {
		t.Fatalf("finished = %+v", done)
	}
	docs := listDocumentsHTTP(t, router)
	if meta, _ := docs[0]["metadata"].(map[string]any); !strings.Contains(fmt.Sprint(meta["acquisition_manifest"]), "e-Stat") {
		t.Fatalf("manifest not attached: %v", docs[0])
	}

	replay := post()
	if replay.Code != http.StatusOK || decodeReceipt(t, replay).ID != receipt.ID {
		t.Fatalf("replay: %d %s", replay.Code, replay.Body.String())
	}
	if got := listDocumentsHTTP(t, router); len(got) != 1 {
		t.Fatalf("replay duplicated evidence: %d documents", len(got))
	}

	export := serve(router, httptest.NewRequest(http.MethodGet, "/api/projects/p1/ingests/"+receipt.ID+"/errors.csv", nil))
	if export.Code != http.StatusOK || export.Body.String() != "row,reason\n2,\"invalid source: \"\"nonsense\"\"\"\n" {
		t.Fatalf("error export: %d %q", export.Code, export.Body.String())
	}
	if resp := serve(router, httptest.NewRequest(http.MethodPost, "/api/projects/p1/ingests/"+receipt.ID+"/cancel", nil)); resp.Code != http.StatusConflict {
		t.Fatalf("cancel READY: %d %s", resp.Code, resp.Body.String())
	}
	if resp := serve(router, httptest.NewRequest(http.MethodGet, "/api/projects/p2/ingests/"+receipt.ID, nil)); resp.Code != http.StatusNotFound {
		t.Fatalf("another project read the ingest: %d", resp.Code)
	}
}

func TestIngestHTTPMapsLimitsAndInvalidRequestsToStatusCodes(t *testing.T) {
	limits := service.DefaultIngestLimits()
	limits.MaxUploadBytes = 128
	router := newIngestTestRouter(t, limits)
	post := func(path, body string) int {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "text/csv")
		return serve(router, req).Code
	}
	cases := []struct {
		name, path, body string
		want             int
	}{
		{"over the file limit", "/api/projects/p1/ingests", strings.Repeat("x", 129), http.StatusRequestEntityTooLarge},
		{"unsupported kind", "/api/projects/p1/ingests?kind=pdf", "id\n", http.StatusBadRequest},
		{"empty file", "/api/projects/p1/ingests", "", http.StatusBadRequest},
		{"unknown project", "/api/projects/nope/ingests", "id\n", http.StatusNotFound},
	}
	for _, c := range cases {
		if got := post(c.path, c.body); got != c.want {
			t.Errorf("%s: status %d, want %d", c.name, got, c.want)
		}
	}
	if resp := serve(router, httptest.NewRequest(http.MethodGet, "/api/projects/p1/ingests/ingest_missing", nil)); resp.Code != http.StatusNotFound {
		t.Errorf("missing ingest: %d", resp.Code)
	}
}

func TestIngestHTTPAdvertisesCapabilityAndAnswers501WhenDisabled(t *testing.T) {
	enabled := newIngestTestRouter(t, service.DefaultIngestLimits())
	var health struct {
		Capabilities struct {
			LargeIngest service.IngestCapability `json:"largeIngest"`
			Runtime     struct {
				Mode string `json:"mode"`
			} `json:"runtime"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(serve(enabled, httptest.NewRequest(http.MethodGet, "/api/health", nil)).Body.Bytes(), &health); err != nil {
		t.Fatal(err)
	}
	if got := health.Capabilities.LargeIngest; !got.Enabled || got.MaxUploadBytes != service.DefaultIngestLimits().MaxUploadBytes {
		t.Fatalf("capability = %+v", got)
	}
	if health.Capabilities.Runtime.Mode != "LOCAL" {
		t.Fatalf("runtime mode = %q, want LOCAL by default", health.Capabilities.Runtime.Mode)
	}

	disabled := newImportTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/projects/p1/ingests", strings.NewReader("id\n"))
	if resp := serve(disabled, req); resp.Code != http.StatusNotImplemented {
		t.Fatalf("disabled ingest: %d %s", resp.Code, resp.Body.String())
	}
}
