package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	httpapi "insight-lab/internal/http"
	"insight-lab/internal/repository/sqlite"
	"insight-lab/internal/sampledata"
	"insight-lab/internal/service"
	"insight-lab/internal/usecase"
)

func newDemoScenarioRouter(t *testing.T) (http.Handler, *sqlite.DocumentRepository) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "demo_scenario.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	projects := sqlite.NewProjectRepository(db)
	documents := sqlite.NewDocumentRepository(db)
	app := usecase.New(usecase.Repositories{Projects: projects, Documents: documents})
	return httpapi.NewRouter(httpapi.Deps{App: app, Demo: &service.DemoLoader{Projects: projects, Documents: documents}}), documents
}

func serveDemo(router http.Handler, method, target string) *httptest.ResponseRecorder {
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(method, target, nil))
	return resp
}

func TestDemoScenariosHTTPIsEmptyAndRefusesProjectsWhenDeliveryBuild(t *testing.T) {
	if sampledata.Embedded {
		t.Skip("delivery build only")
	}
	router, _ := newDemoScenarioRouter(t)
	if resp := serveDemo(router, http.MethodGet, "/api/demo/scenarios"); resp.Code != http.StatusOK || resp.Body.String() != "[]\n" {
		t.Errorf("list: %d %q", resp.Code, resp.Body.String())
	}
	for _, target := range []string{"/api/demo/scenarios/ja-shop-records/input.csv"} {
		if resp := serveDemo(router, http.MethodGet, target); resp.Code != http.StatusConflict {
			t.Errorf("GET %s: %d", target, resp.Code)
		}
	}
	if resp := serveDemo(router, http.MethodPost, "/api/demo/scenarios/ja-shop-records/project"); resp.Code != http.StatusConflict {
		t.Errorf("POST project: %d", resp.Code)
	}
}

// The scenario CSV is served byte for byte, its project is created empty
// and reused on a second click, and the ordinary importer accepts every row.
func TestDemoScenariosHTTPOpensIdempotentEmptyProjectAndServesImportableCSVWhenDemoBuild(t *testing.T) {
	if !sampledata.Embedded {
		t.Skip("demo build only")
	}
	router, documents := newDemoScenarioRouter(t)
	resp := serveDemo(router, http.MethodGet, "/api/demo/scenarios")
	var list []struct {
		ID        string `json:"id"`
		ProjectID string `json:"projectId"`
		DataKind  string `json:"dataKind"`
		Rows      int    `json:"rows"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &list); err != nil || len(list) != 3 {
		t.Fatalf("list: %d %s (%v)", resp.Code, resp.Body.String(), err)
	}
	for _, s := range list {
		csvResp := serveDemo(router, http.MethodGet, "/api/demo/scenarios/"+s.ID+"/input.csv")
		want, _ := sampledata.ScenarioInput(s.ID)
		if csvResp.Code != http.StatusOK || csvResp.Body.String() != string(want) {
			t.Fatalf("%s csv: %d", s.ID, csvResp.Code)
		}
		var first, second struct{ ID string }
		for _, out := range []*struct{ ID string }{&first, &second} {
			r := serveDemo(router, http.MethodPost, "/api/demo/scenarios/"+s.ID+"/project")
			if r.Code != http.StatusOK {
				t.Fatalf("%s project: %d %s", s.ID, r.Code, r.Body.String())
			}
			_ = json.Unmarshal(r.Body.Bytes(), out)
		}
		if first.ID != s.ProjectID || second.ID != s.ProjectID {
			t.Errorf("%s project ids = %s, %s; want %s", s.ID, first.ID, second.ID, s.ProjectID)
		}
		existing, err := documents.ListByProject(context.Background(), s.ProjectID)
		if err != nil || len(existing) != 0 {
			t.Fatalf("%s: new project has %d documents (%v)", s.ID, len(existing), err)
		}

		body, contentType := multipartUpload(t, nil, "file", s.ID+".csv", string(want))
		req := httptest.NewRequest(http.MethodPost, "/api/projects/"+s.ProjectID+"/documents/import", body)
		req.Header.Set("Content-Type", contentType)
		imp := httptest.NewRecorder()
		router.ServeHTTP(imp, req)
		var result struct{ Imported, Skipped int }
		_ = json.Unmarshal(imp.Body.Bytes(), &result)
		if imp.Code != http.StatusOK || result.Imported != s.Rows || result.Skipped != 0 {
			t.Errorf("%s import: %d %s", s.ID, imp.Code, imp.Body.String())
		}
	}
	if r := serveDemo(router, http.MethodPost, "/api/demo/scenarios/nope/project"); r.Code != http.StatusNotFound {
		t.Errorf("unknown scenario: %d", r.Code)
	}
}
