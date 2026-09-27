package handler

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
	"insight-lab/internal/service"
)

// maxIngestManifestBytes bounds the acquisition manifest part of a
// multipart ingest upload; it is read into memory, the file never is.
const maxIngestManifestBytes = 64 << 10

// maxMultipartOverhead is the allowance for multipart headers and fields
// on top of the file itself.
const maxMultipartOverhead = 1 << 20

// Next actions a client can offer for an ingest receipt.
const (
	ingestNextWait     = "WAIT"
	ingestNextAnalyze  = "START_ANALYSIS"
	ingestNextReupload = "FIX_AND_REUPLOAD"
)

type ingestPreviewDTO struct {
	// Scope is SAMPLE: the first documents only, never the whole file.
	Scope     string        `json:"scope"`
	Documents []documentDTO `json:"documents"`
}

type ingestDTO struct {
	ID               string                  `json:"id"`
	ProjectID        string                  `json:"projectId"`
	Kind             string                  `json:"kind"`
	State            domain.IngestState      `json:"state"`
	Stage            string                  `json:"stage"`
	FileName         string                  `json:"fileName,omitempty"`
	FileSHA256       string                  `json:"fileSha256"`
	SizeBytes        int64                   `json:"sizeBytes"`
	BytesRead        int64                   `json:"bytesRead"`
	RowsRead         int64                   `json:"rowsRead"`
	RowsSkipped      int64                   `json:"rowsSkipped"`
	DocumentsCreated int64                   `json:"documentsCreated"`
	ErrorCount       int64                   `json:"errorCount"`
	ErrorExamples    []domain.IngestRowError `json:"errorExamples"`
	ErrorsTruncated  bool                    `json:"errorsTruncated"`
	Failure          string                  `json:"failure,omitempty"`
	NextAction       string                  `json:"nextAction"`
	HasManifest      bool                    `json:"hasManifest"`
	CreatedAt        string                  `json:"createdAt"`
	UpdatedAt        string                  `json:"updatedAt"`
	FinishedAt       string                  `json:"finishedAt,omitempty"`
	Preview          *ingestPreviewDTO       `json:"preview,omitempty"`
}

func toIngestDTO(job *domain.Ingest) ingestDTO {
	dto := ingestDTO{
		ID: job.ID, ProjectID: job.ProjectID, Kind: job.Kind, State: job.State, Stage: job.Stage,
		FileName: job.FileName, FileSHA256: job.FileSHA256, SizeBytes: job.SizeBytes,
		BytesRead: job.BytesRead, RowsRead: job.RowsRead, RowsSkipped: job.RowsSkipped,
		DocumentsCreated: job.DocumentsCreated, ErrorCount: job.ErrorCount,
		ErrorExamples:   job.ErrorExamples,
		ErrorsTruncated: job.ErrorCount > int64(len(job.ErrorExamples)),
		Failure:         job.Failure, HasManifest: job.Manifest != "",
		CreatedAt: job.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: job.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if dto.ErrorExamples == nil {
		dto.ErrorExamples = []domain.IngestRowError{}
	}
	if job.FinishedAt != nil {
		dto.FinishedAt = job.FinishedAt.UTC().Format(time.RFC3339)
	}
	switch job.State {
	case domain.IngestReady:
		dto.NextAction = ingestNextAnalyze
	case domain.IngestFailed, domain.IngestCancelled:
		dto.NextAction = ingestNextReupload
	default:
		dto.NextAction = ingestNextWait
	}
	return dto
}

// CreateIngest accepts a large CSV for asynchronous ingestion. The body is
// either raw CSV (kind and filename in the query) or multipart/form-data
// whose optional "kind" and "manifest" fields precede the "file" part; the
// file is streamed to staging and never held in memory. It answers 202 with
// a new receipt, or 200 with the existing receipt when the same bytes were
// already submitted under the same identity (Idempotency-Key header).
func (h *Handler) CreateIngest(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if h.Ingest == nil {
		writeError(w, http.StatusNotImplemented, service.ErrIngestDisabled.Error())
		return
	}
	if !h.requireProject(w, r, projectID) {
		return
	}
	req := service.IngestRequest{
		ProjectID: projectID, Kind: r.URL.Query().Get("kind"), FileName: r.URL.Query().Get("filename"),
		RequestKey: r.Header.Get("Idempotency-Key"),
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.Ingest.Capability().MaxUploadBytes+maxMultipartOverhead)

	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	var body io.Reader = r.Body
	if mediaType == "multipart/form-data" {
		file, ok := h.readIngestMultipart(w, r, &req)
		if !ok {
			return
		}
		body = file
	}
	if req.Kind == "" {
		req.Kind = service.ImportKindDocuments
	}
	job, created, err := h.Ingest.Submit(r.Context(), req, body)
	if err != nil {
		writeIngestError(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusAccepted
	}
	w.Header().Set("Location", fmt.Sprintf("/api/projects/%s/ingests/%s", projectID, job.ID))
	writeJSON(w, status, toIngestDTO(job))
}

// readIngestMultipart reads the parts before "file" into req and returns
// the file part positioned at its content. ok is false once an error
// response has been written.
func (h *Handler) readIngestMultipart(w http.ResponseWriter, r *http.Request, req *service.IngestRequest) (io.Reader, bool) {
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart body")
		return nil, false
	}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "the file field is required")
			return nil, false
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid multipart body")
			return nil, false
		}
		switch part.FormName() {
		case "file":
			if req.FileName == "" {
				req.FileName = part.FileName()
			}
			return part, true
		case "manifest", "kind":
			value, err := io.ReadAll(io.LimitReader(part, maxIngestManifestBytes+1))
			if err != nil || len(value) > maxIngestManifestBytes {
				writeError(w, http.StatusBadRequest, "the "+part.FormName()+" field is too large")
				return nil, false
			}
			if part.FormName() == "manifest" {
				req.Manifest = value
			} else {
				req.Kind = string(value)
			}
		}
	}
}

