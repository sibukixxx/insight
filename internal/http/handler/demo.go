package handler

import (
	"net/http"

	"insight-lab/internal/sampledata"
)

// CreateDemoProject loads the fixed demo project on demand (the UI's
// "Try the demo" button), independent of whether the process was started
// with --demo. On a delivery build this always fails: no sample data was
// compiled in, so there is nothing to load.
func (h *Handler) CreateDemoProject(w http.ResponseWriter, r *http.Request) {
	if !sampledata.Embedded {
		writeError(w, http.StatusConflict, "this build does not include demo data; start a demo build instead")
		return
	}
	p, err := h.Demo.Ensure(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toProjectDTO(p))
}
