package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/service"
	"insight-lab/internal/triage"
)

// ListImportFormats serves the file formats the import endpoints accept,
// so the browser UI never offers a format the server cannot ingest.
func (h *Handler) ListImportFormats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, service.ImportFormats())
}

// ImportTemplate serves the header-only CSV template for one import kind.
func (h *Handler) ImportTemplate(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	template, err := service.ImportTemplateCSV(kind)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="insight-lab-`+kind+`-template.csv"`)
	_, _ = w.Write(template)
}

type importPreviewDTO struct {
	Kind string `json:"kind"`
	// Scope is EXHAUSTIVE: every row of the (size-limited) file was read.
	// A large-ingest receipt previews a SAMPLE instead.
	Scope          string                   `json:"scope"`
	RecordsRead    int                      `json:"recordsRead"`
	Importable     int                      `json:"importable"`
	Skipped        int                      `json:"skipped"`
	Errors         []service.ImportRowError `json:"errors"`
	FileHash       string                   `json:"fileHash"`
	Documents      []documentDTO            `json:"documents"`
	TotalDocuments int                      `json:"totalDocuments"`
	Profile        *triage.Profile          `json:"profile,omitempty"`
	ProfileError   string                   `json:"profileError,omitempty"`
}

// PreviewImport runs an import as a dry run: POST
// /api/projects/{projectID}/documents/import/preview?kind=documents|analysis
// with the same body as the import itself. Nothing is stored.
func (h *Handler) PreviewImport(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}
	kind := r.URL.Query().Get("kind")
	if _, ok := service.FindImportFormat(kind); !ok {
		writeError(w, http.StatusBadRequest, "kind must be documents or analysis")
		return
	}
	reader, _, closeUpload, ok := readImportUpload(w, r)
	if !ok {
		return
	}
	defer closeUpload()

	preview, err := h.App.PreviewImport(r.Context(), projectID, kind, reader)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	docs := make([]documentDTO, 0, len(preview.Documents))
	for _, d := range preview.Documents {
		docs = append(docs, toDocumentDTO(d))
	}
	writeJSON(w, http.StatusOK, importPreviewDTO{
		Kind: preview.Kind, Scope: "EXHAUSTIVE", RecordsRead: preview.RecordsRead, Importable: preview.Importable,
		Skipped: preview.Skipped, Errors: preview.Errors, FileHash: preview.FileHash,
		Documents: docs, TotalDocuments: preview.TotalDocuments,
		Profile: preview.Profile, ProfileError: preview.ProfileError,
	})
}
