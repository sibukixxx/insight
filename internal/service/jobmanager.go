package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"insight-lab/internal/execution"
	"insight-lab/internal/input"
	"strings"
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
	// AllowedModels are the models, besides the configured one, that callers
	// may bind to pipeline stages (operator config; empty = configured only).
	AllowedModels []string

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

	// planner and preparation implement ExecutionProfile (#91). Without
	// ConfigureExecution only LIGHT/STANDARD are available and raw artifact
	// preparation fails for lack of a resolver.
	planner     execution.Planner
	preparation *Preparation
}

// ConfigureExecution enables raw artifact preparation and, when prep.Heavy is
// set, the HEAVY profile.
func (m *JobManager) ConfigureExecution(prep *Preparation) {
	m.preparation = prep
	m.planner = execution.DefaultPlanner(prep.Capabilities())
}

// ExecutionCapabilities reports what this engine can execute.
func (m *JobManager) ExecutionCapabilities() execution.Capabilities {
	return m.planner.Capabilities
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
		planner:      execution.DefaultPlanner(execution.Capabilities{}),
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
	// ResearchQuestion is optional. Empty means open-ended discovery.
	ResearchQuestion string
	// ReasoningProfile selects semantic specialization; empty means
	// GENERAL_RESEARCH.
	ReasoningProfile domain.ReasoningProfile
	// ExecutionProfile is the requested strategy; empty means AUTO.
	ExecutionProfile execution.Profile
	// ModelBindings optionally binds pipeline stages to operator-allowed
	// models (ModelStages / AllowedModels).
	ModelBindings map[string]string
}

