package insight

import "encoding/json"

// ContractVersion is the Public Engine Contract version this SDK speaks.
const ContractVersion = "1"

// Wire types mirror contracts/public-engine/v1/schema.json $defs one to one.
// drift_test.go fails when either side gains or loses a property.

// SubjectRef is an opaque external subject. Insight stores and echoes it and
// never changes behavior based on its namespace or type.
type SubjectRef struct {
	Namespace string `json:"namespace"`
	ID        string `json:"id"`
	Type      string `json:"type,omitempty"`
}

// EngineBuild identifies an engine build. Unknown values are "UNKNOWN".
type EngineBuild struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Dirty   string `json:"dirty"`
}

type SchemaRef struct {
	Schema  string `json:"schema"`
	Version string `json:"version"`
}

type EngineInfo struct {
	ContractSchema            string      `json:"contractSchema"`
	ContractVersion           string      `json:"contractVersion"`
	SupportedContractVersions []string    `json:"supportedContractVersions"`
	Engine                    EngineBuild `json:"engine"`
	ResearchArtifact          SchemaRef   `json:"researchArtifact"`
	AnalyticalArtifact        SchemaRef   `json:"analyticalArtifact"`
}

// CreateSubjectRequest creates or re-resolves a subject. ContractVersion and
// IdempotencyKey are filled in by the client when empty.
type CreateSubjectRequest struct {
	ContractVersion string            `json:"contractVersion"`
	IdempotencyKey  string            `json:"idempotencyKey"`
	Subject         SubjectRef        `json:"subject"`
	Title           string            `json:"title,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

type Subject struct {
	ContractVersion string            `json:"contractVersion"`
	SubjectID       string            `json:"subjectId"`
	Subject         SubjectRef        `json:"subject"`
	Title           string            `json:"title"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	CreatedAt       string            `json:"createdAt"`
}

// EvidenceDocument is one piece of evidence, identified by ExternalRef
// within its subject.
type EvidenceDocument struct {
	ExternalRef string            `json:"externalRef"`
	Source      string            `json:"source"`
	Title       string            `json:"title,omitempty"`
	Content     string            `json:"content"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// AddEvidenceRequest carries documents and Analytical Artifacts
// (contracts/analytical-artifact/v1) as raw JSON.
type AddEvidenceRequest struct {
	ContractVersion     string             `json:"contractVersion"`
	IdempotencyKey      string             `json:"idempotencyKey"`
	Documents           []EvidenceDocument `json:"documents,omitempty"`
	AnalyticalArtifacts []json.RawMessage  `json:"analyticalArtifacts,omitempty"`
}

type EvidenceItemReceipt struct {
	ExternalRef        string `json:"externalRef,omitempty"`
	ArtifactID         string `json:"artifactId,omitempty"`
	ReproducibilityKey string `json:"reproducibilityKey,omitempty"`
	DocumentID         string `json:"documentId"`
	Status             string `json:"status"`
}

type EvidenceReceipt struct {
	ContractVersion     string                `json:"contractVersion"`
	SubjectID           string                `json:"subjectId"`
	Documents           []EvidenceItemReceipt `json:"documents"`
	AnalyticalArtifacts []EvidenceItemReceipt `json:"analyticalArtifacts"`
}

type StartAnalysisRequest struct {
	ContractVersion      string `json:"contractVersion"`
	IdempotencyKey       string `json:"idempotencyKey"`
	Label                string `json:"label,omitempty"`
	Note                 string `json:"note,omitempty"`
	SemanticAnalysisMode string `json:"semanticAnalysisMode,omitempty"`
}

// AnalysisProvenance holds the run's execution and input snapshots verbatim.
// A snapshot the run never recorded is nil, never an empty object.
type AnalysisProvenance struct {
	Execution json.RawMessage `json:"execution,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
}

