package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"insight-lab/internal/execution"
	"insight-lab/internal/input"
	"log/slog"
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
// goroutines - no external queue system; see docs/detailed-design.md §10)
// and fans progress out to any number of SSE subscribers per analysis. The
// analyses table is the queue of record (#133): workers claim queued rows
// with a conditional update, and cancellation, interruption and retry are
// recorded there, so a restart, a browser refresh or an SSE reconnect
// recovers the authoritative state via GET /api/analysis/{id}.
type JobManager struct {
	// AllowedModels are the models, besides the configured one, that callers
	// may bind to pipeline stages (operator config; empty = configured only).
	AllowedModels []string
	// MaxQueued bounds how many analyses may wait for a worker; Enqueue
	// fails with ErrAnalysisQueueFull beyond it. Zero means
	// DefaultMaxQueuedAnalyses.
	MaxQueued int

	analyses     repository.AnalysisRepository
	pipeline     *Pipeline
	settings     *SettingsStore
	newLLMClient func(Settings) llm.Client
	build        buildinfo.Info

	mu          sync.Mutex
	subscribers map[string]map[chan SSEEvent]struct{}
	// pending holds, in memory only, the settings each queued run was
	// enqueued with. The API key never leaves this map; a run queued before
	// a restart resumes only when the current settings reproduce its
	// execution fingerprint.
	pending map[string]Settings
	// cancels stops the context of each running analysis.
	cancels map[string]context.CancelCauseFunc

	wake chan struct{}
	wg   sync.WaitGroup

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

// RuntimeMode reports where HEAVY partitions run.
func (m *JobManager) RuntimeMode() execution.RuntimeMode {
	return m.preparation.RuntimeMode()
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
		cancels:      map[string]context.CancelCauseFunc{},
		wake:         make(chan struct{}, 1),
		planner:      execution.DefaultPlanner(execution.Capabilities{}),
	}
}

func DefaultLLMClientFactory(s Settings) llm.Client {
	return llm.NewOpenAIClient(s.BaseURL, s.APIKey, s.Model)
}

// DefaultMaxQueuedAnalyses is the admission bound when MaxQueued is zero.
const DefaultMaxQueuedAnalyses = 256

var (
	// ErrAnalysisQueueFull rejects an enqueue beyond MaxQueued.
	ErrAnalysisQueueFull = errors.New("the analysis queue is full; try again later")
	// ErrAnalysisFinished means the analysis already reached a terminal
	// status and cannot be cancelled.
	ErrAnalysisFinished = errors.New("the analysis has already finished")
	// ErrAnalysisNotRetryable means only a failed analysis can be retried.
	ErrAnalysisNotRetryable = errors.New("only a failed analysis can be retried")

	errCancelRequested = errors.New("cancelled by request")
	errNeedsRequeue    = errors.New("the run was queued before the engine restarted and its execution configuration (engine build, prompts, models or provider) no longer matches; retry it as a new run")
)

// RecoverInterrupted marks any analysis left running by a process that
// exited mid-run as failed (FailureInterrupted). Queued analyses stay
// queued and are resumed by Start when their execution configuration is
// still reproducible. Call once at startup, before Start.
func (m *JobManager) RecoverInterrupted(ctx context.Context) (int, error) {
	return m.analyses.FailInterrupted(ctx)
}

func (m *JobManager) Start(ctx context.Context, workers int) {
	for i := 0; i < workers; i++ {
		m.wg.Add(1)
		go m.worker(ctx)
	}
	m.signal()
}

func (m *JobManager) Wait() { m.wg.Wait() }

