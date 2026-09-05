package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/domain"
	"insight-lab/internal/usecase"
)

// GetEvaluation returns the evaluation metrics (see
// docs/detailed-design.md §15) computed by the most recent completed
// analysis. The pipeline stores them as analyses.metrics JSON directly
// (internal/service.Metrics), so this just passes that JSON through.
func (h *Handler) GetEvaluation(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}

	a, err := h.App.LatestAnalysis(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			writeError(w, http.StatusNotFound, "no analysis results are available yet")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if a.Status != domain.AnalysisCompleted || a.Metrics == "" {
		writeError(w, http.StatusConflict, "the latest analysis has not completed")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(a.Metrics))
}
