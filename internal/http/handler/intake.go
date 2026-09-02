package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/domain"
	"insight-lab/internal/usecase"
)

type intakePreviewRequest struct {
	Source       string            `json:"source"`
	Provenance   string            `json:"provenance"`
	Content      string            `json:"content"`
	SpeakerRoles map[string]string `json:"speakerRoles"`
}

type turnDTO struct {
	Speaker string `json:"speaker"`
	Role    string `json:"role"`
	Guessed bool   `json:"guessed"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Text    string `json:"text"`
}

type speakerSummaryDTO struct {
	Label   string `json:"label"`
	Role    string `json:"role"`
	Guessed bool   `json:"guessed"`
	Turns   int    `json:"turns"`
	Chars   int    `json:"chars"`
}

type intakePreviewDTO struct {
	Detected      bool                `json:"detected"`
	Provenance    string              `json:"provenance"`
	Turns         []turnDTO           `json:"turns"`
	Speakers      []speakerSummaryDTO `json:"speakers"`
	Spans         []spanDTO           `json:"spans"`
	Warnings      []string            `json:"warnings"`
	TotalChars    int                 `json:"totalChars"`
	CustomerChars int                 `json:"customerChars"`
	ExcludedChars int                 `json:"excludedChars"`
}

func toSpeakerRoles(m map[string]string) map[string]domain.SpeakerRole {
	out := make(map[string]domain.SpeakerRole, len(m))
	for label, role := range m {
		out[label] = domain.SpeakerRole(role)
	}
	return out
}

// PreviewIntake shows how a paste would be split into speakers before it
// is stored, so the user can correct roles and see exactly which text
// will be treated as the customer's voice.
func (h *Handler) PreviewIntake(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}
	var req intakePreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	preview, err := h.App.PreviewIntake(r.Context(), usecase.PreviewIntakeInput{
		ProjectID: projectID, Source: domain.SourceType(req.Source), Provenance: domain.Provenance(req.Provenance),
		Content: req.Content, SpeakerRoles: toSpeakerRoles(req.SpeakerRoles),
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dto := intakePreviewDTO{
		Detected: preview.Transcript.Detected, Provenance: string(preview.Provenance),
		Turns: []turnDTO{}, Speakers: []speakerSummaryDTO{}, Spans: toSpanDTOs(preview.Spans),
		Warnings:   preview.Transcript.Warnings,
		TotalChars: preview.TotalChars, CustomerChars: preview.CustomerChars, ExcludedChars: preview.ExcludedChars,
	}
	if dto.Warnings == nil {
		dto.Warnings = []string{}
	}
	for _, t := range preview.Transcript.Turns {
		dto.Turns = append(dto.Turns, turnDTO{Speaker: t.Speaker, Role: string(t.Role), Guessed: t.Guessed, Start: t.Start, End: t.End, Text: t.Text})
	}
	for _, s := range preview.Transcript.Speakers {
		dto.Speakers = append(dto.Speakers, speakerSummaryDTO{Label: s.Label, Role: string(s.Role), Guessed: s.Guessed, Turns: s.Turns, Chars: s.Chars})
	}
	writeJSON(w, http.StatusOK, dto)
}

type intakeProfileDTO struct {
	SpeakerRoles  map[string]string     `json:"speakerRoles"`
	MaskTerms     []string              `json:"maskTerms"`
	ColumnMapping *domain.ColumnMapping `json:"columnMapping,omitempty"`
}

func toIntakeProfileDTO(p *domain.IntakeProfile) intakeProfileDTO {
	dto := intakeProfileDTO{SpeakerRoles: map[string]string{}, MaskTerms: []string{}, ColumnMapping: p.ColumnMapping}
	for label, role := range p.SpeakerRoles {
		dto.SpeakerRoles[label] = string(role)
	}
	if p.MaskTerms != nil {
		dto.MaskTerms = p.MaskTerms
	}
	return dto
}

func (h *Handler) GetIntakeProfile(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	profile, err := h.App.GetIntakeProfile(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	writeJSON(w, http.StatusOK, toIntakeProfileDTO(profile))
}

func (h *Handler) UpdateIntakeProfile(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	if !h.requireProject(w, r, projectID) {
		return
	}
	var req intakeProfileDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	profile := domain.IntakeProfile{SpeakerRoles: toSpeakerRoles(req.SpeakerRoles), MaskTerms: req.MaskTerms, ColumnMapping: req.ColumnMapping}
	if err := h.App.UpdateIntakeProfile(r.Context(), projectID, profile); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toIntakeProfileDTO(&profile))
}
