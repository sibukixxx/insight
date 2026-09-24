package publicengine

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"insight-lab/internal/analytical"
)

// ApplyTemporalOperation runs one #73 operation. It stores nothing: a caller
// that wants the derived artifact researched submits it as evidence like any
// other Analytical Artifact.
func (e *Engine) ApplyTemporalOperation(req TemporalOperationRequest) (TemporalOperationResult, error) {
	if req.ContractVersion != ContractVersion {
		return TemporalOperationResult{}, newError(CodeUnsupportedContractVersion, "contractVersion %q is not supported; supported versions: %s", req.ContractVersion, strings.Join(SupportedContractVersions, ", "))
	}
	if len(req.Artifact) == 0 || len(req.Operation) == 0 {
		return TemporalOperationResult{}, newError(CodeInvalidRequest, "artifact and operation are required")
	}
	source, err := analytical.Import(req.Artifact)
	if err != nil {
		return TemporalOperationResult{}, newError(CodeInvalidRequest, "%s", err.Error())
	}
	var spec analytical.OperationSpec
	dec := json.NewDecoder(bytes.NewReader(req.Operation))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&spec); err != nil {
		return TemporalOperationResult{}, newError(CodeInvalidRequest, "operation is not a valid %s: %v", analytical.OperationSchema, err)
	}
	out, err := analytical.ApplyTemporalOperation(*source, spec)
	if errors.Is(err, analytical.ErrInvalidOperation) || errors.Is(err, analytical.ErrInvalidArtifact) {
		return TemporalOperationResult{}, newError(CodeInvalidRequest, "%s", err.Error())
	}
	if err != nil {
		return TemporalOperationResult{}, err
	}
	encoded, err := analytical.Export(out)
	if err != nil {
		return TemporalOperationResult{}, err
	}
	return TemporalOperationResult{ContractVersion: ContractVersion, Artifact: encoded}, nil
}
