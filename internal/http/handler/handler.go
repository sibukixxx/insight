package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"insight-lab/internal/repository"
	"insight-lab/internal/service"
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
	Projects  repository.ProjectRepository
	Documents repository.DocumentRepository
	Demo      *service.DemoLoader
	Build     BuildInfo
}

func New(projects repository.ProjectRepository, documents repository.DocumentRepository, demo *service.DemoLoader, build BuildInfo) *Handler {
	return &Handler{Projects: projects, Documents: documents, Demo: demo, Build: build}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}
