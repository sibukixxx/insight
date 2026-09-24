package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"insight-lab/internal/input"
)

func TestResolve(t *testing.T) {
	small := input.Shape{Documents: 3, InlineBytes: 100}
	raw := input.Shape{Documents: 1, RawArtifacts: 1, RawToPrepare: 1, RawToPrepareByte: 1 << 20}
	huge := input.Shape{RawArtifacts: 1, RawToPrepare: 1, RawToPrepareByte: 1 << 40}
	withHeavy, without := DefaultPlanner(Capabilities{HeavyRuntime: true}), DefaultPlanner(Capabilities{})
	tests := []struct {
		name    string
		planner Planner
		req     Profile
		shape   input.Shape
		want    Profile
		wantErr error
	}{
		{"auto small inline resolves light", without, ProfileAuto, small, ProfileLight, nil},
		{"auto raw to prepare resolves standard", without, ProfileAuto, raw, ProfileStandard, nil},
		{"auto huge raw resolves heavy with adapter", withHeavy, ProfileAuto, huge, ProfileHeavy, nil},
		{"auto huge raw without adapter fails instead of downgrading", without, ProfileAuto, huge, "", ErrProfileUnavailable},
		{"light refuses raw preparation", without, ProfileLight, raw, "", ErrProfileUnavailable},
		{"heavy without adapter fails", without, ProfileHeavy, small, "", ErrProfileUnavailable},
		{"explicit standard is kept for small input", without, ProfileStandard, small, ProfileStandard, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.planner.Resolve(tt.req, tt.shape)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got.Resolved != tt.want {
				t.Fatalf("resolved %s, want %s", got.Resolved, tt.want)
			}
			if err == nil && (got.Reason == "" || got.StrategyVersion != StrategyVersion || got.Requested != tt.req) {
				t.Fatalf("resolution not explained: %+v", got)
			}
		})
	}
}

func TestParseRejectsUnknownProfileAndDefaultsToAuto(t *testing.T) {
	if p, err := Parse(""); err != nil || p != ProfileAuto {
		t.Fatalf("empty = %s, %v", p, err)
	}
	if _, err := Parse("TURBO"); !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("got %v", err)
	}
}

func TestLocalRuntimeResumesAfterRestartWithoutRerunningSucceededPartitions(t *testing.T) {
	dir := t.TempDir()
	spec := JobSpec{ID: "job-1", Partitions: []string{"p0", "p1", "p2"}}
	var calls atomic.Int32
	failing := func(_ context.Context, id string) (json.RawMessage, error) {
		calls.Add(1)
		if id == "p2" {
			return nil, errors.New("disk full")
		}
		return json.RawMessage(fmt.Sprintf("%q", id)), nil
	}
	first := NewLocalRuntime(dir, 2)
	st, err := first.Run(context.Background(), spec, failing)
	if err == nil || st.Status != JobFailed || st.Done != 2 {
		t.Fatalf("first run: state=%+v err=%v", st, err)
	}
	// A new runtime instance simulates a process restart.
	restarted := NewLocalRuntime(dir, 2)
	calls.Store(0)
	ok := func(_ context.Context, id string) (json.RawMessage, error) {
		calls.Add(1)
		return json.RawMessage(fmt.Sprintf("%q", id)), nil
	}
	st, err = restarted.Run(context.Background(), spec, ok)
	if err != nil || st.Status != JobSucceeded || calls.Load() != 1 {
		t.Fatalf("resume: state=%+v err=%v calls=%d", st, err, calls.Load())
	}
	for i, p := range st.Partitions {
		if p.ID != spec.Partitions[i] || string(p.Result) != fmt.Sprintf("%q", p.ID) {
			t.Fatalf("partition order/result lost: %+v", st.Partitions)
		}
	}
	// Running a finished job again is an idempotent no-op.
	calls.Store(0)
	if _, err := restarted.Run(context.Background(), spec, ok); err != nil || calls.Load() != 0 {
		t.Fatalf("rerun of succeeded job executed partitions: calls=%d err=%v", calls.Load(), err)
	}
}

func TestLocalRuntimeCancelStopsJobWithCancelledStatus(t *testing.T) {
	rt := NewLocalRuntime("", 1)
	started := make(chan struct{})
	blocking := func(ctx context.Context, id string) (json.RawMessage, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	done := make(chan error, 1)
	go func() {
		_, err := rt.Run(context.Background(), JobSpec{ID: "job-c", Partitions: []string{"only"}}, blocking)
		done <- err
	}()
	<-started
	if err := rt.Cancel(context.Background(), "job-c"); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, ErrCancelled) {
		t.Fatalf("want ErrCancelled, got %v", err)
	}
	st, _ := rt.Status(context.Background(), "job-c")
	if st.Status != JobCancelled {
		t.Fatalf("status = %s", st.Status)
	}
}

func TestLocalRuntimeRejectsUnsafeJobID(t *testing.T) {
	if _, err := NewLocalRuntime(t.TempDir(), 1).Run(context.Background(), JobSpec{ID: "../x", Partitions: []string{"p"}}, nil); err == nil {
		t.Fatal("path-like job id accepted")
	}
}
