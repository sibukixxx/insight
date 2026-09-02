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

type spanDTO struct {
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Speaker string `json:"speaker,omitempty"`
	Role    string `json:"role"`
}

type documentDTO struct {
	ID         string `json:"id"`
	ProjectID  string `json:"projectId"`
	Source     string `json:"source"`
	Provenance string `json:"provenance"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	// Masked reports whether PII masking changed the content at intake;
	// the pre-masking original stays local and is not returned here.
	Masked    bool              `json:"masked"`
	Spans     []spanDTO         `json:"spans"`
	Situation string            `json:"situation,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt string            `json:"createdAt"`
}

func toSpanDTOs(spans []domain.Span) []spanDTO {
	out := make([]spanDTO, 0, len(spans))
	for _, s := range spans {
		out = append(out, spanDTO{Start: s.Start, End: s.End, Speaker: s.Speaker, Role: string(s.Role)})
	}
	return out
}

func fromSpanDTOs(spans []spanDTO) []domain.Span {
	out := make([]domain.Span, 0, len(spans))
	for _, s := range spans {
		out = append(out, domain.Span{Start: s.Start, End: s.End, Speaker: s.Speaker, Role: domain.SpeakerRole(s.Role)})
	}
	return out
}

func toDocumentDTO(d *domain.Document) documentDTO {
	return documentDTO{
		ID:         d.ID,
		ProjectID:  d.ProjectID,
		Source:     string(d.Source),
		Provenance: string(d.Provenance),
		Title:      d.Title,
		Content:    d.Content,
		Masked:     d.RawContent != "",
		Spans:      toSpanDTOs(d.Spans),
		Situation:  d.Situation(),
		Metadata:   d.Metadata,
		CreatedAt:  d.CreatedAt.UTC().Format(time.RFC3339),
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
	Source     string            `json:"source"`
	Provenance string            `json:"provenance"`
	Title      string            `json:"title"`
	Content    string            `json:"content"`
	Spans      []spanDTO         `json:"spans"`
	Metadata   map[string]string `json:"metadata"`
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
		ProjectID: projectID, Source: source, Provenance: domain.Provenance(req.Provenance),
		Title: req.Title, Content: req.Content, Spans: fromSpanDTOs(req.Spans), Metadata: req.Metadata,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
// id,source,title,content shape (see internal/service/csv_import.go).
func (h *Handler) ImportDocumentsCSV(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}

	var reader io.Reader = r.Body
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		file, _, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "file フィールドが必要です")
			return
		}
		defer file.Close()
		reader = file
	}

	result, err := h.App.ImportDocumentsCSV(r.Context(), projectID, reader)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
