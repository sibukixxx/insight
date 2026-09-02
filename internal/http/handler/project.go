package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/domain"
	"insight-lab/internal/usecase"
)

type projectDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

func toProjectDTO(p *domain.Project) projectDTO {
	return projectDTO{ID: p.ID, Name: p.Name, CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339)}
}

func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.App.ListProjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]projectDTO, 0, len(projects))
	for _, p := range projects {
		out = append(out, toProjectDTO(p))
	}
	writeJSON(w, http.StatusOK, out)
}

type createProjectRequest struct {
	Name string `json:"name"`
}

func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	p, err := h.App.CreateProject(r.Context(), req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toProjectDTO(p))
}

func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "projectID")
	p, err := h.App.GetProject(r.Context(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toProjectDTO(p))
}

func (h *Handler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "projectID")
	if err := h.App.DeleteProject(r.Context(), id); err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
