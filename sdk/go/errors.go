package insight

import "fmt"

// Error codes. The server codes are defined by the contract schema
// (x-errorCodes). CodeUnavailable is produced by the client when the server
// cannot be reached or does not answer with a contract response.
const (
	CodeInvalidRequest             = "INVALID_REQUEST"
	CodeUnsupportedContractVersion = "UNSUPPORTED_CONTRACT_VERSION"
	CodeNotFound                   = "NOT_FOUND"
	CodeIdempotencyConflict        = "IDEMPOTENCY_CONFLICT"
	CodeIdentityConflict           = "IDENTITY_CONFLICT"
	CodeAnalysisNotCompleted       = "ANALYSIS_NOT_COMPLETED"
	CodeAnalysisHasNoHypotheses    = "ANALYSIS_HAS_NO_HYPOTHESES"
	CodeMixedAnalysisRuns          = "MIXED_ANALYSIS_RUNS"
	CodeInternal                   = "INTERNAL"
	CodeUnavailable                = "UNAVAILABLE"
)

// Error is a typed contract error. HTTPStatus is 0 when the error was raised
// by the client (for example CodeUnavailable, or an unsupported contract
// version in a response).
type Error struct {
	Code       string
	Message    string
	HTTPStatus int
	cause      error
}

func (e *Error) Error() string {
	if e.HTTPStatus != 0 {
		return fmt.Sprintf("insight: %s (HTTP %d): %s", e.Code, e.HTTPStatus, e.Message)
	}
	return fmt.Sprintf("insight: %s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.cause }

// Is lets errors.Is match on the code: errors.Is(err, &insight.Error{Code: insight.CodeNotFound}).
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}
