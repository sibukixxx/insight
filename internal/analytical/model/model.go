// Package model contains the shared analytical value types used by artifacts and domain evidence.
package model

import (
 "encoding/json"
 "time"
)

type Hash struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

type DatasetRef struct {
	ID      string `json:"id"`
	URI     string `json:"uri,omitempty"`
	Version string `json:"version"`
	Hash    Hash   `json:"hash"`
}

type SpecRef struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Hash      Hash   `json:"hash"`
}

// ExternalSubjectRef is an opaque producer-owned identity. Insight never
// interprets Namespace or ID as domain state or an instruction to act.
type ExternalSubjectRef struct {
	Namespace string `json:"namespace"`
	ID        string `json:"id"`
}

type Period struct {
	Start string `json:"start"`
	End   string `json:"end"`
	Basis string `json:"basis,omitempty"`
}

type Population struct {
	Description string `json:"description"`
	Unit        string `json:"unit,omitempty"`
}

type MetricDefinition struct {
	Version string `json:"version,omitempty"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Unit        string `json:"unit"`
	Aggregation string `json:"aggregation,omitempty"`
}

type QualityFlag struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

type Result struct {
	Temporal *TemporalMetadata `json:"temporal,omitempty"`
	MetricID   string            `json:"metricId"`
	Dimensions map[string]string `json:"dimensions,omitempty"`
	Period     Period            `json:"period"`
	Value      json.RawMessage   `json:"value,omitempty"`
	Missing    bool              `json:"missing,omitempty"`
	Quality    []QualityFlag     `json:"qualityFlags,omitempty"`
}

type Computation struct {
	Engine        string `json:"engine"`
	EngineVersion string `json:"engineVersion"`
	Deterministic bool   `json:"deterministic"`
	Timezone      string `json:"timezone,omitempty"`
}

type SourceProvenance struct {
	DatasetID          string    `json:"datasetId"`
	Source             string    `json:"source"`
	RetrievedAt        time.Time `json:"retrievedAt"`
	License            string    `json:"license,omitempty"`
	TransformationRefs []string  `json:"transformationRefs,omitempty"`
}


type TemporalMetadata struct {
 ObservedAt time.Time `json:"observedAt"`
 Origin string `json:"origin"`
 Geography string `json:"geography"`
 ValueBasis string `json:"valueBasis"`
}

// TemporalEvidence is an additive projection of an artifact result, not a
// second observation identity. Period is event time; ObservedAt is measurement
// time; RetrievedAt is acquisition time; GeneratedAt is analysis time.
type TemporalEvidence struct {
 ArtifactID string `json:"artifactId"`
 ArtifactHash Hash `json:"artifactHash"`
 ResultIndex int `json:"resultIndex"`
 Metric MetricDefinition `json:"metric"`
 Result Result `json:"result"`
 Population Population `json:"population"`
 Datasets []DatasetRef `json:"datasets"`
 Spec SpecRef `json:"spec"`
 Parameters map[string]any `json:"parameters,omitempty"`
 Filters map[string]any `json:"filters,omitempty"`
 Provenance []SourceProvenance `json:"provenance"`
 Quality []QualityFlag `json:"qualityFlags,omitempty"`
 GeneratedAt time.Time `json:"generatedAt"`
}
