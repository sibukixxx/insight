package handler

import (
	"net/http"

	"insight-lab/internal/execution"
)

// runtimeModes lists the placements this binary supports: DISTRIBUTED
// only when a broker adapter is compiled in.
func runtimeModes() []execution.RuntimeMode {
	modes := []execution.RuntimeMode{execution.RuntimeLocal, execution.RuntimeProcess}
	if len(execution.BrokerSchemes()) > 0 {
		modes = append(modes, execution.RuntimeDistributed)
	}
	return modes
}

func (h *Handler) runtimeMode() execution.RuntimeMode {
	if h.RuntimeMode == "" {
		return execution.RuntimeLocal
	}
	return h.RuntimeMode
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"demoBuild":  h.Build.DemoBuild,
		"clientName": h.Build.ClientName,
		"engine":     h.Build.Engine,
		"capabilities": map[string]any{
			"largeIngest": h.Ingest.Capability(),
			"runtime":     map[string]any{"mode": h.runtimeMode(), "modes": runtimeModes(), "brokers": execution.BrokerSchemes()},
		},
	})
}
