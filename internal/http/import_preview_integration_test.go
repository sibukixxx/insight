package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestImportFormatsHTTPListsOnlyCSVFormatsWhenRequested(t *testing.T) {
	router := newImportTestRouter(t)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/import-formats", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("import formats: %d %s", resp.Code, resp.Body.String())
	}
	var formats []struct {
		Kind       string   `json:"kind"`
		Extensions []string `json:"extensions"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &formats); err != nil {
		t.Fatal(err)
	}
	if len(formats) != 2 || formats[0].Kind != "documents" || formats[1].Kind != "analysis" {
		t.Fatalf("formats = %+v", formats)
	}
	for _, f := range formats {
		if strings.Join(f.Extensions, ",") != ".csv" {
			t.Errorf("%s extensions = %v", f.Kind, f.Extensions)
		}
	}
}

func TestImportTemplateHTTPServesHeaderOnlyCSVAttachmentWhenKindIsKnown(t *testing.T) {
	router := newImportTestRouter(t)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/import-formats/documents/template.csv", nil))
	if resp.Code != http.StatusOK || resp.Body.String() != "id,source,title,content\n" {
		t.Fatalf("template: %d %q", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Header().Get("Content-Disposition"), "attachment") {
		t.Errorf("Content-Disposition = %q", resp.Header().Get("Content-Disposition"))
	}
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/import-formats/xlsx/template.csv", nil))
	if resp.Code != http.StatusNotFound {
		t.Errorf("unknown kind: %d", resp.Code)
	}
}

func TestImportPreviewHTTPReportsRowsWithoutStoringDocumentsWhenUploaded(t *testing.T) {
	router := newImportTestRouter(t)
	body, contentType := multipartUpload(t, nil, "file", "docs.csv", "id,source,title,content\n1,interview,<b>A</b>,hello\n2,bogus,B,x\n")
	req := httptest.NewRequest(http.MethodPost, "/api/projects/p1/documents/import/preview?kind=documents", body)
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", resp.Code, resp.Body.String())
	}
	var preview struct {
		RecordsRead int `json:"recordsRead"`
		Importable  int `json:"importable"`
		Skipped     int `json:"skipped"`
		Documents   []struct {
			Title string `json:"title"`
		} `json:"documents"`
		Profile *struct {
			RowCount int `json:"rowCount"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.RecordsRead != 2 || preview.Importable != 1 || preview.Skipped != 1 || len(preview.Documents) != 1 || preview.Documents[0].Title != "<b>A</b>" {
		t.Errorf("preview = %+v", preview)
	}
	if preview.Profile == nil || preview.Profile.RowCount != 2 {
		t.Errorf("profile = %+v", preview.Profile)
	}
	if docs := listDocumentsHTTP(t, router); len(docs) != 0 {
		t.Errorf("a preview stored %d documents", len(docs))
	}
}

func TestImportPreviewHTTPRejectsUnknownKindAndBadHeaderWhenUploaded(t *testing.T) {
	router := newImportTestRouter(t)
	for _, tc := range []struct{ kind, body string }{{"pdf", "x"}, {"documents", "a,b\n1,2\n"}} {
		req := httptest.NewRequest(http.MethodPost, "/api/projects/p1/documents/import/preview?kind="+tc.kind, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "text/csv")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusBadRequest {
			t.Errorf("kind %s: %d %s", tc.kind, resp.Code, resp.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/projects/missing/documents/import/preview?kind=documents", strings.NewReader("id,source,title,content\n"))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Errorf("missing project: %d", resp.Code)
	}
}
