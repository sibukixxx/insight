package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/execution"
	"insight-lab/internal/input"
)

// MetadataPreparedFrom links a prepared Analytical Artifact document to the
// raw artifact reference it was computed from.
const MetadataPreparedFrom = "public_prepared_from"

// Preparation turns referenced raw artifacts into prepared Analytical
// Artifacts before the canonical pipeline runs (Issues #90/#91/#93). STANDARD
// streams in-process; HEAVY delegates partitions to the configured runtime.
// Both use the same deterministic processor, so the prepared artifact — and
// every research result derived from it — is identical across profiles.
type Preparation struct {
	Resolver    input.Resolver
	Heavy       execution.Runtime
	Partitions  int
	MaxRawBytes int64
}

func (p *Preparation) Capabilities() execution.Capabilities {
	return execution.Capabilities{HeavyRuntime: p != nil && p.Heavy != nil}
}

// Prepare returns new artifact documents for raw references that have a
// preparation spec and no prepared artifact yet. It never modifies docs.
func (p *Preparation) Prepare(ctx context.Context, projectID string, docs []*domain.Document, profile execution.Profile, now time.Time) ([]*domain.Document, error) {
	prepared := map[string]bool{}
	for _, d := range docs {
		if id := d.Metadata[MetadataAnalyticalArtifactID]; id != "" {
			prepared[id] = true
		}
	}
	var out []*domain.Document
	for _, d := range docs {
		ref, ok := input.RefFromDocument(d)
		if !ok || d.Metadata[input.MetadataPreparation] == "" {
			continue
		}
		var spec input.PreparationSpec
		if err := json.Unmarshal([]byte(d.Metadata[input.MetadataPreparation]), &spec); err != nil {
			return nil, fmt.Errorf("raw artifact %s: stored preparation is invalid: %w", d.ID, err)
		}
		id := input.PreparedArtifactID(ref.SHA256, spec)
		if prepared[id] {
			continue
		}
		if p == nil || p.Resolver == nil {
			return nil, fmt.Errorf("%w: no input resolver is configured to read raw artifact %s", input.ErrUnavailable, d.ID)
		}
		agg, err := p.aggregate(ctx, id, ref, spec, profile)
		if err != nil {
			return nil, fmt.Errorf("prepare raw artifact %s: %w", d.ID, err)
		}
		datasetID := firstNonEmptyString(d.Metadata["public_external_ref"], d.ID)
		artifact, err := input.BuildPreparedArtifact(datasetID, ref, spec, agg, d.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("prepare raw artifact %s: %w", d.ID, err)
		}
		doc, err := AnalyticalArtifactDocument(projectID, artifact, now)
		if err != nil {
			return nil, err
		}
		doc.Metadata[MetadataPreparedFrom] = d.ID
		prepared[id] = true
		out = append(out, doc)
	}
	return out, nil
}

func (p *Preparation) aggregate(ctx context.Context, jobID string, ref input.RawArtifactRef, spec input.PreparationSpec, profile execution.Profile) (*input.Aggregate, error) {
	switch profile {
	case execution.ProfileStandard:
		return p.scan(ctx, ref, spec, nil)
	case execution.ProfileHeavy:
		if p.Heavy == nil {
			return nil, fmt.Errorf("%w: HEAVY requires a configured Heavy Execution Adapter", execution.ErrProfileUnavailable)
		}
		k := p.Partitions
		if k <= 0 {
			k = 4
		}
		job := execution.JobSpec{ID: jobID}
		for i := 0; i < k; i++ {
			job.Partitions = append(job.Partitions, fmt.Sprintf("rows-mod-%d-of-%d", i, k))
		}
		state, err := p.Heavy.Run(ctx, job, func(ctx context.Context, partition string) (json.RawMessage, error) {
			var i, n int64
			if _, err := fmt.Sscanf(partition, "rows-mod-%d-of-%d", &i, &n); err != nil {
				return nil, err
			}
			agg, err := p.scan(ctx, ref, spec, func(row int64) bool { return row%n == i })
			if err != nil {
				return nil, err
			}
			return json.Marshal(agg)
		})
		if err != nil {
			return nil, err
		}
		merged := input.NewAggregate()
		for _, part := range state.Partitions {
			var agg input.Aggregate
			if err := json.Unmarshal(part.Result, &agg); err != nil {
				return nil, fmt.Errorf("partition %s result: %w", part.ID, err)
			}
			merged.Merge(&agg)
		}
		return merged, nil
	}
	return nil, fmt.Errorf("%w: profile %s does not prepare raw artifacts", execution.ErrProfileUnavailable, profile)
}

// scan streams the raw bytes once, aggregates the rows keep accepts and
// re-verifies that the bytes still match the registered sha256 and size.
func (p *Preparation) scan(ctx context.Context, ref input.RawArtifactRef, spec input.PreparationSpec, keep input.Keep) (*input.Aggregate, error) {
	rc, err := p.Resolver.Open(ctx, ref.URI)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	v := input.NewVerifyingReader(rc, p.MaxRawBytes)
	agg, err := input.AggregateCSV(v, spec, keep, 0)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(io.Discard, v); err != nil {
		return nil, err
	}
	if _, err := v.Check(ref); err != nil {
		return nil, fmt.Errorf("raw artifact changed after registration: %w", err)
	}
	return agg, nil
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
