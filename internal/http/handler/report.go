package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/usecase"
)

func (h *Handler) ExportProjectReport(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	report, err := h.App.ExportProjectMarkdown(r.Context(), projectID, analysisIDParam(r))
	if err != nil {
		writeRunScopeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="insight-report.md"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(report)
}

func (h *Handler) ExportResearchReport(w http.ResponseWriter, r *http.Request) {
	report, err := h.App.ExportResearchMarkdown(r.Context(), chi.URLParam(r, "runID"))
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, usecase.ErrNotFound):
			status = http.StatusNotFound
		case errors.Is(err, usecase.ErrMixedAnalysisRuns):
			status = http.StatusConflict
		}
		writeError(w, status, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="research-report.md"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(report)
}
