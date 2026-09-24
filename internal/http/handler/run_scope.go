package handler

import (
	"errors"
	"net/http"

	"insight-lab/internal/usecase"
)

// analysisIDParam is the optional run selector shared by every run-scoped
// endpoint. Empty means the latest completed analysis run.
func analysisIDParam(r *http.Request) string {
	return r.URL.Query().Get("analysisId")
}

// writeRunScopeError maps run-resolution failures to HTTP statuses. An
// analysis from another project is reported exactly like an unknown one.
func writeRunScopeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, usecase.ErrNoCompletedAnalysis):
		writeError(w, http.StatusNotFound, "no completed analysis run is available yet")
	case errors.Is(err, usecase.ErrNotFound):
		writeError(w, http.StatusNotFound, "analysis not found in this project")
	case errors.Is(err, usecase.ErrMixedAnalysisRuns):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
