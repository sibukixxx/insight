//go:build jetstream

package jetstream

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"insight-lab/internal/execution"
	"insight-lab/internal/input"
)

// Needs a JetStream-enabled server, e.g.
//   docker run --rm -p 4222:4222 nats:2 -js
//   INSIGHT_LAB_NATS_URL=nats://127.0.0.1:4222 go test -tags jetstream ./internal/execution/jetstream/
func natsURL(t *testing.T) string {
	u := os.Getenv("INSIGHT_LAB_NATS_URL")
	if u == "" {
		t.Skip("INSIGHT_LAB_NATS_URL is not set")
	}
	return u + "?ackWait=1s"
}

const csv = "month,region,value\n2026-01,east,1\n2026-01,\"we\nst\",2\n2026-02,east,\n2026-02,west,4\n"

func TestJetStreamTwoWorkersProduceTheSingleProcessAggregateAndSurviveACrashedWorker(t *testing.T) {
	url := natsURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "d.csv"), []byte(csv), 0o600); err != nil {
		t.Fatal(err)
	}
	spec := input.PreparationSpec{Kind: input.PreparationKindCSVAggregate, Metrics: []input.PreparationMetric{{ID: "v", Name: "V", Column: "value", Aggregation: "sum", Unit: "u"}}, DimensionColumns: []string{"region"}, PeriodColumn: "month", Population: input.PreparationPopulation{Description: "rows"}}
	plan, err := input.PlanCSVShards(ctx, strings.NewReader(csv), 1)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(time.Now().String()))
	jobID := "js-" + hex.EncodeToString(sum[:6])

	coordinator, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer coordinator.Close()

	// A worker that takes a task and dies without acknowledging it.
	crashed, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	workerCtx, stopWorkers := context.WithCancel(ctx)
	for i := 0; i < 2; i++ {
		b, err := Open(ctx, url)
		if err != nil {
			t.Fatal(err)
		}
		defer b.Close()
		wg.Add(1)
		go func(b execution.Broker) {
			defer wg.Done()
			_ = execution.RunRemoteWorker(workerCtx, b, input.Resolvers{"file": input.FileResolver{Root: root}}, "js-worker")
		}(b)
	}
	defer func() { stopWorkers(); wg.Wait() }()

	d := execution.RemoteDispatcher{Broker: coordinator, Timeout: 20 * time.Second}
	merged := input.NewAggregate()
	for i, shard := range plan.Shards {
		w := execution.NewCSVAggregateShardSpec(jobID, "file:d.csv", plan, shard, spec)
		if i == 0 {
			go func() { _, _, _ = crashed.NextTask(ctx); crashed.Close() }()
		}
		res, err := d.Dispatch(ctx, w)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := execution.Accept(w, res)
		if err != nil {
			t.Fatal(err)
		}
		var agg input.Aggregate
		if err := json.Unmarshal(raw, &agg); err != nil {
			t.Fatal(err)
		}
		merged.Merge(&agg)
	}
	whole, _ := input.AggregateCSV(strings.NewReader(csv), spec, nil, 0)
	a, _ := json.Marshal(merged)
	b, _ := json.Marshal(whole)
	if string(a) != string(b) {
		t.Fatalf("jetstream aggregate:\n%s\nwant\n%s", a, b)
	}
}
