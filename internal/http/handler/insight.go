package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/domain"
	"insight-lab/internal/usecase"
)

type insightDTO struct {
	ID                        string  `json:"id"`
	ProjectID                 string  `json:"projectId"`
	Title                     string  `json:"title"`
	Observation               string  `json:"observation"`
	StatedNeed                string  `json:"statedNeed"`
	LatentNeed                string  `json:"latentNeed"`
	JTBD                      string  `json:"jtbd"`
	Expectation               string  `json:"expectation"`
	SurprisingFact            string  `json:"surprisingFact"`
	Rationale                 string  `json:"rationale"`
	Interpretation            string  `json:"interpretation"`
	AlternativeInterpretation string  `json:"alternativeInterpretation"`
	ProductOpportunity        string  `json:"productOpportunity"`
	MonetizationAngle         string  `json:"monetizationAngle"`
	Confidence                float64 `json:"confidence"`
	// QualityFlags are app-side warnings (never model self-assessment)
	// that this insight may not meet the definition of an insight; always
	// an array so the client never has to null-check it.
	QualityFlags []qualityFlagDTO `json:"qualityFlags"`
	CreatedAt    string           `json:"createdAt"`
}

type qualityFlagDTO struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

func toInsightDTO(i *domain.Insight) insightDTO {
	flags := make([]qualityFlagDTO, 0, len(i.QualityFlags))
	for _, f := range i.QualityFlags {
		flags = append(flags, qualityFlagDTO{Code: string(f.Code), Detail: f.Detail})
	}
	return insightDTO{
		ID: i.ID, ProjectID: i.ProjectID, Title: i.Title, Observation: i.Observation,
		StatedNeed: i.StatedNeed, LatentNeed: i.LatentNeed, JTBD: i.JTBD,
		Expectation: i.Expectation, SurprisingFact: i.SurprisingFact, Rationale: i.Rationale,
		Interpretation: i.Interpretation, AlternativeInterpretation: i.AlternativeInterpretation,
		ProductOpportunity: i.ProductOpportunity, MonetizationAngle: i.MonetizationAngle, Confidence: i.Confidence,
		QualityFlags: flags,
		CreatedAt:    i.CreatedAt.UTC().Format(time.RFC3339),
	}
}

type evidenceDTO struct {
	ID             string  `json:"id"`
	DocumentID     string  `json:"documentId"`
	ObservationID  *string `json:"observationId,omitempty"`
	Quote          string  `json:"quote"`
	Type           string  `json:"type"`
	RelevanceScore float64 `json:"relevanceScore"`
	StartOffset    int     `json:"startOffset"`
	EndOffset      int     `json:"endOffset"`
}

func toEvidenceDTOs(evidence []*domain.Evidence) []evidenceDTO {
	out := make([]evidenceDTO, 0, len(evidence))
	for _, e := range evidence {
		out = append(out, evidenceDTO{
			ID: e.ID, DocumentID: e.DocumentID, ObservationID: e.ObservationID,
			Quote: e.Quote, Type: string(e.Type), RelevanceScore: e.RelevanceScore,
			StartOffset: e.StartOffset, EndOffset: e.EndOffset,
		})
	}
	return out
}

func (h *Handler) ListInsights(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}
	list, err := h.App.ListInsights(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]insightDTO, 0, len(list))
	for _, i := range list {
		out = append(out, toInsightDTO(i))
	}
	writeJSON(w, http.StatusOK, out)
}

type insightDetailDTO struct {
	insightDTO
	Evidence []evidenceDTO `json:"evidence"`
	Patterns []patternDTO  `json:"patterns"`
}

func (h *Handler) GetInsight(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "insightID")
	detail, err := h.App.GetInsight(r.Context(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			writeError(w, http.StatusNotFound, "insight not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, insightDetailDTO{
		insightDTO: toInsightDTO(detail.Insight), Evidence: toEvidenceDTOs(detail.Evidence), Patterns: toPatternDTOs(detail.Patterns),
	})
}

func (h *Handler) GetInsightEvidence(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "insightID")
	evidence, err := h.App.GetInsightEvidence(r.Context(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			writeError(w, http.StatusNotFound, "insight not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toEvidenceDTOs(evidence))
}
