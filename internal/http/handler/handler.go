package handler

import (
	"encoding/json"
	"net/http"

	"insight-lab/internal/llm"
	"insight-lab/internal/service"
	"insight-lab/internal/usecase"
)

// BuildInfo describes how this binary was built: whether the demo dataset
// is embedded (a "demo" tagged build) or not (a "delivery" build meant for
// a customer, carrying none of the sample data), and an optional client
// name stamped onto delivery builds for a confidentiality banner.
type BuildInfo struct {
	DemoBuild  bool
	ClientName string
}

type Handler struct {
	App *usecase.Application

	Demo         *service.DemoLoader
	Settings     *service.SettingsStore
	JobManager   *service.JobManager
	NewLLMClient func(service.Settings) llm.Client

	Build BuildInfo
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
