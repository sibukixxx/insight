package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"insight-lab/internal/buildinfo"
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
	build        buildinfo.Info

	mu          sync.Mutex
	subscribers map[string]map[chan SSEEvent]struct{}
	// pending holds, in memory only, the settings each queued run was
	// enqueued with. The API key never leaves this map.
	pending map[string]Settings

	queue chan string
	wg    sync.WaitGroup
}

func NewJobManager(analyses repository.AnalysisRepository, pipeline *Pipeline, settings *SettingsStore, newClient func(Settings) llm.Client) *JobManager {
	return &JobManager{
		analyses:     analyses,
		pipeline:     pipeline,
		settings:     settings,
		newLLMClient: newClient,
		build:        buildinfo.Get(),
		subscribers:  map[string]map[chan SSEEvent]struct{}{},
		pending:      map[string]Settings{},
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

// EnqueueRequest describes a run to schedule. Label, Note and
// SemanticAnalysisMode are optional.
type EnqueueRequest struct {
	ProjectID            string
	Label                string
	Note                 string
	SemanticAnalysisMode domain.AnalysisMode
}

// Enqueue creates the analysis row (status "queued") with the execution
// snapshot of the current settings, and schedules it for a worker. The run
// later executes with exactly these settings, even if they change while it
// waits in the queue.
func (m *JobManager) Enqueue(ctx context.Context, req EnqueueRequest) (*domain.Analysis, error) {
	if req.SemanticAnalysisMode != "" && !req.SemanticAnalysisMode.Valid() {
		return nil, fmt.Errorf("invalid semantic analysis mode %q", req.SemanticAnalysisMode)
	}
	now := time.Now().UTC()
	settings := m.settings.Get()
	execution, err := BuildExecutionSnapshot(settings, req.SemanticAnalysisMode, m.build, now)
	if err != nil {
		return nil, fmt.Errorf("capture execution snapshot: %w", err)
	}
	executionJSON, err := json.Marshal(execution)
	if err != nil {
		return nil, fmt.Errorf("encode execution snapshot: %w", err)
	}
	a := &domain.Analysis{
		ID: newID("ana"), ProjectID: req.ProjectID, Status: domain.AnalysisQueued, CreatedAt: now,
		Label: req.Label, Note: req.Note, SemanticAnalysisMode: req.SemanticAnalysisMode,
		ExecutionSnapshot: string(executionJSON), ExecutionFingerprint: execution.ExecutionFingerprint,
	}
	if err := m.analyses.Create(ctx, a); err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.pending[a.ID] = settings
	m.mu.Unlock()
	m.queue <- a.ID
	return a, nil
}

// errSettingsUnavailable means a queued run lost its enqueue-time settings,
// which only happens if it was not enqueued by this process.
var errSettingsUnavailable = errors.New("the settings this run was enqueued with are no longer available; enqueue it again")

func (m *JobManager) run(ctx context.Context, analysisID string) {
	a, err := m.analyses.Get(ctx, analysisID)
	if err != nil {
		return // analysis row is gone (e.g. project was deleted); nothing to run
	}

	m.mu.Lock()
	settings, ok := m.pending[analysisID]
	delete(m.pending, analysisID)
	m.mu.Unlock()
	if !ok {
		m.fail(ctx, a, errSettingsUnavailable)
		return
	}
	if err := m.recordSettingsDrift(a, settings); err != nil {
		m.fail(ctx, a, err)
		return
	}

	// Without a configured model the pipeline still runs its deterministic
	// dataset pre-analysis (Issue #16); it fails itself, with guidance, when
	// the project has nothing a rule can analyze.
	var client llm.Client
	if settings.Configured() {
		client = m.newLLMClient(settings)
	}

	pipeline := &Pipeline{
		Documents: m.pipeline.Documents, Observations: m.pipeline.Observations,
		Patterns: m.pipeline.Patterns, Insights: m.pipeline.Insights, Evidence: m.pipeline.Evidence,
		LLM: client, Model: settings.Model,
	}

	now := time.Now().UTC()
	docs, err := m.pipeline.Documents.ListByProject(ctx, a.ProjectID)
	if err != nil {
		m.fail(ctx, a, fmt.Errorf("list documents: %w", err))
		return
	}
	input := BuildInputSnapshot(docs, now)
	inputJSON, err := json.Marshal(input)
	if err != nil {
		m.fail(ctx, a, fmt.Errorf("encode input snapshot: %w", err))
		return
	}
	a.InputSnapshot, a.InputFingerprint = string(inputJSON), input.InputFingerprint
	a.Status = domain.AnalysisRunning
	a.StartedAt = &now
	_ = m.analyses.Update(ctx, a)
	m.broadcast(a.ID, SSEEvent{Event: "progress", Data: progressJSON("starting", 0, "Starting analysis...")})

	metrics, err := pipeline.RunDocuments(ctx, a.ID, a.ProjectID, docs, func(step string, progress int, message string) {
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

// recordSettingsDrift marks the snapshot when the live settings no longer
// match the ones the run was enqueued with. The run still executes with the
// enqueued settings, which the snapshot already describes.
func (m *JobManager) recordSettingsDrift(a *domain.Analysis, enqueued Settings) error {
	live := m.settings.Get()
	if live.BaseURL == enqueued.BaseURL && live.Model == enqueued.Model && live.Configured() == enqueued.Configured() {
		return nil
	}
	var execution ExecutionSnapshot
	if err := json.Unmarshal([]byte(a.ExecutionSnapshot), &execution); err != nil {
		return fmt.Errorf("decode execution snapshot: %w", err)
	}
	execution.SettingsChangedBeforeStart = true
	encoded, err := json.Marshal(execution)
	if err != nil {
		return fmt.Errorf("encode execution snapshot: %w", err)
	}
	a.ExecutionSnapshot = string(encoded)
	return nil
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
