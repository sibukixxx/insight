// Package analytical is the stable Go import/export surface for Analytical
// Artifact wire contracts. Consumers do not import Insight internal packages.
package analytical

import (
 internal "insight-lab/internal/analytical"
 "insight-lab/internal/domain"
)

const (
	Schema  = internal.Schema
	Version = internal.Version
)

var (
	ErrInvalidArtifact  = internal.ErrInvalidArtifact
	ErrIdentityConflict = internal.ErrIdentityConflict
)

type Hash = internal.Hash
type DatasetRef = internal.DatasetRef
type SpecRef = internal.SpecRef
type ExternalSubjectRef = internal.ExternalSubjectRef
type Period = internal.Period
type Population = internal.Population
type MetricDefinition = internal.MetricDefinition
type QualityFlag = internal.QualityFlag
type Result = internal.Result
type Computation = internal.Computation
type SourceProvenance = internal.SourceProvenance
type Artifact = internal.Artifact

func Import(data []byte) (*Artifact, error)    { return internal.Import(data) }
func Export(artifact Artifact) ([]byte, error) { return internal.Export(artifact) }
func CheckDuplicate(existing, incoming Artifact) (bool, error) {
	return internal.CheckDuplicate(existing, incoming)
}

type TemporalMetadata = internal.TemporalMetadata
type TemporalEvidence = internal.TemporalEvidence
type ObservationReference = internal.ObservationReference
type ObservationDelta = internal.ObservationDelta
type Observation = domain.Observation
type Candidate = internal.Candidate

func ToCandidatesForAnalysis(artifact Artifact, analysisID string) ([]Candidate,error) {
 return internal.ToCandidatesForAnalysis(artifact,analysisID)
}
func CompareObservations(previous,current Observation) ObservationDelta {
 return internal.CompareObservations(previous,current)
}
func CompareObservationSeries(observations []Observation) []ObservationDelta {
 return internal.CompareObservationSeries(observations)
}

// Temporal Analytics Pack (#73): declarative operations over temporal results
// that return a derived, neutral Analytical Artifact.
const (
	OperationSchema  = internal.OperationSchema
	OperationVersion = internal.OperationVersion
)

var ErrInvalidOperation = internal.ErrInvalidOperation

type OperationSpec = internal.OperationSpec

func ApplyTemporalOperation(source Artifact, spec OperationSpec) (Artifact, error) {
	return internal.ApplyTemporalOperation(source, spec)
}
