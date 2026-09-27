package execution

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"insight-lab/internal/input"
)

// Remote placement (#135). The coordinator keeps job and partition state
// (the Heavy Runtime) and final merging; remote workers pull declarative
// WorkSpecs through a broker and answer with verified WorkResults. The
// broker carries only WorkSpecs (references, byte ranges and hashes) and
// results — never raw bytes, credentials or code. Inputs are read by each
// worker from its own mount of the same immutable input root, and every
// shard is verified against its content hash.
//
// Delivery is at least once: a task is acknowledged only after its result
// has been published, so a worker crash redelivers it. Every dispatch
// carries a fresh fencing token; the coordinator accepts only a result
// with the token it is waiting for, so late or duplicate results of an
// earlier attempt are ignored. Results are deterministic, so running a
// task twice is harmless. There is no exactly-once claim.

// Task is one delivery of a WorkSpec.
type Task struct {
	Token string   `json:"token"`
	Work  WorkSpec `json:"work"`
}

// TaskResult answers one Task.
type TaskResult struct {
	Token  string     `json:"token"`
	Worker string     `json:"worker,omitempty"`
	Result WorkResult `json:"result"`
}

// Broker is the provider-neutral transport port. Adapters (for example
// NATS JetStream, internal/execution/jetstream) live behind build tags so
// the default build depends on none of them.
type Broker interface {
	// PublishTask enqueues t durably for any worker.
	PublishTask(ctx context.Context, t Task) error
	// NextTask blocks until a task is delivered to this worker. ack must be
	// called only after the result is published; a task that is not
	// acknowledged in time is delivered again.
	NextTask(ctx context.Context) (t Task, ack func() error, err error)
	// PublishResult publishes a worker's answer.
	PublishResult(ctx context.Context, r TaskResult) error
	// SubscribeResults delivers results for jobID published from now on.
	SubscribeResults(ctx context.Context, jobID string) (<-chan TaskResult, func(), error)
	Close() error
}

// BrokerOpener opens a broker from its URL.
type BrokerOpener func(ctx context.Context, rawURL string) (Broker, error)

var (
	brokerMu      sync.Mutex
	brokerOpeners = map[string]BrokerOpener{}
)

// RegisterBroker makes a broker adapter available for URLs with scheme.
// Adapters call it from init in packages compiled only with their build tag.
func RegisterBroker(scheme string, open BrokerOpener) {
	brokerMu.Lock()
	defer brokerMu.Unlock()
	brokerOpeners[strings.ToLower(scheme)] = open
}

// BrokerSchemes lists the adapters compiled into this binary.
func BrokerSchemes() []string {
	brokerMu.Lock()
	defer brokerMu.Unlock()
	out := make([]string, 0, len(brokerOpeners))
	for s := range brokerOpeners {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// ErrNoBrokerAdapter means this binary was built without an adapter for the
// broker URL. The distributed runtime then fails; it never runs locally.
var ErrNoBrokerAdapter = errors.New("no broker adapter for this URL in this build")

// OpenBroker opens rawURL with the adapter registered for its scheme.
func OpenBroker(ctx context.Context, rawURL string) (Broker, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" {
		return nil, fmt.Errorf("invalid broker URL %q", rawURL)
	}
	brokerMu.Lock()
	open := brokerOpeners[strings.ToLower(u.Scheme)]
	brokerMu.Unlock()
	if open == nil {
		return nil, fmt.Errorf("%w (%s; compiled adapters: %v; build with -tags jetstream for nats://)", ErrNoBrokerAdapter, u.Scheme, BrokerSchemes())
	}
	return open(ctx, rawURL)
}

// RemoteDispatcher sends each WorkSpec through a broker and waits for the
// answer carrying its fencing token.
type RemoteDispatcher struct {
	Broker Broker
	// Timeout bounds one attempt; the Heavy Runtime retries with a new
	// token. Zero means 30 minutes.
	Timeout time.Duration
}

func (d RemoteDispatcher) Mode() RuntimeMode { return RuntimeDistributed }

func (d RemoteDispatcher) Dispatch(ctx context.Context, w WorkSpec) (WorkResult, error) {
	if err := w.Validate(); err != nil {
		return WorkResult{}, err
	}
	timeout := d.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	results, stop, err := d.Broker.SubscribeResults(ctx, w.JobID)
	if err != nil {
		return WorkResult{}, fmt.Errorf("partition %s: subscribe results: %w", w.PartitionID, err)
	}
	defer stop()
	token := newToken()
	if err := d.Broker.PublishTask(ctx, Task{Token: token, Work: w}); err != nil {
		return WorkResult{}, fmt.Errorf("partition %s: publish task: %w", w.PartitionID, err)
	}
	for {
		select {
		case r, ok := <-results:
			if !ok {
				return WorkResult{}, fmt.Errorf("partition %s: the result subscription closed", w.PartitionID)
			}
			if r.Token == token { // anything else is another attempt or partition
				return r.Result, nil
			}
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return WorkResult{}, fmt.Errorf("partition %s: no remote result within %s", w.PartitionID, timeout)
			}
			return WorkResult{}, ctx.Err()
		}
	}
}

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// RunRemoteWorker pulls tasks until ctx ends: it executes each with rs,
// publishes the result and only then acknowledges the task. A rejected
// WorkSpec is answered with an error result, never executed.
func RunRemoteWorker(ctx context.Context, b Broker, rs input.Resolver, name string) error {
	for {
		t, ack, err := b.NextTask(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		res, execErr := Execute(ctx, rs, t.Work)
		if execErr != nil {
			w := t.Work
			res = WorkResult{Version: w.Version, JobID: w.JobID, PartitionID: w.PartitionID,
				Operation: w.Operation, OperationVersion: w.OperationVersion, Error: execErr.Error()}
		}
		if err := b.PublishResult(ctx, TaskResult{Token: t.Token, Worker: name, Result: res}); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err // unacknowledged: the task is redelivered
		}
		if err := ack(); err != nil && ctx.Err() == nil {
			return err
		}
	}
}

