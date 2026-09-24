package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestCompareEndpointReturnsAttributionAndNonCausalExplanation(t *testing.T) {
	router := newRunScopeRouter(t)
	rec := get(t, router, "/api/projects/p1/analyses/compare?a=a1&b=a2")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Attribution string   `json:"attribution"`
		Explanation []string `json:"explanation"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Attribution != "ATTRIBUTION_UNAVAILABLE" || len(body.Explanation) == 0 {
		t.Fatalf("legacy runs compare = %+v", body)
	}
	if rec := get(t, router, "/api/analysis/a1/compare/a2"); rec.Code != http.StatusOK {
		t.Fatalf("id-addressed compare status %d", rec.Code)
	}
}

func TestCompareEndpointRejectsMissingOrForeignRuns(t *testing.T) {
	router := newRunScopeRouter(t)
	cases := map[string]int{
		"/api/projects/p1/analyses/compare?a=a1":      http.StatusBadRequest,
		"/api/projects/p2/analyses/compare?a=a1&b=a2": http.StatusNotFound,
		"/api/analysis/a1/compare/missing":            http.StatusNotFound,
	}
	for path, want := range cases {
		if rec := get(t, router, path); rec.Code != want {
			t.Errorf("%s: status %d, want %d (%s)", path, rec.Code, want, rec.Body.String())
		}
	}
}

func TestMetricsHistoryEndpointListsRuns(t *testing.T) {
	router := newRunScopeRouter(t)
	rec := get(t, router, "/api/projects/p1/metrics-history")
	var body []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body) != 3 {
		t.Fatalf("history = %s (%v)", rec.Body.String(), err)
	}
}
