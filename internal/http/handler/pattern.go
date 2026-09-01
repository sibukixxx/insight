package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/domain"
)

type patternObservationDTO struct {
	ID          string `json:"id"`
	DocumentID  string `json:"documentId"`
	Quote       string `json:"quote"`
	Behavior    string `json:"behavior"`
	Topic       string `json:"topic,omitempty"`
	StartOffset int    `json:"startOffset"`
	EndOffset   int    `json:"endOffset"`
}

type patternDTO struct {
	ID           string                  `json:"id"`
	Title        string                  `json:"title"`
	Description  string                  `json:"description,omitempty"`
	Observations []patternObservationDTO `json:"observations"`
	CreatedAt    string                  `json:"createdAt"`
}

// toPatternDTOs resolves each pattern's ObservationIDs into full
// Observation rows (quote, offsets, behavior) in one batched lookup, so
// the "reasoning trail" UI can render the actual quotes a pattern was
// built from without an extra round trip per pattern.
func (h *Handler) toPatternDTOs(ctx context.Context, patterns []*domain.Pattern) ([]patternDTO, error) {
	var allIDs []string
	for _, p := range patterns {
		allIDs = append(allIDs, p.ObservationIDs...)
	}
	obs, err := h.Observations.ListByIDs(ctx, allIDs)
	if err != nil {
		return nil, err
	}
	obsByID := make(map[string]*domain.Observation, len(obs))
	for _, o := range obs {
		obsByID[o.ID] = o
	}

	out := make([]patternDTO, 0, len(patterns))
	for _, p := range patterns {
		dto := patternDTO{
			ID: p.ID, Title: p.Title, Description: p.Description,
			CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339),
		}
		for _, oid := range p.ObservationIDs {
			o, ok := obsByID[oid]
			if !ok {
				continue
			}
			dto.Observations = append(dto.Observations, patternObservationDTO{
				ID: o.ID, DocumentID: o.DocumentID, Quote: o.Quote, Behavior: o.Behavior,
				Topic: o.Topic, StartOffset: o.StartOffset, EndOffset: o.EndOffset,
			})
		}
		out = append(out, dto)
	}
	return out, nil
}

func (h *Handler) ListPatterns(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}
	patterns, err := h.Patterns.ListByProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	dtos, err := h.toPatternDTOs(r.Context(), patterns)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dtos)
}
