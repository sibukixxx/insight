package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/domain"
)

// GetEvaluation returns the evaluation metrics (see
// docs/detailed-design.md §15) of one analysis run: the one named by
// ?analysisId=, or else the latest completed run. The pipeline stores them as analyses.metrics JSON directly
// (internal/service.Metrics), so this just passes that JSON through.
func (h *Handler) GetEvaluation(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}

	a, err := h.App.ResolveAnalysis(r.Context(), projectID, analysisIDParam(r))
	if err != nil {
		writeRunScopeError(w, err)
		return
	}
	if a.Status != domain.AnalysisCompleted || a.Metrics == "" {
		writeError(w, http.StatusConflict, "the selected analysis run has not completed")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(a.Metrics))
}
