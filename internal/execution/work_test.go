package execution

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/input"
)

// TestMain doubles as the PROCESS worker: the dispatcher tests re-execute
// this test binary with INSIGHT_LAB_TEST_WORKER set, as the real server
// re-executes `insight-lab worker`.
func TestMain(m *testing.M) {
	if mode := os.Getenv("INSIGHT_LAB_TEST_WORKER"); mode != "" {
		os.Exit(testWorker(mode))
	}
	os.Exit(m.Run())
}

func testWorker(mode string) int {
	switch mode {
	case "crash":
		fmt.Fprintln(os.Stderr, "boom")
		return 3
	case "hang":
		time.Sleep(time.Minute)
		return 0
	case "echo-other-partition":
		var w WorkSpec
		_ = json.NewDecoder(os.Stdin).Decode(&w)
		_ = json.NewEncoder(os.Stdout).Encode(WorkResult{
			Version: w.Version, JobID: w.JobID, PartitionID: "bytes-0-1", Operation: w.Operation,
			OperationVersion: w.OperationVersion, ContentSHA256: w.Input.ContentSHA256,
		})
		return 0
	}
	if os.Getenv("INSIGHT_LAB_API_KEY") != "" {
		fmt.Fprintln(os.Stderr, "the worker inherited a secret")
		return 9
	}
	fs := flag.NewFlagSet("worker", flag.ContinueOnError)
	root := fs.String("input-root", "", "")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return 2
	}
	rs := input.Resolvers{"file": input.FileResolver{Root: *root}}
	if err := ServeWorker(context.Background(), os.Stdin, os.Stdout, rs); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	return 0
}

const workCSV = "month,region,value\n2026-01,east,1\n2026-01,\"we\nst\",2\n2026-02,east,\n2026-02,west,4\n"

var workPrepSpec = input.PreparationSpec{
	Kind:             input.PreparationKindCSVAggregate,
	Metrics:          []input.PreparationMetric{{ID: "v", Name: "Value", Column: "value", Aggregation: "sum", Unit: "u"}},
	DimensionColumns: []string{"region"},
	PeriodColumn:     "month",
	Population:       input.PreparationPopulation{Description: "rows"},
}

// planWork writes workCSV under a new root and returns one WorkSpec per
// shard with the given target size.
func planWork(t *testing.T, target int64) (string, []WorkSpec) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "d.csv"), []byte(workCSV), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := input.PlanCSVShards(context.Background(), strings.NewReader(workCSV), target)
	if err != nil {
		t.Fatal(err)
	}
	var specs []WorkSpec
	for _, s := range plan.Shards {
		specs = append(specs, NewCSVAggregateShardSpec("job-1", "file:d.csv", plan, s, workPrepSpec))
	}
	return root, specs
}

func processDispatcher(root string, env ...string) ProcessDispatcher {
	return ProcessDispatcher{
		Executable: os.Args[0], Args: []string{"-input-root", root},
		Env: append([]string{"INSIGHT_LAB_TEST_WORKER=serve"}, env...), Timeout: 20 * time.Second,
	}
}

func dispatchAll(t *testing.T, d Dispatcher, specs []WorkSpec) *input.Aggregate {
	t.Helper()
	merged := input.NewAggregate()
	for _, w := range specs {
		res, err := d.Dispatch(context.Background(), w)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := Accept(w, res)
		if err != nil {
			t.Fatal(err)
		}
		var agg input.Aggregate
		if err := json.Unmarshal(raw, &agg); err != nil {
			t.Fatal(err)
		}
		merged.Merge(&agg)
	}
	return merged
}

