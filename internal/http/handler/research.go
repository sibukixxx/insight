package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/domain"
	"insight-lab/internal/usecase"
)

func (h *Handler) ListResearchRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := h.App.ListResearchRuns(r.Context(), chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (h *Handler) CreateResearchRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Question        string   `json:"question"`
		InputReferences []string `json:"inputReferences"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	run, err := h.App.CreateResearchRun(r.Context(), usecase.CreateResearchRunInput{ProjectID: chi.URLParam(r, "projectID"), Question: req.Question, InputReferences: req.InputReferences})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

func (h *Handler) GetResearchRun(w http.ResponseWriter, r *http.Request) {
	run, err := h.App.GetResearchRun(r.Context(), chi.URLParam(r, "runID"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, usecase.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *Handler) GetResearchIteration(w http.ResponseWriter, r *http.Request) {
	it, err := h.App.GetResearchIteration(r.Context(), chi.URLParam(r, "runID"), chi.URLParam(r, "iterationID"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, usecase.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, it)
}

func (h *Handler) AppendResearchIteration(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Question        string   `json:"question"`
		InputReferences []string `json:"inputReferences"`
		AddedEvidence   []string `json:"addedEvidence"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	it, err := h.App.AppendResearchIteration(r.Context(), usecase.AppendResearchIterationInput{RunID: chi.URLParam(r, "runID"), Question: req.Question, InputReferences: req.InputReferences, AddedEvidence: req.AddedEvidence})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, it)
}

func (h *Handler) SaveHumanEvaluation(w http.ResponseWriter, r *http.Request) {
	evaluation := &domain.HumanEvaluation{ResearchRunID: chi.URLParam(r, "runID"), IterationID: chi.URLParam(r, "iterationID")}
	if json.NewDecoder(r.Body).Decode(evaluation) != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	evaluation.ResearchRunID, evaluation.IterationID = chi.URLParam(r, "runID"), chi.URLParam(r, "iterationID")
	if err := h.App.SaveHumanEvaluation(r.Context(), evaluation); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, evaluation)
}

func (h *Handler) ApplyResearchHumanOverride(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Readiness  domain.DecisionReadiness  `json:"readiness"`
		StopReason domain.ResearchStopReason `json:"stopReason"`
		Note       string                    `json:"note"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	iteration, err := h.App.ApplyResearchHumanOverride(r.Context(), usecase.ApplyResearchHumanOverrideInput{
		RunID: chi.URLParam(r, "runID"), IterationID: chi.URLParam(r, "iterationID"),
		Readiness: req.Readiness, StopReason: req.StopReason, Note: req.Note,
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, usecase.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, iteration)
}

func (h *Handler) GetHumanHandoff(w http.ResponseWriter, r *http.Request) {
	handoff, err := h.App.GetHumanHandoff(r.Context(), chi.URLParam(r, "runID"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, usecase.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, handoff)
}

func (h *Handler) GetHumanEvaluation(w http.ResponseWriter, r *http.Request) {
	evaluation, err := h.App.GetHumanEvaluation(r.Context(), chi.URLParam(r, "runID"), chi.URLParam(r, "iterationID"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, usecase.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, evaluation)
}