// AnalysisRun is one run of the engine over a subject's evidence.
type AnalysisRun struct {
	ContractVersion      string              `json:"contractVersion"`
	SubjectID            string              `json:"subjectId"`
	AnalysisID           string              `json:"analysisId"`
	Status               string              `json:"status"`
	Error                string              `json:"error,omitempty"`
	Label                string              `json:"label,omitempty"`
	Note                 string              `json:"note,omitempty"`
	SemanticAnalysisMode string              `json:"semanticAnalysisMode,omitempty"`
	ExecutionMode        string              `json:"executionMode,omitempty"`
	Engine               *EngineBuild        `json:"engine,omitempty"`
	ExecutionFingerprint string              `json:"executionFingerprint,omitempty"`
	InputFingerprint     string              `json:"inputFingerprint,omitempty"`
	Provenance           *AnalysisProvenance `json:"provenance,omitempty"`
	CreatedAt            string              `json:"createdAt"`
	StartedAt            string              `json:"startedAt,omitempty"`
	FinishedAt           string              `json:"finishedAt,omitempty"`
}

// Terminal reports whether the run will not change status again.
func (r AnalysisRun) Terminal() bool { return r.Status == "completed" || r.Status == "failed" }

type Observation struct {
	ObservationID string `json:"observationId"`
	DocumentID    string `json:"documentId"`
	ExternalRef   string `json:"externalRef,omitempty"`
	Quote         string `json:"quote"`
	StartOffset   int    `json:"startOffset"`
	EndOffset     int    `json:"endOffset"`
	Behavior      string `json:"behavior"`
	Topic         string `json:"topic,omitempty"`
}

type Finding struct {
	FindingID      string   `json:"findingId"`
	Kind           string   `json:"kind"`
	Title          string   `json:"title"`
	Description    string   `json:"description,omitempty"`
	Expectation    string   `json:"expectation,omitempty"`
	ObservationIDs []string `json:"observationIds"`
}

type AnalysisResults struct {
	ContractVersion string          `json:"contractVersion"`
	SubjectID       string          `json:"subjectId"`
	AnalysisID      string          `json:"analysisId"`
	Observations    []Observation   `json:"observations"`
	Findings        []Finding       `json:"findings"`
	Metrics         json.RawMessage `json:"metrics,omitempty"`
}

type CreateResearchRunRequest struct {
	ContractVersion      string   `json:"contractVersion"`
	IdempotencyKey       string   `json:"idempotencyKey"`
	Question             string   `json:"question"`
	AnalysisID           string   `json:"analysisId"`
	InputReferences      []string `json:"inputReferences,omitempty"`
	SemanticAnalysisMode string   `json:"semanticAnalysisMode,omitempty"`
}

// AddedEvidenceLink records which research gaps an added piece of evidence
// was meant to address.
type AddedEvidenceLink struct {
	Reference string   `json:"reference"`
	GapIDs    []string `json:"gapIds,omitempty"`
	Note      string   `json:"note,omitempty"`
}

type AppendIterationRequest struct {
	ContractVersion string              `json:"contractVersion"`
	IdempotencyKey  string              `json:"idempotencyKey"`
	AnalysisID      string              `json:"analysisId"`
	Question        string              `json:"question,omitempty"`
	AddedEvidence   []AddedEvidenceLink `json:"addedEvidence,omitempty"`
}

// ResearchResult is the latest iteration of a research run. Artifact is the
// insight-lab.research-artifact v1 export, verbatim; View decodes the parts
// the contract promises.
type ResearchResult struct {
	ContractVersion   string          `json:"contractVersion"`
	SubjectID         string          `json:"subjectId"`
	ResearchRunID     string          `json:"researchRunId"`
	IterationID       string          `json:"iterationId"`
	IterationSequence int             `json:"iterationSequence"`
	Analysis          AnalysisRun     `json:"analysis"`
	Artifact          json.RawMessage `json:"artifact"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	ContractVersion string    `json:"contractVersion"`
	Error           ErrorBody `json:"error"`
}
