// Package public is the HTTP/JSON transport of the Public Engine Contract v1
// (contracts/public-engine/v1). It only decodes, delegates to
// internal/publicengine and encodes; every contract rule lives there.
package public

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"insight-lab/internal/publicengine"
)

// Router mounts the contract operations; the caller mounts it at
// /api/public/v1.
func Router(engine *publicengine.Engine) http.Handler {
	h := &handler{engine: engine}
	r := chi.NewRouter()
	r.Get("/engine", h.getEngine)
	r.Post("/subjects", h.createSubject)
	r.Post("/subjects/{subjectID}/evidence", h.addEvidence)
	r.Post("/subjects/{subjectID}/analyses", h.startAnalysis)
	r.Get("/subjects/{subjectID}/analyses/{analysisID}", h.getAnalysis)
	r.Get("/subjects/{subjectID}/analyses/{analysisID}/results", h.getAnalysisResults)
	r.Post("/subjects/{subjectID}/research-runs", h.createResearchRun)
	r.Post("/research-runs/{researchRunID}/iterations", h.appendIteration)
	r.Get("/research-runs/{researchRunID}", h.getResearchRun)
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, &publicengine.Error{Code: publicengine.CodeNotFound, Message: "no such public operation"})
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, &publicengine.Error{Code: publicengine.CodeInvalidRequest, Message: "method not allowed"})
	})
	return r
}

type handler struct{ engine *publicengine.Engine }

func (h *handler) getEngine(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.engine.Engine())
}

func (h *handler) createSubject(w http.ResponseWriter, r *http.Request) {
	var req publicengine.CreateSubjectRequest
	if decode(w, r, &req) {
		writeResult(w)(h.engine.CreateSubject(r.Context(), req))
	}
}

func (h *handler) addEvidence(w http.ResponseWriter, r *http.Request) {
	var req publicengine.AddEvidenceRequest
	if decode(w, r, &req) {
		writeResult(w)(h.engine.AddEvidence(r.Context(), chi.URLParam(r, "subjectID"), req))
	}
}

func (h *handler) startAnalysis(w http.ResponseWriter, r *http.Request) {
	var req publicengine.StartAnalysisRequest
	if decode(w, r, &req) {
		writeResult(w)(h.engine.StartAnalysis(r.Context(), chi.URLParam(r, "subjectID"), req))
	}
}

func (h *handler) getAnalysis(w http.ResponseWriter, r *http.Request) {
	run, err := h.engine.GetAnalysis(r.Context(), chi.URLParam(r, "subjectID"), chi.URLParam(r, "analysisID"))
	writeValue(w, run, err)
}

func (h *handler) getAnalysisResults(w http.ResponseWriter, r *http.Request) {
	results, err := h.engine.GetAnalysisResults(r.Context(), chi.URLParam(r, "subjectID"), chi.URLParam(r, "analysisID"))
	writeValue(w, results, err)
}

func (h *handler) createResearchRun(w http.ResponseWriter, r *http.Request) {
	var req publicengine.CreateResearchRunRequest
	if decode(w, r, &req) {
		writeResult(w)(h.engine.CreateResearchRun(r.Context(), chi.URLParam(r, "subjectID"), req))
	}
}

func (h *handler) appendIteration(w http.ResponseWriter, r *http.Request) {
	var req publicengine.AppendIterationRequest
	if decode(w, r, &req) {
		writeResult(w)(h.engine.AppendIteration(r.Context(), chi.URLParam(r, "researchRunID"), req))
	}
}

func (h *handler) getResearchRun(w http.ResponseWriter, r *http.Request) {
	result, err := h.engine.GetResearchRun(r.Context(), chi.URLParam(r, "researchRunID"))
	writeValue(w, result, err)
}

// decode reads a bounded JSON body. Unknown fields are tolerated so newer
// clients can send additive optional fields.
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, publicengine.MaxRequestBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		message := "request body is not valid JSON"
		if errors.As(err, &tooLarge) {
			message = "request body exceeds the contract limit"
		}
		writeError(w, &publicengine.Error{Code: publicengine.CodeInvalidRequest, Message: message})
		return false
	}
	return true
}

func writeResult(w http.ResponseWriter) func(int, []byte, error) {
	return func(status int, body []byte, err error) {
		if err != nil {
			writeError(w, publicengine.AsError(err))
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}
}

func writeValue(w http.ResponseWriter, value any, err error) {
	if err != nil {
		writeError(w, publicengine.AsError(err))
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func writeError(w http.ResponseWriter, err *publicengine.Error) {
	writeJSON(w, err.Status(), publicengine.ErrorResponse{
		ContractVersion: publicengine.ContractVersion,
		Error:           publicengine.ErrorBody{Code: string(err.Code), Message: err.Message},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
