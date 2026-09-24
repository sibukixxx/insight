package public

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/publicengine"
)

// registerTriageRoutes mounts the Data Triage operations (#92).
func registerTriageRoutes(r chi.Router, h *handler) {
	r.Post("/subjects/{subjectID}/dataset-profiles", h.createDatasetProfile)
	r.Get("/subjects/{subjectID}/dataset-profiles/{profileID}", h.getDatasetProfile)
	r.Post("/subjects/{subjectID}/dataset-profiles/{profileID}/triage", h.triage)
	r.Get("/subjects/{subjectID}/dataset-profiles/{profileID}/selection-plans", h.listSelectionPlans)
	r.Get("/selection-plans/{planID}", h.getSelectionPlan)
	r.Post("/selection-plans/{planID}/revisions", h.reviseSelectionPlan)
}

func (h *handler) createDatasetProfile(w http.ResponseWriter, r *http.Request) {
	var req publicengine.CreateDatasetProfileRequest
	if decode(w, r, &req) {
		writeResult(w)(h.engine.CreateDatasetProfile(r.Context(), chi.URLParam(r, "subjectID"), req))
	}
}

func (h *handler) getDatasetProfile(w http.ResponseWriter, r *http.Request) {
	p, err := h.engine.GetDatasetProfile(r.Context(), chi.URLParam(r, "subjectID"), chi.URLParam(r, "profileID"))
	writeValue(w, p, err)
}

func (h *handler) triage(w http.ResponseWriter, r *http.Request) {
	var req publicengine.TriageRequest
	if decode(w, r, &req) {
		writeResult(w)(h.engine.Triage(r.Context(), chi.URLParam(r, "subjectID"), chi.URLParam(r, "profileID"), req))
	}
}

func (h *handler) listSelectionPlans(w http.ResponseWriter, r *http.Request) {
	list, err := h.engine.ListSelectionPlans(r.Context(), chi.URLParam(r, "subjectID"), chi.URLParam(r, "profileID"))
	writeValue(w, list, err)
}

func (h *handler) getSelectionPlan(w http.ResponseWriter, r *http.Request) {
	p, err := h.engine.GetSelectionPlan(r.Context(), chi.URLParam(r, "planID"))
	writeValue(w, p, err)
}

func (h *handler) reviseSelectionPlan(w http.ResponseWriter, r *http.Request) {
	var req publicengine.ReviseSelectionPlanRequest
	if decode(w, r, &req) {
		writeResult(w)(h.engine.ReviseSelectionPlan(r.Context(), chi.URLParam(r, "planID"), req))
	}
}
