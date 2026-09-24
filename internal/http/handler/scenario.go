package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/domain"
	"insight-lab/internal/usecase"
)

func scenarioStatus(err error) int {
	switch {
	case errors.Is(err, usecase.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, usecase.ErrScenarioInvalid):
		return http.StatusBadRequest
	case errors.Is(err, usecase.ErrScenarioStorageUnavailable):
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}

func (h *Handler) GetScenarioAnalysis(w http.ResponseWriter, r *http.Request) {
	analysis, err := h.App.GetScenarioAnalysis(r.Context(), chi.URLParam(r, "runID"))
	if err != nil {
		writeError(w, scenarioStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, analysis)
}

// CreateScenarioSet stores a curated scenario set. id, version, researchRunId
// and createdAt are assigned by the engine.
func (h *Handler) CreateScenarioSet(w http.ResponseWriter, r *http.Request) {
	var set domain.ScenarioSet
	if json.NewDecoder(r.Body).Decode(&set) != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	set.ResearchRunID = chi.URLParam(r, "runID")
	stored, err := h.App.CreateScenarioSet(r.Context(), set)
	if err != nil {
		writeError(w, scenarioStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, stored)
}

func (h *Handler) ScaffoldScenarioSet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IterationID string                  `json:"iterationId"`
		Baseline    domain.ScenarioBaseline `json:"baseline"`
		Horizon     domain.ScenarioHorizon  `json:"horizon"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	res, err := h.App.ScaffoldScenarios(r.Context(), usecase.ScaffoldScenariosInput{RunID: chi.URLParam(r, "runID"),
		IterationID: req.IterationID, Baseline: req.Baseline, Horizon: req.Horizon})
	if err != nil {
		writeError(w, scenarioStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (h *Handler) EvaluateScenarios(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IterationID      string                        `json:"iterationId"`
		Observations     []domain.IndicatorObservation `json:"observations"`
		AssumptionChecks []domain.AssumptionCheck      `json:"assumptionChecks"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	eval, err := h.App.EvaluateScenarios(r.Context(), usecase.EvaluateScenariosInput{RunID: chi.URLParam(r, "runID"),
		ScenarioSetID: chi.URLParam(r, "scenarioSetID"), IterationID: req.IterationID,
		Input: domain.NewObservationInput{Observations: req.Observations, AssumptionChecks: req.AssumptionChecks}})
	if err != nil {
		writeError(w, scenarioStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, eval)
}
