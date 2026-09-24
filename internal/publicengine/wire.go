// Package publicengine implements the semantics of the Public Engine
// Contract v1 (contracts/public-engine/v1/schema.json): opaque subjects,
// idempotent requests, evidence identity, run-scoped analyses and research
// results. It sits on top of the use-case layer and never branches on a
// subject's namespace or type. Transport lives in internal/http/public.
package publicengine

import (
	"encoding/json"

	"insight-lab/internal/domain"
)

const (
	ContractSchema  = "insight-lab.public-engine"
	ContractVersion = "1"
)

// SupportedContractVersions lists every request contract version this engine
// accepts. A request with any other version fails explicitly.
var SupportedContractVersions = []string{ContractVersion}

// Wire types below mirror schema.json $defs one to one; drift_test.go fails
// when a property is added to or removed from either side.

type SubjectRef struct {
	Namespace string `json:"namespace"`
	ID        string `json:"id"`
	Type      string `json:"type,omitempty"`
}

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

type EvidenceDocument struct {
	ExternalRef string            `json:"externalRef"`
	Source      string            `json:"source"`
	Title       string            `json:"title,omitempty"`
	Content     string            `json:"content"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

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

type AnalysisProvenance struct {
	Execution json.RawMessage `json:"execution,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
}

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
	ContractVersion      string             `json:"contractVersion"`
	IdempotencyKey       string             `json:"idempotencyKey"`
	Question             string             `json:"question"`
	AnalysisID           string             `json:"analysisId"`
	InputReferences      []string           `json:"inputReferences,omitempty"`
	SemanticAnalysisMode string             `json:"semanticAnalysisMode,omitempty"`
	ObservationWindow    *ObservationWindow `json:"observationWindow,omitempty"`
}

type AddedEvidenceLink struct {
	Reference string   `json:"reference"`
	GapIDs    []string `json:"gapIds,omitempty"`
	Note      string   `json:"note,omitempty"`
}

type AppendIterationRequest struct {
	ContractVersion   string              `json:"contractVersion"`
	IdempotencyKey    string              `json:"idempotencyKey"`
	AnalysisID        string              `json:"analysisId"`
	Question          string              `json:"question,omitempty"`
	AddedEvidence     []AddedEvidenceLink `json:"addedEvidence,omitempty"`
	ObservationWindow *ObservationWindow  `json:"observationWindow,omitempty"`
}

type ResearchResult struct {
	ContractVersion   string          `json:"contractVersion"`
	SubjectID         string          `json:"subjectId"`
	ResearchRunID     string          `json:"researchRunId"`
	IterationID       string          `json:"iterationId"`
	IterationSequence int             `json:"iterationSequence"`
	Analysis          AnalysisRun     `json:"analysis"`
	Artifact          json.RawMessage `json:"artifact"`
}

// Longitudinal timeline (#71). Nested entries reuse the domain read model;
// drift_test.go keeps their JSON fields identical to the schema.
type (
	ObservationWindow        = domain.ObservationWindow
	TimelineIteration        = domain.TimelineIteration
	EvidenceEvent            = domain.EvidenceEvent
	TimelineObservationDelta = domain.TimelineObservationDelta
	HypothesisEvent          = domain.HypothesisEvent
	InsightVersion           = domain.InsightVersion
	InstrumentChange         = domain.InstrumentChange
)

type ResearchTimeline struct {
	ContractVersion   string                     `json:"contractVersion"`
	SubjectID         string                     `json:"subjectId"`
	ResearchRunID     string                     `json:"researchRunId"`
	Question          string                     `json:"question"`
	AsOf              string                     `json:"asOf,omitempty"`
	Iterations        []TimelineIteration        `json:"iterations"`
	EvidenceEvents    []EvidenceEvent            `json:"evidenceEvents"`
	ObservationDeltas []TimelineObservationDelta `json:"observationDeltas"`
	HypothesisEvents  []HypothesisEvent          `json:"hypothesisEvents"`
	InsightVersions   []InsightVersion           `json:"insightVersions"`
	InstrumentChanges []InstrumentChange         `json:"instrumentChanges"`
	Limitations       []string                   `json:"limitations"`
}

// Temporal Analytics Pack (#73). Stateless and deterministic: the same
// artifact and operation always return the same derived artifact.
type TemporalOperationRequest struct {
	ContractVersion string          `json:"contractVersion"`
	Artifact        json.RawMessage `json:"artifact"`
	Operation       json.RawMessage `json:"operation"`
}

type TemporalOperationResult struct {
	ContractVersion string          `json:"contractVersion"`
	Artifact        json.RawMessage `json:"artifact"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	ContractVersion string    `json:"contractVersion"`
	Error           ErrorBody `json:"error"`
}
