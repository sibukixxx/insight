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
	"insight-lab/internal/service"
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
	// SpeakerRoles are remembered in the project's intake profile.
	SpeakerRoles map[string]string `json:"speakerRoles"`
	// DetectSpeakers derives spans server-side from the (masked) content.
	DetectSpeakers bool `json:"detectSpeakers"`
	// SkipMask stores the content without PII masking.
	SkipMask bool `json:"skipMask"`
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
		SpeakerRoles: toSpeakerRoles(req.SpeakerRoles), DetectSpeakers: req.DetectSpeakers, SkipMask: req.SkipMask,
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

// uploadReader returns the uploaded file for a multipart request (field
// "file") or the raw body otherwise, plus the optional "mapping" form
// field (a JSON ColumnMapping) when present.
func uploadReader(w http.ResponseWriter, r *http.Request) (io.Reader, string, bool) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		file, _, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "file フィールドが必要です")
			return nil, "", false
		}
		return file, r.FormValue("mapping"), true
	}
	return r.Body, "", true
}

// ImportDocumentsCSV imports a CSV/TSV upload. Without a "mapping" field
// the file must be the fixed id,source,title,content layout; with one
// (JSON ColumnMapping, as returned by the preview) any layout imports.
func (h *Handler) ImportDocumentsCSV(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}
	reader, mappingJSON, ok := uploadReader(w, r)
	if !ok {
		return
	}

	var result *service.ImportResult
	var err error
	if mappingJSON == "" {
		result, err = h.App.ImportDocumentsCSV(r.Context(), projectID, reader)
	} else {
		var mapping domain.ColumnMapping
		if jerr := json.Unmarshal([]byte(mappingJSON), &mapping); jerr != nil {
			writeError(w, http.StatusBadRequest, "mapping の JSON が不正です")
			return
		}
		result, err = h.App.ImportDocumentsTable(r.Context(), projectID, reader, mapping)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type importPreviewDTO struct {
	*service.TablePreview
	// SavedMapping is the mapping this project used last time, if any.
	SavedMapping *domain.ColumnMapping `json:"savedMapping,omitempty"`
}

// PreviewImport parses an upload and proposes a column mapping without
// storing anything.
func (h *Handler) PreviewImport(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}
	reader, _, ok := uploadReader(w, r)
	if !ok {
		return
	}
	preview, saved, err := h.App.PreviewImport(r.Context(), projectID, reader)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, importPreviewDTO{TablePreview: preview, SavedMapping: saved})
}
