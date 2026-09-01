package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/llm"
	"insight-lab/internal/repository"
)

type SSEEvent struct {
	Event string
	Data  string
}

// JobManager runs analyses asynchronously (a fixed pool of worker
// goroutines pulling from an in-memory queue - no external queue system;
// see docs/detailed-design.md §10) and fans progress out to any number of
// SSE subscribers per analysis. Every state transition is also written to
// the analyses table, so a browser refresh or an SSE reconnect can recover
// current status via GET /api/analysis/{id} instead of depending on the
// in-memory channel.
type JobManager struct {
	analyses     repository.AnalysisRepository
	pipeline     *Pipeline
	settings     *SettingsStore
	newLLMClient func(Settings) llm.Client

	mu          sync.Mutex
	subscribers map[string]map[chan SSEEvent]struct{}

	queue chan string
	wg    sync.WaitGroup
}

func NewJobManager(analyses repository.AnalysisRepository, pipeline *Pipeline, settings *SettingsStore, newClient func(Settings) llm.Client) *JobManager {
	return &JobManager{
		analyses:     analyses,
		pipeline:     pipeline,
		settings:     settings,
		newLLMClient: newClient,
		subscribers:  map[string]map[chan SSEEvent]struct{}{},
		queue:        make(chan string, 32),
	}
}

func DefaultLLMClientFactory(s Settings) llm.Client {
	return llm.NewOpenAIClient(s.BaseURL, s.APIKey, s.Model)
}

// RecoverInterrupted marks any analysis left "queued"/"running" by a
// process that exited mid-run as failed. Call once at startup, before
// Start.
func (m *JobManager) RecoverInterrupted(ctx context.Context) (int, error) {
	return m.analyses.FailInterrupted(ctx)
}

func (m *JobManager) Start(ctx context.Context, workers int) {
	for i := 0; i < workers; i++ {
		m.wg.Add(1)
		go m.worker(ctx)
	}
}

func (m *JobManager) Wait() { m.wg.Wait() }

func (m *JobManager) worker(ctx context.Context) {
	defer m.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case analysisID, ok := <-m.queue:
			if !ok {
				return
			}
			m.run(ctx, analysisID)
		}
	}
}

// Enqueue creates the analysis row (status "queued") and schedules it for
// a worker to pick up.
func (m *JobManager) Enqueue(ctx context.Context, projectID string) (*domain.Analysis, error) {
	a := &domain.Analysis{ID: newID("ana"), ProjectID: projectID, Status: domain.AnalysisQueued, CreatedAt: time.Now().UTC()}
	if err := m.analyses.Create(ctx, a); err != nil {
		return nil, err
	}
	m.queue <- a.ID
	return a, nil
}

func (m *JobManager) run(ctx context.Context, analysisID string) {
	a, err := m.analyses.Get(ctx, analysisID)
	if err != nil {
		return // analysis row is gone (e.g. project was deleted); nothing to run
	}

	settings := m.settings.Get()
	if !settings.Configured() {
		m.fail(ctx, a, fmt.Errorf("LLMが設定されていません。設定画面でBase URLとModelを入力してください"))
		return
	}

	pipeline := &Pipeline{
		Documents: m.pipeline.Documents, Observations: m.pipeline.Observations,
		Patterns: m.pipeline.Patterns, Insights: m.pipeline.Insights, Evidence: m.pipeline.Evidence,
		LLM: m.newLLMClient(settings),
	}

	now := time.Now().UTC()
	a.Status = domain.AnalysisRunning
	a.StartedAt = &now
	_ = m.analyses.Update(ctx, a)
	m.broadcast(a.ID, SSEEvent{Event: "progress", Data: progressJSON("starting", 0, "解析を開始しています...")})

	metrics, err := pipeline.Run(ctx, a.ID, a.ProjectID, func(step string, progress int, message string) {
		a.CurrentStep = step
		a.Progress = progress
		_ = m.analyses.Update(ctx, a)
		m.broadcast(a.ID, SSEEvent{Event: "progress", Data: progressJSON(step, progress, message)})
	})
	if err != nil {
		m.fail(ctx, a, err)
		return
	}

	metricsJSON, _ := json.Marshal(metrics)
	finished := time.Now().UTC()
	a.Status = domain.AnalysisCompleted
	a.Progress = 100
	a.CurrentStep = "completed"
	a.Metrics = string(metricsJSON)
	a.FinishedAt = &finished
	_ = m.analyses.Update(ctx, a)
	m.broadcast(a.ID, SSEEvent{Event: "completed", Data: fmt.Sprintf(`{"progress":100,"insightCount":%d}`, metrics.FinalInsightCount)})
}

func (m *JobManager) fail(ctx context.Context, a *domain.Analysis, runErr error) {
	finished := time.Now().UTC()
	a.Status = domain.AnalysisFailed
	a.Error = runErr.Error()
	a.FinishedAt = &finished
	_ = m.analyses.Update(ctx, a)

	msg, _ := json.Marshal(map[string]string{"step": a.CurrentStep, "message": runErr.Error()})
	m.broadcast(a.ID, SSEEvent{Event: "error", Data: string(msg)})
}

func progressJSON(step string, progress int, message string) string {
	b, _ := json.Marshal(map[string]any{"step": step, "progress": progress, "message": message})
	return string(b)
}

// Subscribe registers a channel that receives every SSEEvent broadcast for
// analysisID from now on. Callers must call Unsubscribe when done (e.g.
// when the HTTP client disconnects).
func (m *JobManager) Subscribe(analysisID string) chan SSEEvent {
	ch := make(chan SSEEvent, 16)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.subscribers[analysisID] == nil {
		m.subscribers[analysisID] = map[chan SSEEvent]struct{}{}
	}
	m.subscribers[analysisID][ch] = struct{}{}
	return ch
}

func (m *JobManager) Unsubscribe(analysisID string, ch chan SSEEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if subs, ok := m.subscribers[analysisID]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(m.subscribers, analysisID)
		}
	}
	close(ch)
}

func (m *JobManager) broadcast(analysisID string, ev SSEEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for ch := range m.subscribers[analysisID] {
		select {
		case ch <- ev:
		default: // a slow subscriber must never block the pipeline
		}
	}
}
