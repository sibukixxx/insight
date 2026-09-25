package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	httpapi "insight-lab/internal/http"
)

// The Reference Web is optional (#96): with NoWeb only the APIs are served.
func TestNoWebServesAPIButNotTheReferenceWeb(t *testing.T) {
	for _, tc := range []struct {
		noWeb bool
		want  int
	}{{false, http.StatusOK}, {true, http.StatusNotFound}} {
		rec := httptest.NewRecorder()
		httpapi.NewRouter(httpapi.Deps{NoWeb: tc.noWeb}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != tc.want {
			t.Fatalf("noWeb=%v: GET / = %d, want %d", tc.noWeb, rec.Code, tc.want)
		}
	}
}
