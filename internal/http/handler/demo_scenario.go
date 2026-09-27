package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/sampledata"
	"insight-lab/internal/service"
)

type demoScenarioDTO struct {
	sampledata.Scenario
	ProjectID string `json:"projectId"`
}

// ListDemoScenarios serves the bundled sample scenarios with their
// provenance. A delivery build bundles none and answers an empty list.
func (h *Handler) ListDemoScenarios(w http.ResponseWriter, r *http.Request) {
	scenarios, err := sampledata.Scenarios()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]demoScenarioDTO, 0, len(scenarios))
	for _, s := range scenarios {
		out = append(out, demoScenarioDTO{Scenario: s, ProjectID: service.ScenarioProjectID(s.ID)})
	}
	writeJSON(w, http.StatusOK, out)
}

// DemoScenarioInput serves a scenario's input CSV byte for byte, for the
// download link and for the "use this CSV" preview.
func (h *Handler) DemoScenarioInput(w http.ResponseWriter, r *http.Request) {
	if !sampledata.Embedded {
		writeError(w, http.StatusConflict, "this build does not include demo data; start a demo build instead")
		return
	}
	id := chi.URLParam(r, "scenarioID")
	s, err := sampledata.FindScenario(id)
	if err != nil {
		writeScenarioError(w, err)
		return
	}
	data, err := sampledata.ScenarioInput(id)
	if err != nil {
		writeScenarioError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="insight-sample-`+s.ID+`.csv"`)
	_, _ = w.Write(data)
}

// CreateDemoScenarioProject opens (creating once) the empty project of a
// sample scenario.
func (h *Handler) CreateDemoScenarioProject(w http.ResponseWriter, r *http.Request) {
	if !sampledata.Embedded {
		writeError(w, http.StatusConflict, "this build does not include demo data; start a demo build instead")
		return
	}
	p, err := h.Demo.EnsureScenario(r.Context(), chi.URLParam(r, "scenarioID"))
	if err != nil {
		writeScenarioError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toProjectDTO(p))
}

func writeScenarioError(w http.ResponseWriter, err error) {
	if errors.Is(err, sampledata.ErrUnknownScenario) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}
