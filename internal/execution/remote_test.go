package execution

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"insight-lab/internal/input"
)

// startWorkers runs n remote workers on b until the test ends.
func startWorkers(t *testing.T, b Broker, root string, n int) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rs := input.Resolvers{"file": input.FileResolver{Root: root}}
			if err := RunRemoteWorker(ctx, b, rs, "w"+string(rune('0'+i))); err != nil {
				t.Errorf("worker: %v", err)
			}
		}(i)
	}
	t.Cleanup(func() { cancel(); wg.Wait() })
}

func TestRemoteWorkersProduceTheSingleProcessAggregate(t *testing.T) {
	root, specs := planWork(t, 1)
	b := NewMemoryBroker(time.Second)
	startWorkers(t, b, root, 2)
	whole, _ := input.AggregateCSV(strings.NewReader(workCSV), workPrepSpec, nil, 0)
	want, _ := json.Marshal(whole)
	got, _ := json.Marshal(dispatchAll(t, RemoteDispatcher{Broker: b, Timeout: 10 * time.Second}, specs))
	if string(got) != string(want) {
		t.Fatalf("remote aggregate:\n%s\nwant\n%s", got, want)
	}
}

// takeAndVanish takes one task and never answers or acknowledges it, like a
// worker that crashed mid-task.
func takeAndVanish(t *testing.T, b Broker) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := b.NextTask(ctx); err != nil {
		t.Error(err)
	}
}

