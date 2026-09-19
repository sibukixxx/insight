package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
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

func newImportTestRouter(t *testing.T) http.Handler {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "http_import.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	projects := sqlite.NewProjectRepository(db)
	documents := sqlite.NewDocumentRepository(db)
	ctx := context.Background()
	if err := projects.Create(ctx, &domain.Project{ID: "p1", Name: "Dataset project", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	app := usecase.New(usecase.Repositories{Projects: projects, Documents: documents})
	return httpapi.NewRouter(httpapi.Deps{App: app})
}

// multipartUpload builds a multipart/form-data body with the given text
// fields plus one file field, mirroring what a browser upload sends.
func multipartUpload(t *testing.T, fields map[string]string, fileField, fileName, fileContent string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	part, err := w.CreateFormFile(fileField, fileName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(fileContent)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return body, w.FormDataContentType()
}

func listDocumentsHTTP(t *testing.T, router http.Handler) []map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/projects/p1/documents", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("list documents: %d %s", resp.Code, resp.Body.String())
	}
	var docs []map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &docs); err != nil {
		t.Fatal(err)
	}
	return docs
}

func TestImportDocumentsCSVHTTPWithManifestAttachesProvenanceToDocuments(t *testing.T) {
	router := newImportTestRouter(t)
	manifest := `{"sourceName":"e-Stat","datasetId":"000032143614","retrievalMethod":"download","retrievedAt":"2026-09-01T09:00:00Z","schemaId":"estat-economic-census-enterprise"}`
	csvData := "id,source,title,content\n001,interview,A,本文です\n"
	body, contentType := multipartUpload(t, map[string]string{"manifest": manifest}, "file", "docs.csv", csvData)

	req := httptest.NewRequest(http.MethodPost, "/api/projects/p1/documents/import", body)
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("import: %d %s", resp.Code, resp.Body.String())
	}

	docs := listDocumentsHTTP(t, router)
	if len(docs) != 1 {
		t.Fatalf("documents = %d, want 1", len(docs))
	}
	meta, _ := docs[0]["metadata"].(map[string]any)
	manifestJSON, _ := meta["acquisition_manifest"].(string)
	if meta == nil || !strings.Contains(manifestJSON, "e-Stat") {
		t.Fatalf("manifest not attached: %v", docs[0])
	}
	if hash, _ := meta["dataset_hash"].(string); hash == "" {
		t.Errorf("dataset_hash missing: %v", meta)
	}
}

func TestImportDocumentsCSVHTTPRejectsInvalidManifestBeforeImporting(t *testing.T) {
	router := newImportTestRouter(t)
	manifest := `{"sourceName":"e-Stat","datasetId":"1","retrievalMethod":"download","retrievedAt":"2026-09-01T09:00:00Z","schemaId":"x","queryParameters":{"api_key":"leak"}}`
	csvData := "id,source,title,content\n001,interview,A,本文です\n"
	body, contentType := multipartUpload(t, map[string]string{"manifest": manifest}, "file", "docs.csv", csvData)

	req := httptest.NewRequest(http.MethodPost, "/api/projects/p1/documents/import", body)
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest || !strings.Contains(resp.Body.String(), "credentials") {
		t.Fatalf("expected 400 credential rejection, got %d %s", resp.Code, resp.Body.String())
	}

	if docs := listDocumentsHTTP(t, router); len(docs) != 0 {
		t.Errorf("no documents should be created when the manifest is invalid, got %d", len(docs))
	}
}

func TestImportDocumentsCSVHTTPWithoutManifestStillRecordsFileHash(t *testing.T) {
	router := newImportTestRouter(t)
	csvData := "id,source,title,content\n001,interview,A,本文です\n"
	body, contentType := multipartUpload(t, nil, "file", "docs.csv", csvData)

	req := httptest.NewRequest(http.MethodPost, "/api/projects/p1/documents/import", body)
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("import: %d %s", resp.Code, resp.Body.String())
	}

	docs := listDocumentsHTTP(t, router)
	if len(docs) != 1 {
		t.Fatalf("documents = %d, want 1", len(docs))
	}
	meta, _ := docs[0]["metadata"].(map[string]any)
	if hash, _ := meta["dataset_hash"].(string); hash == "" {
		t.Errorf("dataset_hash should be recorded even without a manifest: %v", meta)
	}
}

func TestImportAnalysisCSVHTTPWithManifestAttachesProvenanceToAggregates(t *testing.T) {
	router := newImportTestRouter(t)
	manifest := `{"sourceName":"ja-company-base","datasetId":"export-2026-01","retrievalMethod":"user_provided","retrievedAt":"2026-03-01T00:00:00Z","schemaId":"ja-company-analysis-csv","unit":"administrative records"}`
	csvData := "corporate_number,name,kind,prefecture_code,prefecture_name,city_code,city_name,assignment_date,update_date,change_date,close_date,close_cause,event_type,source_provider,source_version,source_fetched_at\n" +
		"1000000000001,A,株式会社,13,東京都,229,西東京市,2026-01-02,,,,,ASSIGNED,houjin_bangou,v4,2026-03-01T00:00:00Z\n"
	body, contentType := multipartUpload(t, map[string]string{"manifest": manifest}, "file", "records.csv", csvData)

	req := httptest.NewRequest(http.MethodPost, "/api/projects/p1/documents/import/analysis", body)
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("import: %d %s", resp.Code, resp.Body.String())
	}

	docs := listDocumentsHTTP(t, router)
	if len(docs) != 1 {
		t.Fatalf("documents = %d, want 1", len(docs))
	}
	meta, _ := docs[0]["metadata"].(map[string]any)
	manifestJSON, _ := meta["acquisition_manifest"].(string)
	if meta == nil || !strings.Contains(manifestJSON, "ja-company-base") {
		t.Fatalf("manifest not attached to aggregate: %v", docs[0])
	}
}