func (m *JobManager) signal() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func (m *JobManager) worker(ctx context.Context) {
	defer m.wg.Done()
	for {
		for ctx.Err() == nil {
			a, err := m.analyses.ClaimNextQueued(ctx)
			if errors.Is(err, repository.ErrNotFound) {
				break
			}
			if err != nil {
				if ctx.Err() == nil {
					// Queued rows stay queued; look again shortly instead of
					// stranding them until the next enqueue.
					slog.Error("claim queued analysis", "error", err)
					time.AfterFunc(time.Second, m.signal)
				}
				break
			}
			// Another idle worker may take the next queued run meanwhile.
			m.signal()
			m.run(ctx, a)
		}
		select {
		case <-ctx.Done():
			return
		case <-m.wake:
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
	// OutputLocale is the explicitly requested language of generated text;
	// empty means not requested (text follows the source language).
	OutputLocale domain.OutputLocale
	// ExecutionProfile is the requested strategy; empty means AUTO.
	ExecutionProfile execution.Profile
	// ModelBindings optionally binds pipeline stages to operator-allowed
	// models (ModelStages / AllowedModels).
	ModelBindings map[string]string
	// RetryOf links the run to the failed analysis it retries.
	RetryOf string
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
	if !req.OutputLocale.Valid() {
		return nil, fmt.Errorf("unsupported output locale %q", req.OutputLocale)
	}
	if len(req.ResearchQuestion) > 2000 {
		return nil, fmt.Errorf("research question is limited to 2000 characters")
	}
	maxQueued := m.MaxQueued
	if maxQueued <= 0 {
		maxQueued = DefaultMaxQueuedAnalyses
	}
	if queued, err := m.analyses.CountQueued(ctx); err != nil {
		return nil, err
	} else if queued >= maxQueued {
		return nil, ErrAnalysisQueueFull
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
	var runtimeMode execution.RuntimeMode
	if resolution.Resolved == execution.ProfileHeavy {
		runtimeMode = m.RuntimeMode()
	}
	execution, err := BuildExecutionSnapshotForRun(settings, req.SemanticAnalysisMode, req.ReasoningProfile, req.OutputLocale, m.build, now)
	if err != nil {
		return nil, fmt.Errorf("capture execution snapshot: %w", err)
	}
	execution.ExecutionProfile = &resolution
	execution.RuntimeMode = runtimeMode
	executionJSON, err := json.Marshal(execution)
	if err != nil {
		return nil, fmt.Errorf("encode execution snapshot: %w", err)
	}
	a := &domain.Analysis{
		ID: newID("ana"), ProjectID: req.ProjectID, Status: domain.AnalysisQueued, CreatedAt: now,
		Label: req.Label, Note: req.Note, SemanticAnalysisMode: req.SemanticAnalysisMode, ResearchQuestion: req.ResearchQuestion, ReasoningProfile: req.ReasoningProfile, OutputLocale: req.OutputLocale,
		ExecutionSnapshot: string(executionJSON), ExecutionFingerprint: execution.ExecutionFingerprint,
		RetryOf: req.RetryOf,
	}
	// The settings are registered before the row exists so a worker that
	// claims the row at once always finds them.
	m.mu.Lock()
	m.pending[a.ID] = settings
	m.mu.Unlock()
	if err := m.analyses.Create(ctx, a); err != nil {
		m.mu.Lock()
		delete(m.pending, a.ID)
		m.mu.Unlock()
		return nil, err
	}
	m.signal()
	return a, nil
}

// run executes a claimed (running) analysis to exactly one terminal state.
// When ctx ends first (shutdown) nothing terminal is written: the row stays
// running and the next start marks it interrupted.
func (m *JobManager) run(ctx context.Context, a *domain.Analysis) {
	runCtx, cancel := context.WithCancelCause(ctx)
	m.mu.Lock()
	m.cancels[a.ID] = cancel
	settings, ok := m.pending[a.ID]
	delete(m.pending, a.ID)
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		delete(m.cancels, a.ID)
		m.mu.Unlock()
		cancel(nil)
	}()
	// A cancel requested between the claim and the registration above
	// found no context to stop.
	if fresh, err := m.analyses.Get(ctx, a.ID); err == nil && fresh.CancelRequestedAt != nil {
		cancel(errCancelRequested)
	}

	var metrics *Metrics
	var err error
	if !ok {
		settings, err = m.resumeSettings(a)
	}
	if err == nil {
		metrics, err = m.execute(runCtx, a, settings)
	}
	if ctx.Err() != nil {
		return
	}
	m.finish(context.WithoutCancel(ctx), runCtx, a, metrics, err)
}

// execute runs the pipeline for a with settings and returns its metrics.
func (m *JobManager) execute(ctx context.Context, a *domain.Analysis, settings Settings) (*Metrics, error) {
	if err := m.recordSettingsDrift(a, settings); err != nil {
		return nil, err
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
		OutputLocale: a.OutputLocale,
	}

	now := time.Now().UTC()
	docs, err := m.pipeline.Documents.ListByProject(ctx, a.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	analyzed, docs, err := m.prepareInputs(ctx, a, docs)
	if err != nil {
		return nil, fmt.Errorf("prepare inputs: %w", err)
	}
	inputSnap := BuildInputSnapshotForQuestion(docs, pipeline.ResearchQuestion, now)
	inputJSON, err := json.Marshal(inputSnap)
	if err != nil {
		return nil, fmt.Errorf("encode input snapshot: %w", err)
	}
	a.InputSnapshot, a.InputFingerprint = string(inputJSON), inputSnap.InputFingerprint
	a.Status = domain.AnalysisRunning
	a.StartedAt = &now
	_ = m.analyses.Update(ctx, a)
	m.broadcast(a.ID, SSEEvent{Event: "progress", Data: progressJSON("starting", 0, "Starting analysis...")})

	return pipeline.RunDocuments(ctx, a.ID, a.ProjectID, analyzed, func(step string, progress int, message string) {
		a.CurrentStep = step
		a.Progress = progress
		_ = m.analyses.Update(ctx, a)
		m.broadcast(a.ID, SSEEvent{Event: "progress", Data: progressJSON(step, progress, message)})
	})
}

// finish writes the one terminal state of a run. Completion loses to a
// cancel request recorded before it, so a cancelled run never turns into a
// success and a success is never reported twice.
func (m *JobManager) finish(ctx, runCtx context.Context, a *domain.Analysis, metrics *Metrics, runErr error) {
	finished := time.Now().UTC()
	a.FinishedAt = &finished
	if runErr == nil {
		metricsJSON, _ := json.Marshal(metrics)
		a.Status, a.Progress, a.CurrentStep, a.Metrics = domain.AnalysisCompleted, 100, "completed", string(metricsJSON)
		if ok, err := m.analyses.FinishRunning(ctx, a); err == nil && ok {
			m.broadcast(a.ID, SSEEvent{Event: "completed", Data: fmt.Sprintf(`{"progress":100,"insightCount":%d}`, metrics.FinalInsightCount)})
			return
		}
		a.Metrics = ""
		runErr = errCancelRequested
	}
	switch {
	case errors.Is(context.Cause(runCtx), errCancelRequested) || errors.Is(runErr, errCancelRequested):
		a.FailureCode, a.Error = domain.FailureCancelled, errCancelRequested.Error()
	case errors.Is(runErr, errNeedsRequeue):
		a.FailureCode, a.Error = domain.FailureNeedsRequeue, runErr.Error()
	default:
		a.FailureCode, a.Error = domain.FailureError, runErr.Error()
	}
	a.Status = domain.AnalysisFailed
	if ok, err := m.analyses.FinishRunning(ctx, a); err != nil || !ok {
		return
	}
	m.broadcast(a.ID, terminalEvent(a))
}

// terminalEvent is the SSE event describing a finished analysis.
func terminalEvent(a *domain.Analysis) SSEEvent {
	if a.Status == domain.AnalysisCompleted {
		return SSEEvent{Event: "completed", Data: `{"progress":100}`}
	}
	msg, _ := json.Marshal(map[string]string{"step": a.CurrentStep, "message": a.Error, "code": string(a.FailureCode)})
	return SSEEvent{Event: "error", Data: string(msg)}
}

// TerminalEvent reports the SSE event for a finished analysis, so a
// subscriber that connects after the end still receives it.
func TerminalEvent(a *domain.Analysis) (SSEEvent, bool) {
	if !a.Finished() {
		return SSEEvent{}, false
	}
	return terminalEvent(a), true
}

// resumeSettings returns the settings for a run queued before a restart.
// The API key was never persisted, so the run resumes with the current
// settings only when they reproduce its recorded execution fingerprint
// (same engine build, prompts, provider and models); otherwise it needs to
// be retried as a new run.
func (m *JobManager) resumeSettings(a *domain.Analysis) (Settings, error) {
	var snap ExecutionSnapshot
	if err := json.Unmarshal([]byte(a.ExecutionSnapshot), &snap); err != nil || a.ExecutionFingerprint == "" {
		return Settings{}, errNeedsRequeue
	}
	current := m.settings.Get()
	if snap.LLM != nil && !(len(snap.LLM.Models) == 1 && snap.LLM.Models[0].Stage == "all") {
		current.StageModels = map[string]string{}
		for _, b := range snap.LLM.Models {
			current.StageModels[b.Stage] = b.Model
		}
	}
	rebuilt, err := BuildExecutionSnapshotForRun(current, a.SemanticAnalysisMode, a.ReasoningProfile, a.OutputLocale, m.build, time.Now().UTC())
	if err != nil || rebuilt.ExecutionFingerprint != a.ExecutionFingerprint {
		return Settings{}, errNeedsRequeue
	}
	return current, nil
}

// Cancel stops an analysis. A queued run is cancelled at once; a running
// one records the request and its context is cancelled, reaching model
// calls and HEAVY partitions; its terminal state follows shortly after.
// Cancelling a finished analysis fails with ErrAnalysisFinished.
func (m *JobManager) Cancel(ctx context.Context, analysisID string) (*domain.Analysis, error) {
	a, err := m.analyses.Get(ctx, analysisID)
	if err != nil {
		return nil, err
	}
	switch a.Status {
	case domain.AnalysisQueued:
		ok, err := m.analyses.CancelQueued(ctx, a.ID, errCancelRequested.Error())
		if err != nil {
			return nil, err
		}
		if !ok {
			return m.Cancel(ctx, analysisID) // claimed or finished meanwhile
		}
		m.mu.Lock()
		delete(m.pending, a.ID)
		m.mu.Unlock()
		a, err = m.analyses.Get(ctx, a.ID)
		if err != nil {
			return nil, err
		}
		m.broadcast(a.ID, terminalEvent(a))
		return a, nil
	case domain.AnalysisRunning:
		if _, err := m.analyses.RequestCancel(ctx, a.ID); err != nil {
			return nil, err
		}
		m.mu.Lock()
		cancel := m.cancels[a.ID]
		m.mu.Unlock()
		if cancel != nil {
			cancel(errCancelRequested)
		}
		return m.analyses.Get(ctx, a.ID)
	default:
		return nil, ErrAnalysisFinished
	}
}

// Retry enqueues a failed analysis again as a new run with the same
// request (label, note, semantic mode, question, reasoning profile, output
// locale, requested execution profile and model bindings) under the current
// settings. It is idempotent: a failed analysis has at most one retry, and
// asking again returns it with created false.
func (m *JobManager) Retry(ctx context.Context, analysisID string) (retry *domain.Analysis, created bool, err error) {
	orig, err := m.analyses.Get(ctx, analysisID)
	if err != nil {
		return nil, false, err
	}
	if orig.Status != domain.AnalysisFailed {
		return nil, false, ErrAnalysisNotRetryable
	}
	if existing, err := m.analyses.FindRetry(ctx, orig.ID); err == nil {
		return existing, false, nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, false, err
	}
	req := EnqueueRequest{
		ProjectID: orig.ProjectID, Label: orig.Label, Note: orig.Note, SemanticAnalysisMode: orig.SemanticAnalysisMode,
		ResearchQuestion: orig.ResearchQuestion, ReasoningProfile: orig.ReasoningProfile, OutputLocale: orig.OutputLocale,
		RetryOf: orig.ID,
	}
	var snap ExecutionSnapshot
	if json.Unmarshal([]byte(orig.ExecutionSnapshot), &snap) == nil {
		if snap.ExecutionProfile != nil {
			req.ExecutionProfile = snap.ExecutionProfile.Requested
		}
		if snap.LLM != nil && !(len(snap.LLM.Models) == 1 && snap.LLM.Models[0].Stage == "all") {
			req.ModelBindings = map[string]string{}
			for _, b := range snap.LLM.Models {
				req.ModelBindings[b.Stage] = b.Model
			}
		}
	}
	retry, err = m.Enqueue(ctx, req)
	if errors.Is(err, repository.ErrConflict) {
		existing, findErr := m.analyses.FindRetry(ctx, orig.ID)
		if findErr != nil {
			return nil, false, findErr
		}
		return existing, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return retry, true, nil
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