// Enqueue creates the analysis row (status "queued") with the execution
// snapshot of the current settings, and schedules it for a worker. The run
// later executes with exactly these settings, even if they change while it
// waits in the queue.
func (m *JobManager) Enqueue(ctx context.Context, req EnqueueRequest) (*domain.Analysis, error) {
	if req.SemanticAnalysisMode != "" && !req.SemanticAnalysisMode.Valid() {
		return nil, fmt.Errorf("invalid semantic analysis mode %q", req.SemanticAnalysisMode)
	}
	req.ResearchQuestion = strings.TrimSpace(req.ResearchQuestion)
	req.ReasoningProfile = req.ReasoningProfile.Normalize()
	if !req.ReasoningProfile.Valid() {
		return nil, fmt.Errorf("invalid reasoning profile %q", req.ReasoningProfile)
	}
	if len(req.ResearchQuestion) > 2000 {
		return nil, fmt.Errorf("research question is limited to 2000 characters")
	}
	now := time.Now().UTC()
	settings := m.settings.Get()
	bindings, err := ResolveModelBindings(settings, m.AllowedModels, req.ModelBindings)
	if err != nil {
		return nil, err
	}
	settings.StageModels = bindings
	resolution, err := m.resolveProfile(ctx, req)
	if err != nil {
		return nil, err
	}
	execution, err := BuildExecutionSnapshotForReasoningProfile(settings, req.SemanticAnalysisMode, req.ReasoningProfile, m.build, now)
	if err != nil {
		return nil, fmt.Errorf("capture execution snapshot: %w", err)
	}
	execution.ExecutionProfile = &resolution
	executionJSON, err := json.Marshal(execution)
	if err != nil {
		return nil, fmt.Errorf("encode execution snapshot: %w", err)
	}
	a := &domain.Analysis{
		ID: newID("ana"), ProjectID: req.ProjectID, Status: domain.AnalysisQueued, CreatedAt: now,
		Label: req.Label, Note: req.Note, SemanticAnalysisMode: req.SemanticAnalysisMode, ResearchQuestion: req.ResearchQuestion, ReasoningProfile: req.ReasoningProfile,
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
		client = newStageRouter(settings, m.newLLMClient)
	}

	pipeline := &Pipeline{
		Documents: m.pipeline.Documents, Observations: m.pipeline.Observations,
		Patterns: m.pipeline.Patterns, Insights: m.pipeline.Insights, Evidence: m.pipeline.Evidence,
		LLM: client, Model: settings.Model, ResearchQuestion: a.ResearchQuestion, ReasoningProfile: a.ReasoningProfile.Normalize(),
	}

	now := time.Now().UTC()
	docs, err := m.pipeline.Documents.ListByProject(ctx, a.ProjectID)
	if err != nil {
		m.fail(ctx, a, fmt.Errorf("list documents: %w", err))
		return
	}
	analyzed, docs, err := m.prepareInputs(ctx, a, docs)
	if err != nil {
		m.fail(ctx, a, fmt.Errorf("prepare inputs: %w", err))
		return
	}
	inputSnap := BuildInputSnapshotForQuestion(docs, pipeline.ResearchQuestion, now)
	inputJSON, err := json.Marshal(inputSnap)
	if err != nil {
		m.fail(ctx, a, fmt.Errorf("encode input snapshot: %w", err))
		return
	}
	a.InputSnapshot, a.InputFingerprint = string(inputJSON), inputSnap.InputFingerprint
	a.Status = domain.AnalysisRunning
	a.StartedAt = &now
	_ = m.analyses.Update(ctx, a)
	m.broadcast(a.ID, SSEEvent{Event: "progress", Data: progressJSON("starting", 0, "Starting analysis...")})

	metrics, err := pipeline.RunDocuments(ctx, a.ID, a.ProjectID, analyzed, func(step string, progress int, message string) {
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

// resolveProfile measures the project's input shape and resolves the
// requested profile. An unavailable profile fails the enqueue explicitly.
func (m *JobManager) resolveProfile(ctx context.Context, req EnqueueRequest) (execution.Resolution, error) {
	requested := req.ExecutionProfile
	if requested == "" {
		requested = execution.ProfileAuto
	}
	docs, err := m.pipeline.Documents.ListByProject(ctx, req.ProjectID)
	if err != nil {
		return execution.Resolution{}, fmt.Errorf("list documents: %w", err)
	}
	shape, err := input.MeasureShape(ctx, input.NewDocumentSource(docs))
	if err != nil {
		return execution.Resolution{}, err
	}
	return m.planner.Resolve(requested, shape)
}

// resolvedProfile reads the profile recorded at enqueue time.
func resolvedProfile(a *domain.Analysis) execution.Profile {
	var snap ExecutionSnapshot
	if json.Unmarshal([]byte(a.ExecutionSnapshot), &snap) != nil || snap.ExecutionProfile == nil {
		return execution.ProfileLight
	}
	return snap.ExecutionProfile.Resolved
}

// prepareInputs creates prepared Analytical Artifacts for referenced raw
// inputs, then returns the documents the pipeline analyzes (raw references
// excluded: their content is a descriptor, not evidence text) and the full
// set for the input snapshot.
func (m *JobManager) prepareInputs(ctx context.Context, a *domain.Analysis, docs []*domain.Document) (analyzed, all []*domain.Document, err error) {
	shape, err := input.MeasureShape(ctx, input.NewDocumentSource(docs))
	if err != nil {
		return nil, nil, err
	}
	if shape.RawToPrepare > 0 {
		created, err := m.preparation.Prepare(ctx, a.ProjectID, docs, resolvedProfile(a), time.Now().UTC())
		if err != nil {
			return nil, nil, err
		}
		if len(created) > 0 {
			if err := m.pipeline.Documents.CreateBatch(ctx, created); err != nil {
				return nil, nil, fmt.Errorf("save prepared artifacts: %w", err)
			}
			docs = append(append([]*domain.Document{}, docs...), created...)
		}
	}
	for _, d := range docs {
		if !input.IsRawReference(d) {
			analyzed = append(analyzed, d)
		}
	}
	return analyzed, docs, nil
}
