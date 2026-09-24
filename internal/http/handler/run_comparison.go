package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/usecase"
)

// CompareProjectAnalyses compares two runs of one project (issue #83):
// GET /api/projects/{projectID}/analyses/compare?a=&b=
func (h *Handler) CompareProjectAnalyses(w http.ResponseWriter, r *http.Request) {
	c, err := h.App.CompareAnalyses(r.Context(), chi.URLParam(r, "projectID"), r.URL.Query().Get("a"), r.URL.Query().Get("b"))
	if err != nil {
		writeComparisonError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// CompareAnalyses compares two runs addressed by ID; both must belong to the
// same project: GET /api/analysis/{analysisID}/compare/{otherID}
func (h *Handler) CompareAnalyses(w http.ResponseWriter, r *http.Request) {
	c, err := h.App.CompareAnalysesByID(r.Context(), chi.URLParam(r, "analysisID"), chi.URLParam(r, "otherID"))
	if err != nil {
		writeComparisonError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// MetricsHistory lists a project's runs with metrics and fingerprints.
func (h *Handler) MetricsHistory(w http.ResponseWriter, r *http.Request) {
	history, err := h.App.MetricsHistory(r.Context(), chi.URLParam(r, "projectID"))
	if err != nil {
		writeComparisonError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, history)
}

func writeComparisonError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, usecase.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "query parameters a and b are required")
	case errors.Is(err, usecase.ErrCrossProjectComparison):
		writeError(w, http.StatusBadRequest, "analysis runs belong to different projects")
	default:
		writeRunScopeError(w, err)
	}
}