func TestProcessAndInProcessWorkersProduceTheSameMergedAggregate(t *testing.T) {
	t.Setenv("INSIGHT_LAB_API_KEY", "sk-must-not-leak")
	root, specs := planWork(t, 1)
	if len(specs) != 4 {
		t.Fatalf("planned %d shards, want one per record", len(specs))
	}
	whole, err := input.AggregateCSV(strings.NewReader(workCSV), workPrepSpec, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := json.Marshal(whole)
	local, _ := json.Marshal(dispatchAll(t, InProcess{Resolver: input.Resolvers{"file": input.FileResolver{Root: root}}}, specs))
	process, _ := json.Marshal(dispatchAll(t, processDispatcher(root), specs))
	if string(local) != string(want) || string(process) != string(want) {
		t.Fatalf("aggregates differ:\nwhole   %s\nlocal   %s\nprocess %s", want, local, process)
	}
}

func TestWorkSpecValidationAllowsOnlyTheKnownOperationAndAnIntactSpec(t *testing.T) {
	_, specs := planWork(t, 1<<20)
	good := specs[0]
	cases := map[string]func(*WorkSpec){
		"unknown operation":  func(w *WorkSpec) { w.Operation = "shell" },
		"unknown version":    func(w *WorkSpec) { w.OperationVersion = "2" },
		"wire version":       func(w *WorkSpec) { w.Version = "insight-lab.work/v0" },
		"tampered spec":      func(w *WorkSpec) { w.Spec = json.RawMessage(`{"kind":"other"}`) },
		"range before data":  func(w *WorkSpec) { w.Input.Offset = 0 },
		"unsafe partition":   func(w *WorkSpec) { w.PartitionID = "../x" },
		"missing shard hash": func(w *WorkSpec) { w.Input.ContentSHA256 = "" },
	}
	for name, mutate := range cases {
		w := good
		mutate(&w)
		if _, err := Execute(context.Background(), nil, w); !errors.Is(err, ErrWorkRejected) {
			t.Errorf("%s: %v, want ErrWorkRejected", name, err)
		}
	}
}

func TestWorkerCannotReadOutsideItsRootOrUseChangedBytes(t *testing.T) {
	root, specs := planWork(t, 1<<20)
	rs := input.Resolvers{"file": input.FileResolver{Root: root}}

	escape := specs[0]
	escape.Input.URI = "file:../d.csv"
	if res, err := Execute(context.Background(), rs, escape); err != nil || !strings.Contains(res.Error, input.ErrUnavailable.Error()) {
		t.Fatalf("escape: %+v %v", res, err)
	}

	if err := os.WriteFile(filepath.Join(root, "d.csv"), []byte(strings.Replace(workCSV, "4", "9", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := Execute(context.Background(), rs, specs[0])
	if err != nil || !strings.Contains(res.Error, "changed after planning") {
		t.Fatalf("mutated input: %+v %v", res, err)
	}
	if _, err := Accept(specs[0], res); err == nil {
		t.Fatal("a failed shard result was accepted")
	}
}

func TestAcceptRefusesAResultForAnotherPartitionOrWithABrokenHash(t *testing.T) {
	root, specs := planWork(t, 1)
	res, err := InProcess{Resolver: input.Resolvers{"file": input.FileResolver{Root: root}}}.Dispatch(context.Background(), specs[0])
	if err != nil {
		t.Fatal(err)
	}
	other := res
	other.PartitionID = specs[1].PartitionID
	if _, err := Accept(specs[0], other); !errors.Is(err, ErrWorkRejected) {
		t.Fatalf("result for another partition: %v", err)
	}
	broken := res
	broken.Result = json.RawMessage(`{}`)
	if _, err := Accept(specs[0], broken); !errors.Is(err, ErrWorkRejected) {
		t.Fatalf("result with a broken hash: %v", err)
	}
	stale := processDispatcher(root)
	stale.Env = []string{"INSIGHT_LAB_TEST_WORKER=echo-other-partition"}
	echoed, err := stale.Dispatch(context.Background(), specs[0])
	if err != nil {
		t.Fatal(err) // transport succeeds; Accept is what refuses it
	}
	if _, err := Accept(specs[0], echoed); !errors.Is(err, ErrWorkRejected) {
		t.Fatalf("a worker answering another partition: %v", err)
	}
}

func TestProcessDispatcherReportsCrashesAndTimeouts(t *testing.T) {
	root, specs := planWork(t, 1<<20)
	crash := processDispatcher(root)
	crash.Env = []string{"INSIGHT_LAB_TEST_WORKER=crash"}
	if _, err := crash.Dispatch(context.Background(), specs[0]); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("crash: %v", err)
	}
	hang := processDispatcher(root)
	hang.Env, hang.Timeout = []string{"INSIGHT_LAB_TEST_WORKER=hang"}, 300*time.Millisecond
	start := time.Now()
	if _, err := hang.Dispatch(context.Background(), specs[0]); err == nil || !strings.Contains(err.Error(), "exceeded") {
		t.Fatalf("timeout: %v", err)
	}
	if time.Since(start) > 10*time.Second {
		t.Fatal("the hung worker was not killed")
	}
}

func TestLocalRuntimeRetriesACrashedProcessPartitionAndCancelStopsTheJob(t *testing.T) {
	root, specs := planWork(t, 1)
	rt := NewLocalRuntime(t.TempDir(), 2)
	byID := map[string]WorkSpec{}
	job := JobSpec{ID: "job-1"}
	for _, w := range specs {
		byID[w.PartitionID] = w
		job.Partitions = append(job.Partitions, w.PartitionID)
	}
	attempts := map[string]int{}
	var mu = make(chan struct{}, 1)
	good, crash := processDispatcher(root), processDispatcher(root)
	crash.Env = []string{"INSIGHT_LAB_TEST_WORKER=crash"}
	state, err := rt.Run(context.Background(), job, func(ctx context.Context, id string) (json.RawMessage, error) {
		mu <- struct{}{}
		attempts[id]++
		first := attempts[id] == 1
		<-mu
		d := good
		if first && id == specs[0].PartitionID {
			d = crash // the first attempt of one partition crashes
		}
		res, err := d.Dispatch(ctx, byID[id])
		if err != nil {
			return nil, err
		}
		return Accept(byID[id], res)
	})
	if err != nil || state.Status != JobSucceeded || state.Partitions[0].Attempts != 2 {
		t.Fatalf("state = %+v, err = %v", state, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	hang := processDispatcher(root)
	hang.Env = []string{"INSIGHT_LAB_TEST_WORKER=hang"}
	if _, err := NewLocalRuntime(t.TempDir(), 1).Run(ctx, JobSpec{ID: "job-2", Partitions: job.Partitions}, func(ctx context.Context, id string) (json.RawMessage, error) {
		res, err := hang.Dispatch(ctx, byID[id])
		if err != nil {
			return nil, err
		}
		return Accept(byID[id], res)
	}); err == nil {
		t.Fatal("a cancelled job succeeded")
	}
}
