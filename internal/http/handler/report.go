package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/usecase"
)

func (h *Handler) ExportProjectReport(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	report, err := h.App.ExportProjectMarkdown(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="insight-report.md"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(report)
}