func TestRemoteTaskIsRedeliveredWhenAWorkerCrashesBeforeAcknowledging(t *testing.T) {
	root, specs := planWork(t, 1<<20)
	b := NewMemoryBroker(50 * time.Millisecond)
	done := make(chan struct{})
	go func() { defer close(done); takeAndVanish(t, b) }()
	go func() { <-done; startWorkers(t, b, root, 1) }()
	res, err := RemoteDispatcher{Broker: b, Timeout: 10 * time.Second}.Dispatch(context.Background(), specs[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Accept(specs[0], res); err != nil {
		t.Fatalf("redelivered result: %v", err)
	}
}

// answerWithoutAck publishes a correct result but never acknowledges the
// task, like a worker that crashed between publishing and acknowledging.
func answerWithoutAck(t *testing.T, b Broker, root string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	task, _, err := b.NextTask(ctx)
	if err != nil {
		t.Error(err)
		return
	}
	res, _ := Execute(ctx, input.Resolvers{"file": input.FileResolver{Root: root}}, task.Work)
	_ = b.PublishResult(ctx, TaskResult{Token: task.Token, Result: res})
}

func TestRemoteDuplicateCompletionAfterRedeliveryIsHarmless(t *testing.T) {
	root, specs := planWork(t, 1<<20)
	b := NewMemoryBroker(50 * time.Millisecond)
	go answerWithoutAck(t, b, root)
	res, err := RemoteDispatcher{Broker: b, Timeout: 10 * time.Second}.Dispatch(context.Background(), specs[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Accept(specs[0], res); err != nil {
		t.Fatal(err)
	}
	startWorkers(t, b, root, 1) // takes the redelivery and answers again
	deadline := time.Now().Add(5 * time.Second)
	for b.Pending() > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if b.Pending() != 0 {
		t.Fatal("the redelivered task was never acknowledged")
	}
}

func TestRemoteLateResultOfATimedOutAttemptIsIgnored(t *testing.T) {
	root, specs := planWork(t, 1)
	b := NewMemoryBroker(time.Minute)
	d := RemoteDispatcher{Broker: b, Timeout: 100 * time.Millisecond}

	// Attempt 1 times out; its task is held by a slow worker.
	var stale Task
	taken := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		stale, _, _ = b.NextTask(ctx)
		close(taken)
	}()
	if _, err := d.Dispatch(context.Background(), specs[0]); err == nil || !strings.Contains(err.Error(), "no remote result") {
		t.Fatalf("attempt 1: %v", err)
	}
	<-taken

	// Attempt 2 (a new token) must ignore attempt 1's late, wrong answer.
	d.Timeout = 10 * time.Second
	type out struct {
		res WorkResult
		err error
	}
	got := make(chan out, 1)
	go func() {
		res, err := d.Dispatch(context.Background(), specs[0])
		got <- out{res, err}
	}()
	time.Sleep(50 * time.Millisecond)
	late := WorkResult{Version: WorkSpecVersion, JobID: specs[0].JobID, PartitionID: specs[0].PartitionID,
		Operation: specs[0].Operation, OperationVersion: specs[0].OperationVersion, Error: "stale attempt"}
	_ = b.PublishResult(context.Background(), TaskResult{Token: stale.Token, Result: late})
	startWorkers(t, b, root, 1)
	r := <-got
	if r.err != nil {
		t.Fatal(r.err)
	}
	if _, err := Accept(specs[0], r.res); err != nil {
		t.Fatalf("attempt 2 took the stale answer: %v", err)
	}
}

func TestRemoteWorkerAnswersAMalformedWorkSpecWithAnErrorAndNeverRunsIt(t *testing.T) {
	root, specs := planWork(t, 1<<20)
	b := NewMemoryBroker(time.Second)
	bad := specs[0]
	bad.Operation = "shell"
	results, stop, _ := b.SubscribeResults(context.Background(), bad.JobID)
	defer stop()
	_ = b.PublishTask(context.Background(), Task{Token: "t1", Work: bad})
	startWorkers(t, b, root, 1)
	select {
	case r := <-results:
		if !strings.Contains(r.Result.Error, ErrWorkRejected.Error()) || r.Result.Result != nil {
			t.Fatalf("malformed spec answer = %+v", r.Result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no answer to a malformed spec")
	}
	if _, err := (RemoteDispatcher{Broker: b}).Dispatch(context.Background(), bad); !errors.Is(err, ErrWorkRejected) {
		t.Fatalf("the coordinator must not publish a malformed spec: %v", err)
	}
}

func TestRemoteDispatchStopsWhenCancelled(t *testing.T) {
	_, specs := planWork(t, 1<<20)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	if _, err := (RemoteDispatcher{Broker: NewMemoryBroker(time.Second)}).Dispatch(ctx, specs[0]); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled dispatch: %v", err)
	}
}

// countingBroker counts published tasks.
type countingBroker struct {
	*MemoryBroker
	published atomic.Int32
}

func (c *countingBroker) PublishTask(ctx context.Context, t Task) error {
	c.published.Add(1)
	return c.MemoryBroker.PublishTask(ctx, t)
}

func TestCoordinatorRestartResumesOnlyUnfinishedRemotePartitions(t *testing.T) {
	root, specs := planWork(t, 1)
	stateDir := t.TempDir()
	byID := map[string]WorkSpec{}
	job := JobSpec{ID: "job-1"}
	for _, w := range specs {
		byID[w.PartitionID] = w
		job.Partitions = append(job.Partitions, w.PartitionID)
	}
	b := &countingBroker{MemoryBroker: NewMemoryBroker(time.Second)}
	startWorkers(t, b, root, 2)
	run := func(ctx context.Context, rt *LocalRuntime, failAfter int32) (JobState, error) {
		var calls atomic.Int32
		return rt.Run(ctx, job, func(ctx context.Context, id string) (json.RawMessage, error) {
			if failAfter > 0 && calls.Add(1) > failAfter {
				return nil, errors.New("coordinator stopped")
			}
			res, err := RemoteDispatcher{Broker: b, Timeout: 10 * time.Second}.Dispatch(ctx, byID[id])
			if err != nil {
				return nil, err
			}
			return Accept(byID[id], res)
		})
	}
	first := NewLocalRuntime(stateDir, 1)
	first.MaxAttempts = 1
	if _, err := run(context.Background(), first, 2); err == nil {
		t.Fatal("the interrupted job succeeded")
	}
	before := b.published.Load()
	state, err := run(context.Background(), NewLocalRuntime(stateDir, 2), 0) // a new coordinator process
	if err != nil || state.Status != JobSucceeded {
		t.Fatalf("resumed job: %+v %v", state, err)
	}
	if again := b.published.Load() - before; again != int32(len(specs))-2 {
		t.Fatalf("resume republished %d tasks, want only the %d unfinished", again, len(specs)-2)
	}
}
