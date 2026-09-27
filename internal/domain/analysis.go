package domain

import (
	"insight-lab/internal/analytical/model"
	"time"
)

type Observation struct {
	Temporal *model.TemporalEvidence `json:"temporal,omitempty"`
	ID       string
	// AnalysisID is the run that produced the observation. It is empty for
	// observations recorded before runs owned them, when the run could not
	// be proven from pattern or evidence links.
	AnalysisID  string
	DocumentID  string
	Quote       string
	StartOffset int
	EndOffset   int
	Behavior    string
	Topic       string
	CreatedAt   time.Time
}

type AnalysisStatus string

const (
	AnalysisQueued    AnalysisStatus = "queued"
	AnalysisRunning   AnalysisStatus = "running"
	AnalysisCompleted AnalysisStatus = "completed"
	AnalysisFailed    AnalysisStatus = "failed"
)

// AnalysisFailureCode says why a failed analysis failed (#133). The legacy
// status stays "failed" for every code, so readers that only know the four
// statuses keep working; an empty code on a legacy row means not recorded.
type AnalysisFailureCode string

const (
	// FailureError is a failure of the run itself (input, model, pipeline).
	FailureError AnalysisFailureCode = "ERROR"
	// FailureCancelled means a caller cancelled the run.
	FailureCancelled AnalysisFailureCode = "CANCELLED"
	// FailureInterrupted means the process stopped while the run was
	// running. Model-backed stages may already have been called, so the run
	// is never resumed in place; retry it as a new run.
	FailureInterrupted AnalysisFailureCode = "INTERRUPTED"
	// FailureNeedsRequeue means a queued run could not be resumed after a
	// restart because its execution configuration (engine build, prompts,
	// models or provider) is no longer what was captured at enqueue.
	FailureNeedsRequeue AnalysisFailureCode = "NEEDS_REQUEUE"
)

// AnalysisLifecycle is the coordinator state derived from the stored status
// and failure code.
type AnalysisLifecycle string

const (
	LifecycleQueued          AnalysisLifecycle = "QUEUED"
	LifecycleRunning         AnalysisLifecycle = "RUNNING"
	LifecycleCancelRequested AnalysisLifecycle = "CANCEL_REQUESTED"
	LifecycleSucceeded       AnalysisLifecycle = "SUCCEEDED"
	LifecycleFailed          AnalysisLifecycle = "FAILED"
	LifecycleCancelled       AnalysisLifecycle = "CANCELLED"
	LifecycleInterrupted     AnalysisLifecycle = "INTERRUPTED"
)

// Lifecycle maps the stored state to the coordinator state machine.
func (a *Analysis) Lifecycle() AnalysisLifecycle {
	switch a.Status {
	case AnalysisQueued:
		return LifecycleQueued
	case AnalysisRunning:
		if a.CancelRequestedAt != nil {
			return LifecycleCancelRequested
		}
		return LifecycleRunning
	case AnalysisCompleted:
		return LifecycleSucceeded
	}
	switch a.FailureCode {
	case FailureCancelled:
		return LifecycleCancelled
	case FailureInterrupted, FailureNeedsRequeue:
		return LifecycleInterrupted
	}
	return LifecycleFailed
}

// Finished reports whether the analysis reached a terminal status.
func (a *Analysis) Finished() bool {
	return a.Status == AnalysisCompleted || a.Status == AnalysisFailed
}

type Analysis struct {
	ID          string
	ProjectID   string
	Status      AnalysisStatus
	CurrentStep string
	Progress    int
	Error       string
	Metrics     string
	// Label and Note are optional human descriptions of why the run exists.
	Label string
	Note  string
	// SemanticAnalysisMode is how the requester asked the input to be read,
	// or empty when the request did not say.
	SemanticAnalysisMode AnalysisMode
	// ResearchQuestion is optional semantic input. Empty means open-ended
	// discovery. It is persisted independently from execution configuration.
	ResearchQuestion string
	// ReasoningProfile selects the semantic specialization of the shared
	// research pipeline. Empty legacy rows normalize to GENERAL_RESEARCH.
	ReasoningProfile ReasoningProfile
	// OutputLocale is the explicitly requested language of model-generated
	// text. Empty means not requested (legacy rows and omitted requests).
	OutputLocale OutputLocale
	// ExecutionSnapshot and InputSnapshot are the JSON snapshots captured at
	// enqueue time and at run start. Both are empty for runs recorded before
	// snapshots existed; that means "not recorded", never "same".
	ExecutionSnapshot    string
	InputSnapshot        string
	ExecutionFingerprint string
	InputFingerprint     string
	StartedAt            *time.Time
	FinishedAt           *time.Time
	CreatedAt            time.Time

	// FailureCode is set when Status is failed by a lifecycle-aware engine.
	FailureCode AnalysisFailureCode
	// CancelRequestedAt is when a caller asked to cancel the running run.
	CancelRequestedAt *time.Time
	// RetryOf is the failed analysis this run retries, if any.
	RetryOf string
}
