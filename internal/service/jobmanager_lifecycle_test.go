package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/llm"
	"insight-lab/internal/repository/sqlite"
)

var lifecycleModelSettings = Settings{BaseURL: "http://model.invalid/v1", Model: "m1"}

// awaitFinished polls until the analysis reaches a terminal status.
func (h *jobHarness) awaitFinished(t *testing.T, id string) *domain.Analysis {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		a, err := h.analyses.Get(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if a.Finished() {
			return a
		}
		if time.Now().After(deadline) {
			t.Fatalf("analysis %s did not finish (status %s)", id, a.Status)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (h *jobHarness) start(t *testing.T, workers int) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	h.jobs.Start(ctx, workers)
	var once sync.Once
	stop := func() { once.Do(func() { cancel(); h.jobs.Wait() }) }
	t.Cleanup(stop)
	return stop
}

// restart builds a second JobManager over the same database, as a new
// process would: nothing enqueued in memory survives.
func (h *jobHarness) restart(t *testing.T, settings Settings) *jobHarness {
	t.Helper()
	next := *h
	next.settings = NewSettingsStore(settings)
	next.jobs = NewJobManager(h.analyses, h.jobs.pipeline, next.settings, func(Settings) llm.Client { return newFakeLLM() })
	next.jobs.build = testBuild
	if _, err := next.jobs.RecoverInterrupted(context.Background()); err != nil {
		t.Fatal(err)
	}
	return &next
}

// blockingLLM blocks every call until its context ends, after announcing
// the first call on entered.
type blockingLLM struct {
	once    sync.Once
	entered chan struct{}
}

func (b *blockingLLM) Generate(ctx context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, error) {
	b.once.Do(func() { close(b.entered) })
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestQueuedAnalysisResumesAfterRestartWhenItsConfigurationIsReproducible(t *testing.T) {
	h := newJobHarness(t, datasetJobDocs(), Settings{})
	id := h.enqueue(t, EnqueueRequest{Label: "before restart"})

	after := h.restart(t, Settings{})
	after.start(t, 1)
	a := after.awaitFinished(t, id)
	if a.Status != domain.AnalysisCompleted || a.Lifecycle() != domain.LifecycleSucceeded {
		t.Fatalf("resumed analysis = %s/%s (%s)", a.Status, a.FailureCode, a.Error)
	}
}

func TestQueuedAnalysisNeedsRequeueAfterRestartWhenSettingsNoLongerMatch(t *testing.T) {
	h := newJobHarness(t, datasetJobDocs(), Settings{})
	id := h.enqueue(t, EnqueueRequest{})

	after := h.restart(t, lifecycleModelSettings) // now model-backed: a different execution
	after.start(t, 1)
	a := after.awaitFinished(t, id)
	if a.Status != domain.AnalysisFailed || a.FailureCode != domain.FailureNeedsRequeue || a.Lifecycle() != domain.LifecycleInterrupted {
		t.Fatalf("analysis = %s/%s lifecycle %s", a.Status, a.FailureCode, a.Lifecycle())
	}
	if a.Metrics != "" {
		t.Fatal("a refused run must not carry results")
	}
}

func TestRunningAnalysisIsInterruptedNotResumedAfterACrash(t *testing.T) {
	h := newJobHarness(t, datasetJobDocs(), Settings{})
	id := h.enqueue(t, EnqueueRequest{})
	if _, err := h.analyses.ClaimNextQueued(context.Background()); err != nil { // the crashed worker's claim
		t.Fatal(err)
	}

	after := h.restart(t, Settings{})
	after.start(t, 1)
	a, _ := after.analyses.Get(context.Background(), id)
	if a.Status != domain.AnalysisFailed || a.FailureCode != domain.FailureInterrupted || a.Error == "" {
		t.Fatalf("analysis after restart = %s/%s %q", a.Status, a.FailureCode, a.Error)
	}
}

func TestShutdownMidRunLeavesTheRunForRecoveryInsteadOfFailingOrCompletingIt(t *testing.T) {
	h := newJobHarness(t, datasetJobDocs(), lifecycleModelSettings)
	block := &blockingLLM{entered: make(chan struct{})}
	h.jobs.newLLMClient = func(Settings) llm.Client { return block }
	id := h.enqueue(t, EnqueueRequest{})
	stop := h.start(t, 1)
	<-block.entered
	stop()

	a, _ := h.analyses.Get(context.Background(), id)
	if a.Status != domain.AnalysisRunning {
		t.Fatalf("status after shutdown = %s, want running until recovery", a.Status)
	}
	if n, err := h.analyses.FailInterrupted(context.Background()); err != nil || n != 1 {
		t.Fatalf("recovery touched %d rows: %v", n, err)
	}
}

func TestCancelStopsAQueuedAnalysisBeforeItRuns(t *testing.T) {
	h := newJobHarness(t, datasetJobDocs(), Settings{})
	id := h.enqueue(t, EnqueueRequest{})
	a, err := h.jobs.Cancel(context.Background(), id)
	if err != nil || a.Lifecycle() != domain.LifecycleCancelled || a.Status != domain.AnalysisFailed {
		t.Fatalf("cancel queued = %+v %v", a, err)
	}
	h.start(t, 1)
	time.Sleep(50 * time.Millisecond)
	if got, _ := h.analyses.Get(context.Background(), id); got.Lifecycle() != domain.LifecycleCancelled || got.StartedAt != nil {
		t.Fatalf("a cancelled queued run was started: %+v", got)
	}
}

func TestCancelReachesARunningAnalysisAndFinishesItAsCancelled(t *testing.T) {
	h := newJobHarness(t, datasetJobDocs(), lifecycleModelSettings)
	block := &blockingLLM{entered: make(chan struct{})}
	h.jobs.newLLMClient = func(Settings) llm.Client { return block }
	id := h.enqueue(t, EnqueueRequest{})
	h.start(t, 1)
	<-block.entered

	requested, err := h.jobs.Cancel(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if l := requested.Lifecycle(); l != domain.LifecycleCancelRequested && l != domain.LifecycleCancelled {
		t.Fatalf("lifecycle right after cancel = %s", l)
	}
	a := h.awaitFinished(t, id)
	if a.Lifecycle() != domain.LifecycleCancelled || a.Metrics != "" {
		t.Fatalf("cancelled run = %s/%s metrics=%q", a.Status, a.FailureCode, a.Metrics)
	}
	if _, err := h.jobs.Cancel(context.Background(), id); !errors.Is(err, ErrAnalysisFinished) {
		t.Fatalf("cancelling a finished run: %v", err)
	}
}

// cancelOnFirstProgress records a cancel request directly in the table
// (without stopping the run's context) on the first progress write, so the
// deterministic pipeline completes although a cancel was requested before
// it finished.
type cancelOnFirstProgress struct {
	*sqlite.AnalysisRepository
	once sync.Once
	t    *testing.T
}

func (c *cancelOnFirstProgress) Update(ctx context.Context, a *domain.Analysis) error {
	c.once.Do(func() {
		if ok, err := c.RequestCancel(ctx, a.ID); err != nil || !ok {
			c.t.Errorf("request cancel: ok=%v err=%v", ok, err)
		}
	})
	return c.AnalysisRepository.Update(ctx, a)
}

func TestCompletionLosesToACancelRequestedBeforeTheRunFinished(t *testing.T) {
	h := newJobHarness(t, datasetJobDocs(), Settings{})
	h.jobs.analyses = &cancelOnFirstProgress{AnalysisRepository: h.analyses, t: t}
	id := h.enqueue(t, EnqueueRequest{})
	h.start(t, 1)
	a := h.awaitFinished(t, id)
	if a.Status == domain.AnalysisCompleted || a.Lifecycle() != domain.LifecycleCancelled || a.Metrics != "" {
		t.Fatalf("run = %s/%s lifecycle %s metrics=%q err=%q; a requested cancel must not become success", a.Status, a.FailureCode, a.Lifecycle(), a.Metrics, a.Error)
	}
}

func TestRetryEnqueuesAFailedAnalysisOnceWithTheSameRequest(t *testing.T) {
	h := newJobHarness(t, datasetJobDocs(), Settings{})
	id := h.enqueue(t, EnqueueRequest{Label: "L", Note: "N", ResearchQuestion: "Why?", OutputLocale: domain.OutputLocaleJaJP})
	if _, err := h.jobs.Cancel(context.Background(), id); err != nil {
		t.Fatal(err)
	}

	retry, created, err := h.jobs.Retry(context.Background(), id)
	if err != nil || !created {
		t.Fatalf("retry: created=%v err=%v", created, err)
	}
	if retry.RetryOf != id || retry.Label != "L" || retry.Note != "N" || retry.ResearchQuestion != "Why?" || retry.OutputLocale != domain.OutputLocaleJaJP || retry.Status != domain.AnalysisQueued {
		t.Fatalf("retry = %+v", retry)
	}
	again, created, err := h.jobs.Retry(context.Background(), id)
	if err != nil || created || again.ID != retry.ID {
		t.Fatalf("second retry created=%v id=%s err=%v, want the existing %s", created, again.ID, err, retry.ID)
	}

	h.start(t, 1)
	done := h.awaitFinished(t, retry.ID)
	if done.Status != domain.AnalysisCompleted {
		t.Fatalf("retry run = %s (%s)", done.Status, done.Error)
	}
	if _, _, err := h.jobs.Retry(context.Background(), retry.ID); !errors.Is(err, ErrAnalysisNotRetryable) {
		t.Fatalf("retrying a completed run: %v", err)
	}
}

func TestEnqueueRejectsRunsBeyondTheQueueBound(t *testing.T) {
	h := newJobHarness(t, datasetJobDocs(), Settings{})
	h.jobs.MaxQueued = 1
	h.enqueue(t, EnqueueRequest{})
	if _, err := h.jobs.Enqueue(context.Background(), EnqueueRequest{ProjectID: "proj_1"}); !errors.Is(err, ErrAnalysisQueueFull) {
		t.Fatalf("enqueue over the bound: %v", err)
	}
}
