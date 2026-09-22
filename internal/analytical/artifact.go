// Package analytical defines the provider-neutral boundary between deterministic
// analytical producers and Insight's evidence-first research core.
package analytical

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"insight-lab/internal/domain"
)

const (
	Schema  = "insight-lab.analytical-artifact"
	Version = "1"
)

var (
	ErrInvalidArtifact  = errors.New("invalid analytical artifact")
	ErrIdentityConflict = errors.New("analytical artifact identity conflict")
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

type Artifact struct {
	ArtifactSchema  string              `json:"artifactSchema"`
	SchemaVersion   string              `json:"schemaVersion"`
	ID              string              `json:"id"`
	ArtifactHash    Hash                `json:"artifactHash"`
	Producer        string              `json:"producer"`
	ProducerVersion string              `json:"producerVersion"`
	GeneratedAt     time.Time           `json:"generatedAt"`
	ExternalSubject *ExternalSubjectRef `json:"externalSubject,omitempty"`

	Datasets    []DatasetRef       `json:"datasets"`
	Spec        SpecRef            `json:"spec"`
	Parameters  map[string]any     `json:"parameters,omitempty"`
	Dimensions  []string           `json:"dimensions,omitempty"`
	Filters     map[string]any     `json:"filters,omitempty"`
	Period      Period             `json:"period"`
	Population  Population         `json:"population"`
	Metrics     []MetricDefinition `json:"metrics"`
	Results     []Result           `json:"results"`
	Quality     []QualityFlag      `json:"qualityFlags,omitempty"`
	Computation Computation        `json:"computation"`
	Provenance  []SourceProvenance `json:"provenance"`
}

func Import(data []byte) (*Artifact, error) {
	var artifact Artifact
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&artifact); err != nil {
		return nil, fmt.Errorf("%w: decode: %v", ErrInvalidArtifact, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("%w: input must contain exactly one JSON value", ErrInvalidArtifact)
	}
	if err := artifact.Validate(); err != nil {
		return nil, err
	}
	return &artifact, nil
}

func Export(artifact Artifact) ([]byte, error) {
	if err := artifact.Validate(); err != nil {
		return nil, err
	}
	return json.MarshalIndent(artifact, "", "  ")
}

func (a Artifact) Validate() error {
	fail := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrInvalidArtifact, fmt.Sprintf(format, args...))
	}
	if a.ArtifactSchema != Schema || a.SchemaVersion != Version {
		return fail("unsupported schema %q version %q", a.ArtifactSchema, a.SchemaVersion)
	}
	if blank(a.ID) || blank(a.Producer) || blank(a.ProducerVersion) || a.GeneratedAt.IsZero() {
		return fail("id, producer, producerVersion and generatedAt are required")
	}
	if a.ExternalSubject != nil && (blank(a.ExternalSubject.Namespace) || blank(a.ExternalSubject.ID)) {
		return fail("externalSubject requires namespace and id")
	}
	if err := validateHash("artifactHash", a.ArtifactHash); err != nil {
		return fail("%v", err)
	}
	if len(a.Datasets) == 0 {
		return fail("at least one dataset is required")
	}
	datasetIDs := map[string]struct{}{}
	for i, dataset := range a.Datasets {
		if blank(dataset.ID) || blank(dataset.Version) {
			return fail("datasets[%d] requires id and version", i)
		}
		if _, exists := datasetIDs[dataset.ID]; exists {
			return fail("duplicate dataset id %q", dataset.ID)
		}
		datasetIDs[dataset.ID] = struct{}{}
		if err := validateHash(fmt.Sprintf("datasets[%d].hash", i), dataset.Hash); err != nil {
			return fail("%v", err)
		}
	}
	if blank(a.Spec.Kind) || blank(a.Spec.Reference) {
		return fail("spec kind and reference are required")
	}
	if err := validateHash("spec.hash", a.Spec.Hash); err != nil {
		return fail("%v", err)
	}
	if err := validatePeriod("period", a.Period); err != nil {
		return fail("%v", err)
	}
	if blank(a.Population.Description) {
		return fail("population.description is required")
	}
	if len(a.Metrics) == 0 || len(a.Results) == 0 {
		return fail("metrics and results must not be empty")
	}
	metricIDs := map[string]struct{}{}
	for i, metric := range a.Metrics {
		if blank(metric.ID) || blank(metric.Name) || blank(metric.Unit) {
			return fail("metrics[%d] requires id, name and unit", i)
		}
		if _, exists := metricIDs[metric.ID]; exists {
			return fail("duplicate metric id %q", metric.ID)
		}
		metricIDs[metric.ID] = struct{}{}
	}
	for i, result := range a.Results {
		if _, exists := metricIDs[result.MetricID]; !exists {
			return fail("results[%d] references unknown metric %q", i, result.MetricID)
		}
		if err := validatePeriod(fmt.Sprintf("results[%d].period", i), result.Period); err != nil {
			return fail("%v", err)
		}
		if result.Missing && len(result.Value) != 0 {
			return fail("results[%d] cannot have value when missing", i)
		}
		if !result.Missing && !validScalar(result.Value) {
			return fail("results[%d].value must be a JSON scalar", i)
		}
	}
	if blank(a.Computation.Engine) || blank(a.Computation.EngineVersion) || !a.Computation.Deterministic {
		return fail("computation requires engine, engineVersion and deterministic=true")
	}
	if len(a.Provenance) == 0 {
		return fail("source provenance is required")
	}
	covered := map[string]struct{}{}
	for i, provenance := range a.Provenance {
		if _, exists := datasetIDs[provenance.DatasetID]; !exists {
			return fail("provenance[%d] references unknown dataset %q", i, provenance.DatasetID)
		}
		if blank(provenance.Source) || provenance.RetrievedAt.IsZero() {
			return fail("provenance[%d] requires source and retrievedAt", i)
		}
		covered[provenance.DatasetID] = struct{}{}
	}
	for datasetID := range datasetIDs {
		if _, ok := covered[datasetID]; !ok {
			return fail("dataset %q has no source provenance", datasetID)
		}
	}
	return nil
}

