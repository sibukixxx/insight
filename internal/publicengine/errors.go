package publicengine

import (
	"errors"
	"fmt"
	"net/http"

	"insight-lab/internal/domain"
	"insight-lab/internal/execution"
	"insight-lab/internal/input"
	"insight-lab/internal/usecase"
)

// Code is a stable, typed error code of the public contract.
type Code string

const (
	CodeInvalidRequest              Code = "INVALID_REQUEST"
	CodeUnsupportedContractVersion  Code = "UNSUPPORTED_CONTRACT_VERSION"
	CodeNotFound                    Code = "NOT_FOUND"
	CodeIdempotencyConflict         Code = "IDEMPOTENCY_CONFLICT"
	CodeIdentityConflict            Code = "IDENTITY_CONFLICT"
	CodeAnalysisNotCompleted        Code = "ANALYSIS_NOT_COMPLETED"
	CodeAnalysisHasNoHypotheses     Code = "ANALYSIS_HAS_NO_HYPOTHESES"
	CodeMixedAnalysisRuns           Code = "MIXED_ANALYSIS_RUNS"
	CodeStaleIteration              Code = "STALE_ITERATION"
	CodeInternal                    Code = "INTERNAL"
	CodeExecutionProfileUnavailable Code = "EXECUTION_PROFILE_UNAVAILABLE"
	CodeInputSourceUnavailable      Code = "INPUT_SOURCE_UNAVAILABLE"
	CodeInputVerificationFailed     Code = "INPUT_VERIFICATION_FAILED"
)

var errorStatus = map[Code]int{
	CodeInvalidRequest:              http.StatusBadRequest,
	CodeUnsupportedContractVersion:  http.StatusBadRequest,
	CodeNotFound:                    http.StatusNotFound,
	CodeIdempotencyConflict:         http.StatusConflict,
	CodeIdentityConflict:            http.StatusConflict,
	CodeAnalysisNotCompleted:        http.StatusConflict,
	CodeAnalysisHasNoHypotheses:     http.StatusConflict,
	CodeMixedAnalysisRuns:           http.StatusConflict,
	CodeStaleIteration:              http.StatusConflict,
	CodeInternal:                    http.StatusInternalServerError,
	CodeExecutionProfileUnavailable: http.StatusUnprocessableEntity,
	CodeInputSourceUnavailable:      http.StatusUnprocessableEntity,
	CodeInputVerificationFailed:     http.StatusBadRequest,
}

// Error is a contract error. Status is the HTTP status the code maps to.
type Error struct {
	Code    Code
	Message string
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Message }

// Status returns the HTTP status for the error code.
func (e *Error) Status() int { return errorStatus[e.Code] }

func newError(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// AsError maps any error to a contract error. Use-case sentinels keep their
// meaning; anything unrecognised is INTERNAL and its detail is not leaked.
func AsError(err error) *Error {
	var contractErr *Error
	switch {
	case errors.As(err, &contractErr):
		return contractErr
	case errors.Is(err, usecase.ErrScenarioInvalid):
		// Validation messages name the violated guardrail and carry no
		// internal detail.
		return newError(CodeInvalidRequest, "%s", err.Error())
	case errors.Is(err, domain.ErrInvalidObservationWindow):
		return newError(CodeInvalidRequest, "%s", err.Error())
	case errors.Is(err, usecase.ErrNotFound):
		return newError(CodeNotFound, "resource not found")
	case errors.Is(err, usecase.ErrAnalysisNotCompleted):
		return newError(CodeAnalysisNotCompleted, "the analysis run has not completed")
	case errors.Is(err, usecase.ErrAnalysisHasNoHypotheses):
		return newError(CodeAnalysisHasNoHypotheses, "the analysis run produced no hypotheses to research; a model-backed run is required")
	case errors.Is(err, usecase.ErrStaleIteration):
		return newError(CodeStaleIteration, "previousIterationId is not the latest iteration of the research run")
	case errors.Is(err, usecase.ErrReEvaluationConflict):
		return newError(CodeIdempotencyConflict, "correlationKey was already used for a different analysis run")
	case errors.Is(err, usecase.ErrInvalidInput):
		return newError(CodeInvalidRequest, "%s", err.Error())
	case errors.Is(err, usecase.ErrCrossProjectComparison):
		return newError(CodeNotFound, "analysis not found for this subject")
	case errors.Is(err, execution.ErrProfileUnavailable):
		return newError(CodeExecutionProfileUnavailable, "%s", err.Error())
	case errors.Is(err, execution.ErrInvalidProfile):
		return newError(CodeInvalidRequest, "%s", err.Error())
	case errors.Is(err, input.ErrUnavailable):
		return newError(CodeInputSourceUnavailable, "%s", err.Error())
	case errors.Is(err, input.ErrVerification):
		return newError(CodeInputVerificationFailed, "%s", err.Error())
	case errors.Is(err, usecase.ErrMixedAnalysisRuns):
		return newError(CodeMixedAnalysisRuns, "the research iteration spans more than one analysis run")
	}
	return newError(CodeInternal, "internal error")
}
