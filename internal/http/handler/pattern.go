package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/usecase"
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

// patternDTO carries both kinds of "noticing": kind == "repetition" is a
// behavior seen across several people; kind == "deviation" is a trace of
// desire - a behavior that broke a common-sense expectation, with the
// expectation and the actual behavior (description) shown side by side so
// the reader can judge the gap themselves.
type patternDTO struct {
	ID            string                  `json:"id"`
	Kind          string                  `json:"kind"`
	Title         string                  `json:"title"`
	Description   string                  `json:"description,omitempty"`
	Expectation   string                  `json:"expectation,omitempty"`
	DeviationType string                  `json:"deviationType,omitempty"`
	Observations  []patternObservationDTO `json:"observations"`
	CreatedAt     string                  `json:"createdAt"`
}

// toPatternDTOs resolves each pattern's ObservationIDs into full
// Observation rows (quote, offsets, behavior) in one batched lookup, so
// the "reasoning trail" UI can render the actual quotes a pattern was
// built from without an extra round trip per pattern.
func toPatternDTOs(patterns []usecase.PatternDetail) []patternDTO {
	out := make([]patternDTO, 0, len(patterns))
	for _, detail := range patterns {
		p := detail.Pattern
		dto := patternDTO{
			ID: p.ID, Kind: string(p.Kind), Title: p.Title, Description: p.Description,
			Expectation: p.Expectation, DeviationType: string(p.DeviationType),
			CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339),
		}
		for _, o := range detail.Observations {
			dto.Observations = append(dto.Observations, patternObservationDTO{
				ID: o.ID, DocumentID: o.DocumentID, Quote: o.Quote, Behavior: o.Behavior,
				Topic: o.Topic, StartOffset: o.StartOffset, EndOffset: o.EndOffset,
			})
		}
		out = append(out, dto)
	}
	return out
}

func (h *Handler) ListPatterns(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}
	patterns, err := h.App.ListPatterns(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toPatternDTOs(patterns))
}
