package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
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
	Resolver input.Resolver
	Heavy    execution.Runtime
	// Dispatcher places HEAVY partitions (#134): nil runs them in this
	// process; a ProcessDispatcher runs each in a child worker process.
	Dispatcher execution.Dispatcher
	// Partitions is the target number of HEAVY shards; zero means 4.
	Partitions int
	// ShardBytes, when positive, fixes the shard size instead.
	ShardBytes  int64
	MaxRawBytes int64
	// Concurrency bounds how many raw artifacts are prepared at once
	// (STANDARD bounded concurrency, #91). Zero means 2. Output order and
	// content never depend on it.
	Concurrency int
}

func (p *Preparation) Capabilities() execution.Capabilities {
	return execution.Capabilities{HeavyRuntime: p != nil && p.Heavy != nil}
}

// RuntimeMode is where HEAVY partitions run.
func (p *Preparation) RuntimeMode() execution.RuntimeMode {
	return p.dispatcher().Mode()
}

func (p *Preparation) dispatcher() execution.Dispatcher {
	if p != nil && p.Dispatcher != nil {
		return p.Dispatcher
	}
	var rs input.Resolver
	if p != nil {
		rs = p.Resolver
	}
	return execution.InProcess{Resolver: rs}
}

// Prepare returns new artifact documents for raw references that have a
// preparation spec and no prepared artifact yet. It never modifies docs.
// Independent raw artifacts are prepared with bounded concurrency; results
// keep input order, and the first failure cancels the rest.
func (p *Preparation) Prepare(ctx context.Context, projectID string, docs []*domain.Document, profile execution.Profile, now time.Time) ([]*domain.Document, error) {
	type task struct {
		doc  *domain.Document
		ref  input.RawArtifactRef
		spec input.PreparationSpec
		id   string
	}
	prepared := map[string]bool{}
	for _, d := range docs {
		if id := d.Metadata[MetadataAnalyticalArtifactID]; id != "" {
			prepared[id] = true
		}
	}
	var tasks []task
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
		prepared[id] = true
		tasks = append(tasks, task{doc: d, ref: ref, spec: spec, id: id})
	}
	if len(tasks) == 0 {
		return nil, nil
	}

	limit := p.Concurrency
	if limit <= 0 {
		limit = 2
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	out := make([]*domain.Document, len(tasks))
	errs := make([]error, len(tasks))
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i, t := range tasks {
		wg.Add(1)
		go func(i int, t task) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				errs[i] = ctx.Err()
				return
			}
			defer func() { <-sem }()
			doc, err := p.prepareOne(ctx, projectID, t.doc, t.ref, t.spec, t.id, profile, now)
			if err != nil {
				errs[i] = err
				cancel()
				return
			}
			out[i] = doc
		}(i, t)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil && !errors.Is(err, context.Canceled) {
			return nil, err
		}
	}
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (p *Preparation) prepareOne(ctx context.Context, projectID string, d *domain.Document, ref input.RawArtifactRef, spec input.PreparationSpec, id string, profile execution.Profile, now time.Time) (*domain.Document, error) {
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
	return doc, nil
}

func (p *Preparation) aggregate(ctx context.Context, jobID string, ref input.RawArtifactRef, spec input.PreparationSpec, profile execution.Profile) (*input.Aggregate, error) {
	switch profile {
	case execution.ProfileStandard:
		return p.scan(ctx, ref, spec, nil)
	case execution.ProfileHeavy:
		if p.Heavy == nil {
			return nil, fmt.Errorf("%w: HEAVY requires a configured Heavy Execution Adapter", execution.ErrProfileUnavailable)
		}
		// One bounded scan plans whole-record byte shards and verifies the
		// registered bytes; each partition then reads only its own shard.
		plan, err := p.planShards(ctx, ref)
		if err != nil {
			return nil, err
		}
		if len(plan.Shards) == 0 {
			return input.NewAggregate(), nil
		}
		job := execution.JobSpec{ID: jobID}
		work := make(map[string]execution.WorkSpec, len(plan.Shards))
		for _, shard := range plan.Shards {
			w := execution.NewCSVAggregateShardSpec(jobID, ref.URI, plan, shard, spec)
			job.Partitions = append(job.Partitions, w.PartitionID)
			work[w.PartitionID] = w
		}
		dispatcher := p.dispatcher()
		state, err := p.Heavy.Run(ctx, job, func(ctx context.Context, partition string) (json.RawMessage, error) {
			w, ok := work[partition]
			if !ok {
				return nil, fmt.Errorf("unknown partition %q", partition)
			}
			res, err := dispatcher.Dispatch(ctx, w)
			if err != nil {
				return nil, err
			}
			return execution.Accept(w, res)
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

// planShards scans the raw bytes once to cut whole-record shards and
// checks them against the registered sha256 and size.
func (p *Preparation) planShards(ctx context.Context, ref input.RawArtifactRef) (input.CSVShardPlan, error) {
	rc, err := p.Resolver.Open(ctx, ref.URI)
	if err != nil {
		return input.CSVShardPlan{}, err
	}
	defer rc.Close()
	v := input.NewVerifyingReader(rc, p.MaxRawBytes)
	target := p.ShardBytes
	if target <= 0 {
		k := p.Partitions
		if k <= 0 {
			k = 4
		}
		target = max(ref.SizeBytes/int64(k)+1, 1<<20)
	}
	plan, err := input.PlanCSVShards(ctx, v, target)
	if err != nil {
		return input.CSVShardPlan{}, err
	}
	if _, err := v.Check(ref); err != nil {
		return input.CSVShardPlan{}, fmt.Errorf("raw artifact changed after registration: %w", err)
	}
	return plan, nil
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
