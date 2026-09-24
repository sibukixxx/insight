package publicengine

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strconv"
	"strings"

	"insight-lab/internal/domain"
	"insight-lab/internal/execution"
	"insight-lab/internal/input"
)

// DefaultMaxRawArtifactBytes bounds one referenced raw artifact. The bytes
// are streamed, never held in memory; the bound limits verification time.
const DefaultMaxRawArtifactBytes = 16 << 30

const maxInputSourcesPerRequest = 50

// Option configures optional engine capabilities.
type Option func(*Engine)

// WithInputResolver lets the engine read referenced raw artifacts. Without
// it, inputSources fail with INPUT_SOURCE_UNAVAILABLE.
func WithInputResolver(r input.Resolver, maxBytes int64) Option {
	return func(e *Engine) {
		e.resolver = r
		if maxBytes <= 0 {
			maxBytes = DefaultMaxRawArtifactBytes
		}
		e.maxRawBytes = maxBytes
	}
}

// capabilityReporter is implemented by *service.JobManager.
type capabilityReporter interface {
	ExecutionCapabilities() execution.Capabilities
}

func (e *Engine) capabilities() execution.Capabilities {
	if r, ok := e.jobs.(capabilityReporter); ok {
		return r.ExecutionCapabilities()
	}
	return execution.Capabilities{}
}

func profileInfos(c execution.Capabilities) []ExecutionProfileInfo {
	var out []ExecutionProfileInfo
	for _, p := range c.Profiles() {
		out = append(out, ExecutionProfileInfo{Profile: string(p.Profile), Available: p.Available, Description: p.Description})
	}
	return out
}

// addInputSources verifies each referenced raw artifact by streaming it and
// returns the reference documents to create. The engine records only the
// sha256 and size it measured itself.
func (e *Engine) addInputSources(ctx context.Context, projectID string, byRef map[string]*domain.Document, sources []InputSource, seen map[string]bool) ([]*domain.Document, []InputSourceReceipt, error) {
	if len(sources) > maxInputSourcesPerRequest {
		return nil, nil, newError(CodeInvalidRequest, "at most %d input sources per request", maxInputSourcesPerRequest)
	}
	var created []*domain.Document
	receipts := []InputSourceReceipt{}
	for i, in := range sources {
		spec, invalid := validateInputSource(in)
		if invalid != nil {
			return nil, nil, newError(CodeInvalidRequest, "inputSources[%d]: %s", i, invalid.Message)
		}
		if seen[in.ExternalRef] {
			return nil, nil, newError(CodeInvalidRequest, "inputSources[%d]: externalRef %q appears more than once", i, in.ExternalRef)
		}
		seen[in.ExternalRef] = true
		if e.resolver == nil {
			return nil, nil, newError(CodeInputSourceUnavailable, "inputSources[%d]: this engine has no input resolver configured", i)
		}
		claimed := input.RawArtifactRef{URI: in.RawArtifact.URI, MediaType: in.RawArtifact.MediaType, Name: in.RawArtifact.Name,
			Version: in.RawArtifact.Version, SizeBytes: in.RawArtifact.SizeBytes, SHA256: strings.ToLower(in.RawArtifact.SHA256)}
		verified, err := input.Verify(ctx, e.resolver, claimed, e.maxRawBytes)
		if err != nil {
			contractErr := AsError(err)
			contractErr.Message = fmt.Sprintf("inputSources[%d]: %s", i, contractErr.Message)
			return nil, nil, contractErr
		}
		receipt := InputSourceReceipt{ExternalRef: in.ExternalRef, SHA256: verified.SHA256, SizeBytes: verified.SizeBytes, VerifiedBy: "engine", Preparation: "NOT_REQUESTED"}
		if spec != nil {
			receipt.Preparation, receipt.PreparedArtifactID = "PENDING", input.PreparedArtifactID(verified.SHA256, *spec)
		}
		doc := rawReferenceDocument(projectID, in, verified, spec, e)
		if stored, ok := byRef[in.ExternalRef]; ok {
			if !sameReference(stored, doc) {
				return nil, nil, newError(CodeIdentityConflict, "input source %q already exists with different content", in.ExternalRef)
			}
			receipt.DocumentID, receipt.Status = stored.ID, "UNCHANGED"
			receipts = append(receipts, receipt)
			continue
		}
		receipt.DocumentID, receipt.Status = doc.ID, "CREATED"
		created = append(created, doc)
		receipts = append(receipts, receipt)
	}
	return created, receipts, nil
}

