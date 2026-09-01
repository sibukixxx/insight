package handler

import "net/http"

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"demoBuild":  h.Build.DemoBuild,
		"clientName": h.Build.ClientName,
	})
}
