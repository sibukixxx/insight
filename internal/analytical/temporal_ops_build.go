package analytical

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// derived accumulates the metrics and results of one operation.
type derived struct {
	op         string
	srcVersion string
	metrics    []MetricDefinition
	results    []Result
	notes      []QualityFlag
}

func (d *derived) metric(id, name, unit, description string) string {
	full := d.op + ":" + id
	for _, m := range d.metrics {
		if m.ID == full {
			return full
		}
	}
	d.metrics = append(d.metrics, MetricDefinition{
		ID: full, Name: name, Unit: unit, Description: description, Aggregation: d.op,
		Version: "temporal-ops/" + OperationVersion + ":" + d.op + ":" + d.srcVersion,
	})
	return full
}

// add appends one result. A nil value is recorded as missing with the given
// reason; unknown never becomes zero.
func (d *derived) add(metricID string, p Period, dims map[string]string, v *float64, tmpl *TemporalMetadata, basis string, missingCode string, flags ...QualityFlag) {
	r := Result{MetricID: metricID, Period: p, Dimensions: dims, Quality: flags}
	if tmpl != nil {
		t := *tmpl
		t.Origin = "derived"
		if basis != "" {
			t.ValueBasis = basis
		}
		r.Temporal = &t
	}
	if v == nil {
		r.Missing = true
		if missingCode != "" {
			r.Quality = append(r.Quality, QualityFlag{Code: missingCode})
		}
	} else {
		b, _ := json.Marshal(*v)
		r.Value = b
	}
	d.results = append(d.results, r)
}

func (d *derived) limitation(msg string) {
	d.notes = append(d.notes, QualityFlag{Code: "LIMITATION", Message: msg})
}

// ApplyTemporalOperation runs one declarative operation over a validated
// source artifact and returns a derived, validated Analytical Artifact.
// It is a pure function of (source, spec): the derived artifact inherits the
// source generatedAt, so re-running yields the identical artifact and hash.
func ApplyTemporalOperation(source Artifact, spec OperationSpec) (Artifact, error) {
	if err := source.Validate(); err != nil {
		return Artifact{}, err
	}
	if err := spec.Validate(); err != nil {
		return Artifact{}, err
	}
	d, err := computeOperation(source, spec)
	if err != nil {
		return Artifact{}, err
	}
	if len(d.results) == 0 {
		return Artifact{}, fmt.Errorf("%w: operation produced no results", ErrInvalidOperation)
	}
	d.limitation("Descriptive arithmetic over recorded values; it does not explain why values moved.")
	specHash := spec.Hash()
	idSum := sha256.Sum256([]byte(source.ArtifactHash.Value + "|" + specHash.Value))
	var specMap map[string]any
	b, _ := json.Marshal(spec)
	_ = json.Unmarshal(b, &specMap)
	dimSet := map[string]bool{}
	for _, r := range d.results {
		for k := range r.Dimensions {
			dimSet[k] = true
		}
	}
	var dims []string
	for k := range dimSet {
		dims = append(dims, k)
	}
	sort.Strings(dims)
	provenance := make([]SourceProvenance, len(source.Provenance))
	for i, p := range source.Provenance {
		p.TransformationRefs = append(append([]string(nil), p.TransformationRefs...),
			"artifact:"+source.ID, "temporal-operation:"+spec.Operation+"@"+OperationVersion+":sha256:"+specHash.Value)
		provenance[i] = p
	}
	out := Artifact{
		ArtifactSchema: Schema, SchemaVersion: Version,
		ID:       "derived:" + hex.EncodeToString(idSum[:16]),
		Producer: operationEngine, ProducerVersion: OperationVersion, GeneratedAt: source.GeneratedAt,
		ExternalSubject: source.ExternalSubject,
		Datasets:        source.Datasets,
		Spec:            SpecRef{Kind: "temporal-operation", Reference: OperationSchema + "/v" + OperationVersion + "#" + spec.Operation, Hash: specHash},
		Parameters: map[string]any{
			"sourceArtifactId": source.ID, "sourceArtifactHash": source.ArtifactHash.Algorithm + ":" + source.ArtifactHash.Value,
			"operation": specMap,
		},
		Dimensions:  dims,
		Filters:     source.Filters,
		Period:      source.Period,
		Population:  source.Population,
		Metrics:     d.metrics,
		Results:     d.results,
		Quality:     append(append([]QualityFlag(nil), source.Quality...), d.notes...),
		Computation: Computation{Engine: operationEngine, EngineVersion: OperationVersion, Deterministic: true, Timezone: "UTC"},
		Provenance:  provenance,
	}
	canonical, err := json.Marshal(out)
	if err != nil {
		return Artifact{}, err
	}
	sum := sha256.Sum256(canonical)
	out.ArtifactHash = Hash{Algorithm: "sha256", Value: hex.EncodeToString(sum[:])}
	if err := out.Validate(); err != nil {
		return Artifact{}, fmt.Errorf("derived artifact is invalid: %w", err)
	}
	return out, nil
}
