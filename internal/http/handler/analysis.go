package handler

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

type analysisDTO struct {
	ID          string  `json:"id"`
	ProjectID   string  `json:"projectId"`
	Status      string  `json:"status"`
	CurrentStep string  `json:"currentStep,omitempty"`
	Progress    int     `json:"progress"`
	Error       string  `json:"error,omitempty"`
	StartedAt   *string `json:"startedAt,omitempty"`
	FinishedAt  *string `json:"finishedAt,omitempty"`
	CreatedAt   string  `json:"createdAt"`
}

func toAnalysisDTO(a *domain.Analysis) analysisDTO {
	dto := analysisDTO{
		ID: a.ID, ProjectID: a.ProjectID, Status: string(a.Status),
		CurrentStep: a.CurrentStep, Progress: a.Progress, Error: a.Error,
		CreatedAt: a.CreatedAt.UTC().Format(time.RFC3339),
	}
	if a.StartedAt != nil {
		s := a.StartedAt.UTC().Format(time.RFC3339)
		dto.StartedAt = &s
	}
	if a.FinishedAt != nil {
		s := a.FinishedAt.UTC().Format(time.RFC3339)
		dto.FinishedAt = &s
	}
	return dto
}

func (h *Handler) CreateAnalysis(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}
	a, err := h.JobManager.Enqueue(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, toAnalysisDTO(a))
}

func (h *Handler) GetAnalysis(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "analysisID")
	a, err := h.Analyses.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "analysis not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toAnalysisDTO(a))
}

func (h *Handler) ListAnalyses(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}
	list, err := h.Analyses.ListByProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]analysisDTO, 0, len(list))
	for _, a := range list {
		out = append(out, toAnalysisDTO(a))
	}
	writeJSON(w, http.StatusOK, out)
}

// AnalysisEvents streams progress/error/completed events over SSE (see
// docs/detailed-design.md §9). A client that reloads mid-run should fall
// back to GET /api/analysis/{id} for the current snapshot rather than
// depend on catching every event.
func (h *Handler) AnalysisEvents(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "analysisID")
	if _, err := h.Analyses.Get(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "analysis not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := h.JobManager.Subscribe(id)
	defer h.JobManager.Unsubscribe(id, ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Event, ev.Data)
			flusher.Flush()
			if ev.Event == "completed" || ev.Event == "error" {
				return
			}
		}
	}
}
