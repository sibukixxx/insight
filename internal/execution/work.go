package execution

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"insight-lab/internal/input"
)

// Portable work (#134). A WorkSpec is a versioned, serializable description
// of one partition's deterministic computation: an allowlisted operation, an
// immutable input byte range with its hash, and the declarative spec. It
// carries no closures, commands, SQL, credentials or host paths, so the
// same spec can run in-process, in a child process of this binary, or on a
// remote worker. The coordinator merges results; workers only compute.

// WorkSpecVersion is the wire version of WorkSpec and WorkResult.
const WorkSpecVersion = "insight-lab.work/v1"

// OperationCSVAggregateShard aggregates one CSV shard with
// input.AggregateCSV; its result is the JSON of an input.Aggregate.
const (
	OperationCSVAggregateShard        = "csv-aggregate-shard"
	OperationCSVAggregateShardVersion = "1"
)

const (
	// MaxWorkSpecBytes bounds a serialized WorkSpec.
	MaxWorkSpecBytes = 1 << 20
	// DefaultMaxResultBytes bounds a serialized result.
	DefaultMaxResultBytes = 64 << 20
)

// ErrWorkRejected means a WorkSpec or WorkResult failed validation: an
// unknown operation or version, a malformed range, or a result that does
// not answer the spec it was dispatched for.
var ErrWorkRejected = errors.New("work rejected")

// WorkInput is an immutable byte range of a raw artifact. The worker reads
// [0, HeaderLength) followed by [Offset, Offset+Length) and verifies that
// those bytes hash to ContentSHA256 before its result is used.
type WorkInput struct {
	URI           string `json:"uri"`
	FileSHA256    string `json:"fileSha256"`
	HeaderLength  int64  `json:"headerLength"`
	Offset        int64  `json:"offset"`
	Length        int64  `json:"length"`
	ContentSHA256 string `json:"contentSha256"`
}

type WorkLimits struct {
	MaxGroups      int   `json:"maxGroups,omitempty"`
	MaxResultBytes int64 `json:"maxResultBytes,omitempty"`
}

type WorkSpec struct {
	Version          string          `json:"version"`
	JobID            string          `json:"jobId"`
	PartitionID      string          `json:"partitionId"`
	Operation        string          `json:"operation"`
	OperationVersion string          `json:"operationVersion"`
	Input            WorkInput       `json:"input"`
	Spec             json.RawMessage `json:"spec"`
	SpecSHA256       string          `json:"specSha256"`
	Limits           WorkLimits      `json:"limits"`
}

// WorkResult answers exactly one WorkSpec. The identity fields echo the
// spec so the coordinator can refuse a result for another job, partition,
// operation or input.
type WorkResult struct {
	Version          string          `json:"version"`
	JobID            string          `json:"jobId"`
	PartitionID      string          `json:"partitionId"`
	Operation        string          `json:"operation"`
	OperationVersion string          `json:"operationVersion"`
	ContentSHA256    string          `json:"contentSha256"`
	Result           json.RawMessage `json:"result,omitempty"`
	ResultSHA256     string          `json:"resultSha256,omitempty"`
	Error            string          `json:"error,omitempty"`
}

// NewCSVAggregateShardSpec builds the WorkSpec for one planned shard.
func NewCSVAggregateShardSpec(jobID, uri string, plan input.CSVShardPlan, shard input.CSVShard, spec input.PreparationSpec) WorkSpec {
	specJSON := spec.SpecJSON()
	sum := sha256.Sum256(specJSON)
	return WorkSpec{
		Version: WorkSpecVersion, JobID: jobID, PartitionID: shard.ID(),
		Operation: OperationCSVAggregateShard, OperationVersion: OperationCSVAggregateShardVersion,
		Input: WorkInput{
			URI: uri, FileSHA256: plan.SHA256, HeaderLength: plan.HeaderLength,
			Offset: shard.Offset, Length: shard.Length, ContentSHA256: shard.ContentSHA256,
		},
		Spec: specJSON, SpecSHA256: hex.EncodeToString(sum[:]),
	}
}