func validateInputSource(in InputSource) (*input.PreparationSpec, *Error) {
	if strings.TrimSpace(in.ExternalRef) == "" || len(in.ExternalRef) > 256 {
		return nil, newError(CodeInvalidRequest, "externalRef must be 1-256 characters")
	}
	if in.Kind != string(input.KindRawArtifact) {
		return nil, newError(CodeInvalidRequest, "kind %q is not supported here (use %s; documents and analyticalArtifacts have their own fields)", in.Kind, input.KindRawArtifact)
	}
	if strings.TrimSpace(in.RawArtifact.URI) == "" || strings.TrimSpace(in.RawArtifact.MediaType) == "" {
		return nil, newError(CodeInvalidRequest, "rawArtifact.uri and rawArtifact.mediaType are required")
	}
	if len(in.Title) > 512 || len(in.RawArtifact.URI) > 2048 || in.RawArtifact.SizeBytes < 0 {
		return nil, newError(CodeInvalidRequest, "title is limited to 512 and uri to 2048 characters; sizeBytes must not be negative")
	}
	if err := validateMetadata(in.Metadata, true); err != nil {
		return nil, AsError(err)
	}
	if len(in.Preparation) == 0 || string(in.Preparation) == "null" {
		return nil, nil
	}
	var spec input.PreparationSpec
	dec := json.NewDecoder(strings.NewReader(string(in.Preparation)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&spec); err != nil {
		return nil, newError(CodeInvalidRequest, "preparation is not a valid preparation spec: %v", err)
	}
	if err := spec.Validate(); err != nil {
		return nil, newError(CodeInvalidRequest, "%v", err)
	}
	if !strings.HasPrefix(strings.ToLower(in.RawArtifact.MediaType), "text/csv") {
		return nil, newError(CodeInvalidRequest, "preparation %s requires mediaType text/csv", spec.Kind)
	}
	return &spec, nil
}

func rawReferenceDocument(projectID string, in InputSource, ref input.RawArtifactRef, spec *input.PreparationSpec, e *Engine) *domain.Document {
	metadata := maps.Clone(in.Metadata)
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata[metadataExternalRef] = in.ExternalRef
	metadata[input.MetadataKind] = string(input.KindRawArtifact)
	metadata[input.MetadataRawURI] = ref.URI
	metadata[input.MetadataRawMedia] = ref.MediaType
	metadata[input.MetadataRawSHA256] = ref.SHA256
	metadata[input.MetadataRawSize] = strconv.FormatInt(ref.SizeBytes, 10)
	if ref.Name != "" {
		metadata[input.MetadataRawName] = ref.Name
	}
	if ref.Version != "" {
		metadata[input.MetadataRawVersion] = ref.Version
	}
	if spec != nil {
		metadata[input.MetadataPreparation] = string(spec.SpecJSON())
	}
	title := firstNonEmpty(in.Title, ref.Name, in.ExternalRef)
	content := fmt.Sprintf("Raw artifact reference %s (%s, %d bytes, sha256 %s). Content is read only through preparation.", ref.URI, ref.MediaType, ref.SizeBytes, ref.SHA256)
	return &domain.Document{ID: newID("doc"), ProjectID: projectID, Source: domain.SourceDataset, Title: title, Content: content, Metadata: metadata, CreatedAt: e.now()}
}

// sameReference compares identity-bearing fields; the descriptor text and
// registration time are derived and ignored.
func sameReference(stored, incoming *domain.Document) bool {
	return stored.Title == incoming.Title && maps.Equal(stored.Metadata, incoming.Metadata)
}
