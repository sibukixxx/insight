package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/domain"
	"insight-lab/internal/usecase"
)

type documentDTO struct {
	ID        string            `json:"id"`
	ProjectID string            `json:"projectId"`
	Source    string            `json:"source"`
	Title     string            `json:"title"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt string            `json:"createdAt"`
}

func toDocumentDTO(d *domain.Document) documentDTO {
	return documentDTO{
		ID:        d.ID,
		ProjectID: d.ProjectID,
		Source:    string(d.Source),
		Title:     d.Title,
		Content:   d.Content,
		Metadata:  d.Metadata,
		CreatedAt: d.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func (h *Handler) requireProject(w http.ResponseWriter, r *http.Request, projectID string) bool {
	if err := h.App.RequireProject(r.Context(), projectID); err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			writeError(w, http.StatusNotFound, "project not found")
			return false
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return false
	}
	return true
}

func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}
	docs, err := h.App.ListDocuments(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]documentDTO, 0, len(docs))
	for _, d := range docs {
		out = append(out, toDocumentDTO(d))
	}
	writeJSON(w, http.StatusOK, out)
}

type createDocumentRequest struct {
	Source   string            `json:"source"`
	Title    string            `json:"title"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata"`
}

func (h *Handler) CreateDocument(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}

	var req createDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	source := domain.SourceType(req.Source)
	if !source.Valid() {
		writeError(w, http.StatusBadRequest, "invalid source type")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	d, err := h.App.CreateDocument(r.Context(), usecase.CreateDocumentInput{
		ProjectID: projectID, Source: source, Title: req.Title, Content: req.Content, Metadata: req.Metadata,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toDocumentDTO(d))
}

func (h *Handler) GetDocument(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "documentID")
	d, err := h.App.GetDocument(r.Context(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			writeError(w, http.StatusNotFound, "document not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toDocumentDTO(d))
}

// ImportDocumentsCSV accepts either a multipart/form-data upload (field
// name "file") or a raw text/csv body, in the fixed
// id,source,title,content shape (see internal/service/csv_import.go). A
// multipart upload may also include a "manifest" field: the JSON
// acquisition manifest from Issue #16, describing where the file came from.
func (h *Handler) ImportDocumentsCSV(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}

	reader, manifest, closeUpload, ok := readImportUpload(w, r)
	if !ok {
		return
	}
	defer closeUpload()

	result, err := h.App.ImportDocumentsCSV(r.Context(), projectID, reader, manifest)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ImportAnalysisCSV accepts the ja-company-base AnalysisRecord CSV contract
// and deterministically aggregates it into dataset Documents. It accepts
// the same optional "manifest" multipart field as ImportDocumentsCSV.
func (h *Handler) ImportAnalysisCSV(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}
	reader, manifest, closeUpload, ok := readImportUpload(w, r)
	if !ok {
		return
	}
	defer closeUpload()

	result, err := h.App.ImportAnalysisCSV(r.Context(), projectID, reader, manifest)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// readImportUpload extracts the CSV body and, for multipart uploads, the
// optional "manifest" JSON field. ok is false once an error response has
// already been written. The returned close function must be deferred by
// the caller (not here) since the upload's underlying file must stay open
// until the caller has finished reading it.
func readImportUpload(w http.ResponseWriter, r *http.Request) (reader io.Reader, manifest io.Reader, closeUpload func(), ok bool) {
	closeUpload = func() {}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return r.Body, nil, closeUpload, true
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "the file field is required")
		return nil, nil, closeUpload, false
	}
	closeUpload = func() { file.Close() }
	if m := r.FormValue("manifest"); m != "" {
		manifest = strings.NewReader(m)
	}
	return file, manifest, closeUpload, true
}