// Validate checks the spec before any byte is read.
func (w WorkSpec) Validate() error {
	reject := func(format string, args ...any) error {
		return fmt.Errorf("%w: "+format, append([]any{ErrWorkRejected}, args...)...)
	}
	if w.Version != WorkSpecVersion {
		return reject("unsupported work spec version %q", w.Version)
	}
	if w.Operation != OperationCSVAggregateShard || w.OperationVersion != OperationCSVAggregateShardVersion {
		return reject("operation %s/%s is not allowed", w.Operation, w.OperationVersion)
	}
	if !jobIDPattern.MatchString(w.JobID) || !jobIDPattern.MatchString(w.PartitionID) {
		return reject("job and partition ids must be safe identifiers")
	}
	in := w.Input
	if in.HeaderLength <= 0 || in.Offset < in.HeaderLength || in.Length <= 0 || len(in.ContentSHA256) != 64 {
		return reject("the input range is malformed")
	}
	if len(w.Spec) > MaxWorkSpecBytes {
		return reject("the spec is larger than %d bytes", MaxWorkSpecBytes)
	}
	if sum := sha256.Sum256(w.Spec); hex.EncodeToString(sum[:]) != w.SpecSHA256 {
		return reject("the spec does not match its sha256")
	}
	return nil
}

// Execute runs one validated WorkSpec in this process. A computation error
// is reported in WorkResult.Error; only a rejected spec returns an error.
func Execute(ctx context.Context, rs input.Resolver, w WorkSpec) (WorkResult, error) {
	if err := w.Validate(); err != nil {
		return WorkResult{}, err
	}
	res := WorkResult{
		Version: WorkSpecVersion, JobID: w.JobID, PartitionID: w.PartitionID,
		Operation: w.Operation, OperationVersion: w.OperationVersion,
	}
	out, err := aggregateShard(ctx, rs, w)
	if err != nil {
		res.Error = err.Error()
		return res, nil
	}
	sum := sha256.Sum256(out)
	res.ContentSHA256, res.Result, res.ResultSHA256 = w.Input.ContentSHA256, out, hex.EncodeToString(sum[:])
	return res, nil
}

