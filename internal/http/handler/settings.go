package handler

import (
	"encoding/json"
	"net/http"

	"insight-lab/internal/service"
)

type settingsDTO struct {
	Model        string `json:"model"`
	BaseURL      string `json:"baseUrl"`
	MaskedAPIKey string `json:"maskedApiKey"`
	HasAPIKey    bool   `json:"hasApiKey"`
	Configured   bool   `json:"configured"`
}

func toSettingsDTO(s service.Settings) settingsDTO {
	return settingsDTO{
		Model: s.Model, BaseURL: s.BaseURL,
		MaskedAPIKey: s.MaskedAPIKey(), HasAPIKey: s.APIKey != "",
		Configured: s.Configured(),
	}
}

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, toSettingsDTO(h.Settings.Get()))
}

type updateSettingsRequest struct {
	APIKey  string `json:"apiKey"`
	Model   string `json:"model"`
	BaseURL string `json:"baseUrl"`
}

func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req updateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	updated := h.Settings.Update(service.Settings{APIKey: req.APIKey, Model: req.Model, BaseURL: req.BaseURL})
	writeJSON(w, http.StatusOK, toSettingsDTO(updated))
}

func (h *Handler) TestSettings(w http.ResponseWriter, r *http.Request) {
	settings := h.Settings.Get()
	if !settings.Configured() {
		writeError(w, http.StatusBadRequest, "Base URLとModelを設定してください")
		return
	}
	client := h.NewLLMClient(settings)
	result, err := service.ProbeConnection(r.Context(), client)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"mode": string(result.Mode)})
}
