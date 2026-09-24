package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

// Heavy Execution Adapter (Issue #93). A Runtime executes a job as independent
// partitions with persistent identity: re-running a job ID after a restart
// resumes it and skips partitions that already succeeded. Research semantics
// never live here; partition output converges to the Analytical Artifact
// boundary through the caller's merge.

type JobStatus string

const (
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

// ErrCancelled is returned when a job was cancelled.
var ErrCancelled = errors.New("heavy job cancelled")

type PartitionState struct {
	ID       string          `json:"id"`
	Status   JobStatus       `json:"status"`
	Attempts int             `json:"attempts"`
	Result   json.RawMessage `json:"result,omitempty"`
	Error    string          `json:"error,omitempty"`
}

// JobState is machine-readable progress and provenance.
type JobState struct {
	ID         string           `json:"id"`
	Status     JobStatus        `json:"status"`
	Partitions []PartitionState `json:"partitions"`
	Done       int              `json:"done"`
	Total      int              `json:"total"`
	UpdatedAt  time.Time        `json:"updatedAt"`
}

// JobSpec names a job and its partitions. IDs must be stable so that retries
// are idempotent.
type JobSpec struct {
	ID         string
	Partitions []string
}

// PartitionFunc processes one partition and returns its serialized result.
type PartitionFunc func(ctx context.Context, partitionID string) (json.RawMessage, error)

// Runtime is the provider-neutral port. Implementations may be local
// processes, queues or remote workers.
type Runtime interface {
	// Run executes (or resumes) the job and blocks until it ends. On success
	// every partition has a result.
	Run(ctx context.Context, spec JobSpec, fn PartitionFunc) (JobState, error)
	Status(ctx context.Context, jobID string) (JobState, error)
	Cancel(ctx context.Context, jobID string) error
}

// LocalRuntime is the in-process reference implementation. With Dir set,
// job state is persisted as JSON so identity and finished partitions survive
// a process restart.
type LocalRuntime struct {
	Dir         string
	Concurrency int
	MaxAttempts int

	mu      sync.Mutex
	jobs    map[string]*JobState
	cancels map[string]context.CancelFunc
}

var jobIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func NewLocalRuntime(dir string, concurrency int) *LocalRuntime {
	return &LocalRuntime{Dir: dir, Concurrency: concurrency, MaxAttempts: 2, jobs: map[string]*JobState{}, cancels: map[string]context.CancelFunc{}}
}

func (r *LocalRuntime) load(id string) (*JobState, bool) {
	if s, ok := r.jobs[id]; ok {
		return s, true
	}
	if r.Dir == "" {
		return nil, false
	}
	b, err := os.ReadFile(filepath.Join(r.Dir, id+".json"))
	if err != nil {
		return nil, false
	}
	var s JobState
	if json.Unmarshal(b, &s) != nil {
		return nil, false
	}
	r.jobs[id] = &s
	return &s, true
}

// persist must be called with r.mu held.
func (r *LocalRuntime) persist(s *JobState) error {
	s.Done, s.UpdatedAt = 0, time.Now().UTC()
	for _, p := range s.Partitions {
		if p.Status == JobSucceeded {
			s.Done++
		}
	}
	if r.Dir == "" {
		return nil
	}
	if err := os.MkdirAll(r.Dir, 0o750); err != nil {
		return err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := filepath.Join(r.Dir, s.ID+".json.tmp")
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(r.Dir, s.ID+".json"))
}

func (r *LocalRuntime) snapshot(s *JobState) JobState {
	out := *s
	out.Partitions = append([]PartitionState(nil), s.Partitions...)
	return out
}

func (r *LocalRuntime) Status(_ context.Context, jobID string) (JobState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.load(jobID)
	if !ok {
		return JobState{}, fmt.Errorf("heavy job %q not found", jobID)
	}
	return r.snapshot(s), nil
}

func (r *LocalRuntime) Cancel(_ context.Context, jobID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.load(jobID)
	if !ok {
		return fmt.Errorf("heavy job %q not found", jobID)
	}
	if cancel := r.cancels[jobID]; cancel != nil {
		cancel()
	}
	if s.Status == JobRunning {
		s.Status = JobCancelled
	}
	return r.persist(s)
}

func (r *LocalRuntime) Run(parent context.Context, spec JobSpec, fn PartitionFunc) (JobState, error) {
	if !jobIDPattern.MatchString(spec.ID) || len(spec.Partitions) == 0 {
		return JobState{}, fmt.Errorf("heavy job needs a safe id and at least one partition")
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	r.mu.Lock()
	if _, running := r.cancels[spec.ID]; running {
		r.mu.Unlock()
		return JobState{}, fmt.Errorf("heavy job %q is already running", spec.ID)
	}
	s, ok := r.load(spec.ID)
	if !ok || s.Status == JobFailed || s.Status == JobCancelled {
		s = r.resume(s, spec)
		r.jobs[spec.ID] = s
	}
	if s.Status == JobSucceeded {
		out := r.snapshot(s)
		r.mu.Unlock()
		return out, nil
	}
	r.cancels[spec.ID] = cancel
	if err := r.persist(s); err != nil {
		r.mu.Unlock()
		return JobState{}, err
	}
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.cancels, spec.ID)
		r.mu.Unlock()
	}()
	r.runPartitions(ctx, s, fn)
	return r.finish(ctx, s)
}