func aggregateShard(ctx context.Context, rs input.Resolver, w WorkSpec) (json.RawMessage, error) {
	var spec input.PreparationSpec
	if err := json.Unmarshal(w.Spec, &spec); err != nil {
		return nil, fmt.Errorf("decode preparation spec: %w", err)
	}
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	header, err := input.OpenRange(ctx, rs, w.Input.URI, 0, w.Input.HeaderLength)
	if err != nil {
		return nil, err
	}
	defer header.Close()
	body, err := input.OpenRange(ctx, rs, w.Input.URI, w.Input.Offset, w.Input.Length)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	h := sha256.New()
	counted := &countingReader{r: io.TeeReader(io.MultiReader(header, body), h)}
	agg, err := input.AggregateCSV(counted, spec, nil, w.Limits.MaxGroups)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(io.Discard, counted); err != nil {
		return nil, err
	}
	if want := w.Input.HeaderLength + w.Input.Length; counted.n != want {
		return nil, fmt.Errorf("%w: read %d bytes of the shard, planned %d", input.ErrVerification, counted.n, want)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != w.Input.ContentSHA256 {
		return nil, fmt.Errorf("%w: the shard bytes changed after planning", input.ErrVerification)
	}
	out, err := json.Marshal(agg)
	if err != nil {
		return nil, err
	}
	limit := w.Limits.MaxResultBytes
	if limit <= 0 {
		limit = DefaultMaxResultBytes
	}
	if int64(len(out)) > limit {
		return nil, fmt.Errorf("the shard result is larger than %d bytes", limit)
	}
	return out, nil
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// Accept checks that res answers w and returns its result. A result for
// another job, partition, operation or input, a failed computation or a
// result whose hash does not match is refused, so a stale or duplicate
// completion can never be merged.
func Accept(w WorkSpec, res WorkResult) (json.RawMessage, error) {
	if res.Error != "" {
		return nil, fmt.Errorf("partition %s: %s", w.PartitionID, res.Error)
	}
	if res.Version != w.Version || res.JobID != w.JobID || res.PartitionID != w.PartitionID ||
		res.Operation != w.Operation || res.OperationVersion != w.OperationVersion || res.ContentSHA256 != w.Input.ContentSHA256 {
		return nil, fmt.Errorf("%w: the result does not answer partition %s of job %s", ErrWorkRejected, w.PartitionID, w.JobID)
	}
	if sum := sha256.Sum256(res.Result); hex.EncodeToString(sum[:]) != res.ResultSHA256 {
		return nil, fmt.Errorf("%w: the result of partition %s does not match its sha256", ErrWorkRejected, w.PartitionID)
	}
	return res.Result, nil
}

// RuntimeMode is where partitions execute. It is independent of the
// ExecutionProfile: a profile chooses the strategy, a mode the placement.
type RuntimeMode string

const (
	RuntimeLocal   RuntimeMode = "LOCAL"
	RuntimeProcess RuntimeMode = "PROCESS"
)

// ParseRuntimeMode accepts local or process (any case); empty is LOCAL.
func ParseRuntimeMode(s string) (RuntimeMode, error) {
	switch m := RuntimeMode(bytes.ToUpper([]byte(s))); m {
	case "":
		return RuntimeLocal, nil
	case RuntimeLocal, RuntimeProcess:
		return m, nil
	}
	return "", fmt.Errorf("unknown runtime %q (want local or process)", s)
}

// Dispatcher delivers one WorkSpec to a worker and returns its answer.
type Dispatcher interface {
	Dispatch(ctx context.Context, w WorkSpec) (WorkResult, error)
	Mode() RuntimeMode
}

// InProcess executes work in this process.
type InProcess struct{ Resolver input.Resolver }

func (d InProcess) Dispatch(ctx context.Context, w WorkSpec) (WorkResult, error) {
	return Execute(ctx, d.Resolver, w)
}

func (InProcess) Mode() RuntimeMode { return RuntimeLocal }

// ProcessDispatcher runs each WorkSpec in a new child process of the same
// binary (`insight-lab worker`), which reads the spec on stdin and writes
// the result on stdout. The child gets no inherited environment, so API
// keys and other secrets never reach it; it reads inputs only through its
// own -input-root.
type ProcessDispatcher struct {
	Executable string
	Args       []string
	// Env is the child's entire environment (for example GOMEMLIMIT).
	Env []string
	// Timeout bounds one partition; zero means 30 minutes.
	Timeout time.Duration
	// MaxResultBytes bounds the child's stdout; zero means
	// DefaultMaxResultBytes plus the envelope.
	MaxResultBytes int64
}

func (d ProcessDispatcher) Mode() RuntimeMode { return RuntimeProcess }

func (d ProcessDispatcher) Dispatch(ctx context.Context, w WorkSpec) (WorkResult, error) {
	if err := w.Validate(); err != nil {
		return WorkResult{}, err
	}
	spec, err := json.Marshal(w)
	if err != nil {
		return WorkResult{}, err
	}
	timeout := d.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, d.Executable, d.Args...)
	cmd.Env = append([]string{}, d.Env...)
	cmd.Stdin = bytes.NewReader(spec)
	limit := d.MaxResultBytes
	if limit <= 0 {
		limit = DefaultMaxResultBytes + 1<<20
	}
	stdout := &cappedBuffer{max: limit}
	stderr := &cappedBuffer{max: 4 << 10}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.WaitDelay = 5 * time.Second
	runErr := cmd.Run()
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return WorkResult{}, fmt.Errorf("partition %s: the worker exceeded %s", w.PartitionID, timeout)
		}
		return WorkResult{}, ctx.Err()
	}
	if runErr != nil {
		return WorkResult{}, fmt.Errorf("partition %s: worker failed: %v: %s", w.PartitionID, runErr, bytes.TrimSpace(stderr.buf.Bytes()))
	}
	if stdout.overflow {
		return WorkResult{}, fmt.Errorf("partition %s: the worker output exceeds %d bytes", w.PartitionID, limit)
	}
	var res WorkResult
	if err := json.Unmarshal(stdout.buf.Bytes(), &res); err != nil {
		return WorkResult{}, fmt.Errorf("partition %s: unreadable worker output: %w", w.PartitionID, err)
	}
	return res, nil
}

// cappedBuffer keeps at most max bytes and records whether more arrived.
type cappedBuffer struct {
	buf      bytes.Buffer
	max      int64
	overflow bool
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if room := c.max - int64(c.buf.Len()); int64(len(p)) > room {
		c.overflow = true
		if room > 0 {
			c.buf.Write(p[:room])
		}
		return len(p), nil
	}
	return c.buf.Write(p)
}

// ServeWorker is the child side of ProcessDispatcher: it reads one
// WorkSpec from in, executes it with rs and writes the WorkResult to out.
func ServeWorker(ctx context.Context, in io.Reader, out io.Writer, rs input.Resolver) error {
	raw, err := io.ReadAll(io.LimitReader(in, 2*MaxWorkSpecBytes+1))
	if err != nil {
		return err
	}
	if len(raw) > 2*MaxWorkSpecBytes {
		return fmt.Errorf("%w: the work spec is too large", ErrWorkRejected)
	}
	var w WorkSpec
	if err := json.Unmarshal(raw, &w); err != nil {
		return fmt.Errorf("%w: unreadable work spec: %v", ErrWorkRejected, err)
	}
	res, err := Execute(ctx, rs, w)
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(res)
}

// WorkerExecutable is the binary PROCESS workers run: this one.
func WorkerExecutable() (string, error) { return os.Executable() }