func writeIngestError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrIngestInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrIngestTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, service.ErrIngestStorage):
		writeError(w, http.StatusInsufficientStorage, err.Error())
	case errors.Is(err, service.ErrIngestNotCancellable):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, repository.ErrNotFound):
		writeError(w, http.StatusNotFound, "ingest not found")
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func (h *Handler) ListIngests(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if h.Ingest == nil {
		writeError(w, http.StatusNotImplemented, service.ErrIngestDisabled.Error())
		return
	}
	if !h.requireProject(w, r, projectID) {
		return
	}
	jobs, err := h.Ingest.List(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]ingestDTO, 0, len(jobs))
	for _, job := range jobs {
		out = append(out, toIngestDTO(job))
	}
	writeJSON(w, http.StatusOK, out)
}

// GetIngest returns the receipt with a SAMPLE preview of its first
// documents.
func (h *Handler) GetIngest(w http.ResponseWriter, r *http.Request) {
	job, ok := h.loadIngest(w, r)
	if !ok {
		return
	}
	docs, err := h.Ingest.Preview(r.Context(), job)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	dto := toIngestDTO(job)
	dto.Preview = &ingestPreviewDTO{Scope: "SAMPLE", Documents: make([]documentDTO, 0, len(docs))}
	for _, d := range docs {
		dto.Preview.Documents = append(dto.Preview.Documents, toDocumentDTO(d))
	}
	writeJSON(w, http.StatusOK, dto)
}

// CancelIngest answers 202 with the receipt; a running ingest reaches
// CANCELLED shortly after, which the client observes by polling.
func (h *Handler) CancelIngest(w http.ResponseWriter, r *http.Request) {
	if h.Ingest == nil {
		writeError(w, http.StatusNotImplemented, service.ErrIngestDisabled.Error())
		return
	}
	job, err := h.Ingest.Cancel(r.Context(), chi.URLParam(r, "projectID"), chi.URLParam(r, "ingestID"))
	if err != nil {
		writeIngestError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, toIngestDTO(job))
}

// IngestErrors exports every rejected row as CSV (row,reason). An ingest
// without rejected rows exports the header only.
func (h *Handler) IngestErrors(w http.ResponseWriter, r *http.Request) {
	job, ok := h.loadIngest(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-errors.csv"`, job.ID))
	f, err := h.Ingest.OpenErrors(job)
	if errors.Is(err, os.ErrNotExist) {
		_, _ = io.WriteString(w, "row,reason\n")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close()
	_, _ = io.Copy(w, f)
}

func (h *Handler) loadIngest(w http.ResponseWriter, r *http.Request) (*domain.Ingest, bool) {
	if h.Ingest == nil {
		writeError(w, http.StatusNotImplemented, service.ErrIngestDisabled.Error())
		return nil, false
	}
	job, err := h.Ingest.Get(r.Context(), chi.URLParam(r, "projectID"), chi.URLParam(r, "ingestID"))
	if err != nil {
		writeIngestError(w, err)
		return nil, false
	}
	return job, true
}
