// Package analytical is the stable Go import/export surface for Analytical
// Artifact wire contracts. Consumers do not import Insight internal packages.
package analytical

import internal "insight-lab/internal/analytical"

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
