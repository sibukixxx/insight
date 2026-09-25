package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/domain"
	"insight-lab/internal/service"
	"insight-lab/internal/usecase"
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
	// Metrics is the recorded evaluation summary, including run provenance.
	// Only a completed run carries it.
	Metrics json.RawMessage `json:"metrics,omitempty"`

	Label                string `json:"label,omitempty"`
	Note                 string `json:"note,omitempty"`
	SemanticAnalysisMode string `json:"semanticAnalysisMode,omitempty"`
	ResearchQuestion     string `json:"researchQuestion,omitempty"`
	ReasoningProfile     string `json:"reasoningProfile,omitempty"`
	// ExecutionSnapshot and InputSnapshot are absent for runs recorded before
	// snapshots existed. A missing snapshot means "not recorded".
	ExecutionSnapshot    json.RawMessage `json:"executionSnapshot,omitempty"`
	InputSnapshot        json.RawMessage `json:"inputSnapshot,omitempty"`
	ExecutionFingerprint string          `json:"executionFingerprint,omitempty"`
	InputFingerprint     string          `json:"inputFingerprint,omitempty"`
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
	if a.Status == domain.AnalysisCompleted && json.Valid([]byte(a.Metrics)) {
		dto.Metrics = json.RawMessage(a.Metrics)
	}
	dto.Label, dto.Note, dto.SemanticAnalysisMode, dto.ResearchQuestion = a.Label, a.Note, string(a.SemanticAnalysisMode), a.ResearchQuestion
	if json.Valid([]byte(a.ExecutionSnapshot)) {
		dto.ExecutionSnapshot = json.RawMessage(a.ExecutionSnapshot)
		dto.ReasoningProfile = string(service.AnalysisReasoningProfile(a))
	}
	if json.Valid([]byte(a.InputSnapshot)) {
		dto.InputSnapshot = json.RawMessage(a.InputSnapshot)
	}
	dto.ExecutionFingerprint, dto.InputFingerprint = a.ExecutionFingerprint, a.InputFingerprint
	return dto
}

func (h *Handler) CreateAnalysis(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}
	// The body is optional; an empty request starts an unlabelled run.
	var req struct {
		Label                string                  `json:"label"`
		Note                 string                  `json:"note"`
		SemanticAnalysisMode domain.AnalysisMode     `json:"semanticAnalysisMode"`
		ResearchQuestion     string                  `json:"researchQuestion"`
		ReasoningProfile     domain.ReasoningProfile `json:"reasoningProfile"`
	}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	if req.SemanticAnalysisMode != "" && !req.SemanticAnalysisMode.Valid() {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid semanticAnalysisMode %q", req.SemanticAnalysisMode))
		return
	}
	if req.ReasoningProfile != "" && !req.ReasoningProfile.Valid() {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid reasoningProfile %q", req.ReasoningProfile))
		return
	}
	a, err := h.JobManager.Enqueue(r.Context(), service.EnqueueRequest{
		ProjectID: projectID, Label: strings.TrimSpace(req.Label), Note: strings.TrimSpace(req.Note), SemanticAnalysisMode: req.SemanticAnalysisMode, ResearchQuestion: strings.TrimSpace(req.ResearchQuestion),
		ReasoningProfile: req.ReasoningProfile,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, toAnalysisDTO(a))
}

func (h *Handler) GetAnalysis(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "analysisID")
	a, err := h.App.GetAnalysis(r.Context(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
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
	list, err := h.App.ListAnalyses(r.Context(), projectID)
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
	if _, err := h.App.GetAnalysis(r.Context(), id); err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
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