// CheckDuplicate defines idempotency: the same id and artifact hash is a
// duplicate no-op; reusing an id with different content is a conflict.
func CheckDuplicate(existing, incoming Artifact) (bool, error) {
	if err := existing.Validate(); err != nil {
		return false, err
	}
	if err := incoming.Validate(); err != nil {
		return false, err
	}
	if existing.ID != incoming.ID {
		return false, nil
	}
	if equalHash(existing.ArtifactHash, incoming.ArtifactHash) {
		return true, nil
	}
	return false, fmt.Errorf("%w: id %q has different artifactHash", ErrIdentityConflict, incoming.ID)
}

// ReproducibilityKey identifies the producer-independent deterministic inputs.
func (a Artifact) ReproducibilityKey() string {
	parts := []string{strings.ToLower(a.Spec.Hash.Algorithm) + ":" + strings.ToLower(a.Spec.Hash.Value)}
	for _, d := range a.Datasets {
		parts = append(parts, d.ID+"@"+d.Version+":"+strings.ToLower(d.Hash.Algorithm)+":"+strings.ToLower(d.Hash.Value))
	}
	parameters, _ := json.Marshal(a.Parameters)
	filters, _ := json.Marshal(a.Filters)
	parts = append(parts, "parameters:"+string(parameters), "filters:"+string(filters))
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func validateHash(name string, h Hash) error {
	if strings.TrimSpace(h.Algorithm) != "sha256" {
		return fmt.Errorf("%s.algorithm must be sha256", name)
	}
	v := strings.TrimSpace(h.Value)
	if len(v) != 64 {
		return fmt.Errorf("%s.value must contain 64 hexadecimal characters", name)
	}
	if _, err := hex.DecodeString(v); err != nil {
		return fmt.Errorf("%s.value must be hexadecimal", name)
	}
	return nil
}

func validatePeriod(name string, p Period) error {
	if blank(p.Start) || blank(p.End) {
		return fmt.Errorf("%s requires start and end", name)
	}
	if p.Start > p.End {
		return fmt.Errorf("%s.start must not be after end", name)
	}
	return nil
}

func validScalar(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var v any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&v) != nil {
		return false
	}
	switch v.(type) {
	case nil, bool, string, json.Number:
		return true
	}
	return false
}

func blank(s string) bool { return strings.TrimSpace(s) == "" }
func equalHash(a, b Hash) bool {
	return strings.EqualFold(a.Algorithm, b.Algorithm) && strings.EqualFold(a.Value, b.Value)
}

// Candidate is intentionally not an Insight. A deterministic result crosses
// the boundary only as neutral evidence plus an observation candidate.
type Candidate struct {
	ArtifactID  string
	ResultIndex int
	Evidence    domain.Evidence
	Observation domain.Observation
}

func ToCandidates(artifact Artifact) ([]Candidate, error) {
	if err := artifact.Validate(); err != nil {
		return nil, err
	}
	metrics := make(map[string]MetricDefinition, len(artifact.Metrics))
	for _, metric := range artifact.Metrics {
		metrics[metric.ID] = metric
	}
	out := make([]Candidate, 0, len(artifact.Results))
	for i, result := range artifact.Results {
		metric := metrics[result.MetricID]
		id := fmt.Sprintf("%s:%s:%d", artifact.ID, result.MetricID, i)
		quote := resultStatement(metric, result)
		observationID := id
		out = append(out, Candidate{
			ArtifactID: artifact.ID, ResultIndex: i,
			Evidence:    domain.Evidence{ID: id, DocumentID: artifact.ID, ObservationID: &observationID, Quote: quote, Type: domain.EvidenceNeutral},
			Observation: domain.Observation{ID: observationID, DocumentID: artifact.ID, Quote: quote, Behavior: "deterministic analytical result; interpretation required", Topic: metric.Name, CreatedAt: artifact.GeneratedAt},
		})
	}
	return out, nil
}

func resultStatement(metric MetricDefinition, result Result) string {
	value := "missing"
	if !result.Missing {
		value = string(result.Value)
	}
	dimensions := make([]string, 0, len(result.Dimensions))
	for key, val := range result.Dimensions {
		dimensions = append(dimensions, key+"="+val)
	}
	sort.Strings(dimensions)
	suffix := ""
	if len(dimensions) > 0 {
		suffix = " (" + strings.Join(dimensions, ", ") + ")"
	}
	return fmt.Sprintf("%s = %s %s for %s..%s%s", metric.Name, value, metric.Unit, result.Period.Start, result.Period.End, suffix)
}
