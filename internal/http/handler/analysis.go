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
	"insight-lab/internal/execution"
	"insight-lab/internal/repository"
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
	OutputLocale         string `json:"outputLocale,omitempty"`
	// ExecutionSnapshot and InputSnapshot are absent for runs recorded before
	// snapshots existed. A missing snapshot means "not recorded".
	ExecutionSnapshot    json.RawMessage `json:"executionSnapshot,omitempty"`
	InputSnapshot        json.RawMessage `json:"inputSnapshot,omitempty"`
	ExecutionFingerprint string          `json:"executionFingerprint,omitempty"`
	InputFingerprint     string          `json:"inputFingerprint,omitempty"`

	// Lifecycle is the coordinator state (#133): QUEUED, RUNNING,
	// CANCEL_REQUESTED, SUCCEEDED, FAILED, CANCELLED or INTERRUPTED. Status
	// keeps the four legacy values for existing readers.
	Lifecycle         string  `json:"lifecycle"`
	FailureCode       string  `json:"failureCode,omitempty"`
	CancelRequestedAt *string `json:"cancelRequestedAt,omitempty"`
	RetryOf           string  `json:"retryOf,omitempty"`
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
	dto.Label, dto.Note, dto.SemanticAnalysisMode, dto.ResearchQuestion, dto.ReasoningProfile = a.Label, a.Note, string(a.SemanticAnalysisMode), a.ResearchQuestion, string(a.ReasoningProfile.Normalize())
	dto.OutputLocale = string(a.OutputLocale)
	if json.Valid([]byte(a.ExecutionSnapshot)) {
		dto.ExecutionSnapshot = json.RawMessage(a.ExecutionSnapshot)
	}
	if json.Valid([]byte(a.InputSnapshot)) {
		dto.InputSnapshot = json.RawMessage(a.InputSnapshot)
	}
	dto.ExecutionFingerprint, dto.InputFingerprint = a.ExecutionFingerprint, a.InputFingerprint
	dto.Lifecycle, dto.FailureCode, dto.RetryOf = string(a.Lifecycle()), string(a.FailureCode), a.RetryOf
	if a.CancelRequestedAt != nil {
		s := a.CancelRequestedAt.UTC().Format(time.RFC3339)
		dto.CancelRequestedAt = &s
	}
	return dto
}

// writeAnalysisLifecycleError maps JobManager lifecycle errors to statuses.
func writeAnalysisLifecycleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		writeError(w, http.StatusNotFound, "analysis not found")
	case errors.Is(err, service.ErrAnalysisQueueFull):
		w.Header().Set("Retry-After", "5")
		writeError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, service.ErrAnalysisFinished), errors.Is(err, service.ErrAnalysisNotRetryable):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrModelBindingUnavailable), errors.Is(err, execution.ErrProfileUnavailable),
		errors.Is(err, service.ErrExplorationNeedsModel), errors.Is(err, service.ErrExplorationHasEvidence):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrExplorationNeedsQuestion):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

// CancelAnalysis cancels a queued or running analysis and answers 202 with
// its state: CANCELLED for a queued run, CANCEL_REQUESTED for a running one
// until it stops. A finished analysis answers 409.
func (h *Handler) CancelAnalysis(w http.ResponseWriter, r *http.Request) {
	a, err := h.JobManager.Cancel(r.Context(), chi.URLParam(r, "analysisID"))
	if err != nil {
		writeAnalysisLifecycleError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, toAnalysisDTO(a))
}

// RetryAnalysis enqueues a failed analysis again as a new run (202), or
// returns the retry that already exists (200). Only failed analyses can be
// retried (409 otherwise).
func (h *Handler) RetryAnalysis(w http.ResponseWriter, r *http.Request) {
	a, created, err := h.JobManager.Retry(r.Context(), chi.URLParam(r, "analysisID"))
	if err != nil {
		writeAnalysisLifecycleError(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusAccepted
	}
	writeJSON(w, status, toAnalysisDTO(a))
}

func (h *Handler) CreateAnalysis(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}
	// The body is optional; an empty request starts an unlabelled run.
	var req struct {
		Label                string              `json:"label"`
		Note                 string              `json:"note"`
		SemanticAnalysisMode domain.AnalysisMode `json:"semanticAnalysisMode"`
		ResearchQuestion     string                  `json:"researchQuestion"`
		ReasoningProfile     domain.ReasoningProfile `json:"reasoningProfile"`
		OutputLocale         domain.OutputLocale     `json:"outputLocale"`
		// Exploratory asks for a question-only exploration (#158): it needs
		// a researchQuestion, a configured model and a project with no
		// documents. Reference API only.
		Exploratory bool `json:"exploratory"`
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
	if !req.OutputLocale.Valid() {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported outputLocale %q", req.OutputLocale))
		return
	}
	a, err := h.JobManager.Enqueue(r.Context(), service.EnqueueRequest{
		ProjectID: projectID, Label: strings.TrimSpace(req.Label), Note: strings.TrimSpace(req.Note), SemanticAnalysisMode: req.SemanticAnalysisMode, ResearchQuestion: strings.TrimSpace(req.ResearchQuestion), ReasoningProfile: req.ReasoningProfile.Normalize(), OutputLocale: req.OutputLocale,
		Exploratory: req.Exploratory,
	})
	if err != nil {
		writeAnalysisLifecycleError(w, err)
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
// docs/detailed-design.md §9). The stream opens with a "status" event (or,
// for a finished analysis, its terminal event) read from the database after
// subscribing, so a client that reconnects never waits for an event that
// was already sent. GET /api/analysis/{id} stays the state of record.
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

	current, err := h.App.GetAnalysis(r.Context(), id)
	if err != nil {
		return
	}
	if ev, done := service.TerminalEvent(current); done {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Event, ev.Data)
		flusher.Flush()
		return
	}
	status, _ := json.Marshal(map[string]any{"step": current.CurrentStep, "progress": current.Progress, "lifecycle": current.Lifecycle()})
	fmt.Fprintf(w, "event: status\ndata: %s\n\n", status)
	flusher.Flush()

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