// MemoryBroker is the in-process reference Broker used by conformance
// tests: durable within the process, redelivers unacknowledged tasks after
// AckWait, and fans results out to every subscriber of the job.
type MemoryBroker struct {
	AckWait time.Duration

	mu       sync.Mutex
	queue    []Task
	inflight map[string]inflightTask // by delivery id
	subs     map[string]map[chan TaskResult]struct{}
	notify   chan struct{}
	seq      int
	closed   bool
}

type inflightTask struct {
	task     Task
	deadline time.Time
}

func NewMemoryBroker(ackWait time.Duration) *MemoryBroker {
	return &MemoryBroker{AckWait: ackWait, inflight: map[string]inflightTask{}, subs: map[string]map[chan TaskResult]struct{}{}, notify: make(chan struct{}, 1)}
}

func (m *MemoryBroker) wake() {
	select {
	case m.notify <- struct{}{}:
	default:
	}
}

func (m *MemoryBroker) PublishTask(_ context.Context, t Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("broker closed")
	}
	m.queue = append(m.queue, t)
	m.wake()
	return nil
}

func (m *MemoryBroker) NextTask(ctx context.Context) (Task, func() error, error) {
	for {
		m.mu.Lock()
		now := time.Now()
		for id, f := range m.inflight { // redeliver what was not acknowledged
			if now.After(f.deadline) {
				m.queue = append(m.queue, f.task)
				delete(m.inflight, id)
			}
		}
		if len(m.queue) > 0 {
			t := m.queue[0]
			m.queue = m.queue[1:]
			m.seq++
			id := fmt.Sprint(m.seq)
			m.inflight[id] = inflightTask{task: t, deadline: now.Add(m.AckWait)}
			m.mu.Unlock()
			ack := func() error {
				m.mu.Lock()
				defer m.mu.Unlock()
				delete(m.inflight, id)
				return nil
			}
			return t, ack, nil
		}
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return Task{}, nil, ctx.Err()
		case <-m.notify:
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (m *MemoryBroker) PublishResult(_ context.Context, r TaskResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for ch := range m.subs[r.Result.JobID] {
		select {
		case ch <- r:
		default: // a slow subscriber drops it; the coordinator retries
		}
	}
	return nil
}

func (m *MemoryBroker) SubscribeResults(_ context.Context, jobID string) (<-chan TaskResult, func(), error) {
	ch := make(chan TaskResult, 64)
	m.mu.Lock()
	if m.subs[jobID] == nil {
		m.subs[jobID] = map[chan TaskResult]struct{}{}
	}
	m.subs[jobID][ch] = struct{}{}
	m.mu.Unlock()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			m.mu.Lock()
			delete(m.subs[jobID], ch)
			m.mu.Unlock()
		})
	}
	return ch, stop, nil
}

func (m *MemoryBroker) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// Pending reports queued plus unacknowledged tasks (for tests).
func (m *MemoryBroker) Pending() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.queue) + len(m.inflight)
}
