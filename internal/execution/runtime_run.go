package execution

import (
	"context"
	"fmt"
	"sync"
)

// resume keeps succeeded partitions of a previous attempt and resets the
// rest. Partition order follows the spec so results merge deterministically.
func (r *LocalRuntime) resume(prev *JobState, spec JobSpec) *JobState {
	done := map[string]PartitionState{}
	if prev != nil {
		for _, p := range prev.Partitions {
			if p.Status == JobSucceeded {
				done[p.ID] = p
			}
		}
	}
	s := &JobState{ID: spec.ID, Status: JobRunning, Total: len(spec.Partitions)}
	for _, id := range spec.Partitions {
		if p, ok := done[id]; ok {
			s.Partitions = append(s.Partitions, p)
			continue
		}
		s.Partitions = append(s.Partitions, PartitionState{ID: id, Status: JobRunning})
	}
	return s
}

// runPartitions processes pending partitions with bounded concurrency; each
// failure is retried up to MaxAttempts and reported per partition.
func (r *LocalRuntime) runPartitions(ctx context.Context, s *JobState, fn PartitionFunc) {
	workers := r.Concurrency
	if workers <= 0 {
		workers = 1
	}
	maxAttempts := r.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i := range s.Partitions {
		r.mu.Lock()
		pending := s.Partitions[i].Status != JobSucceeded
		id := s.Partitions[i].ID
		r.mu.Unlock()
		if !pending {
			continue
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return
		}
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			defer func() { <-sem }()
			for {
				if ctx.Err() != nil {
					return
				}
				result, err := fn(ctx, id)
				r.mu.Lock()
				p := &s.Partitions[i]
				p.Attempts++
				if err == nil {
					p.Status, p.Result, p.Error = JobSucceeded, result, ""
				} else {
					p.Error = err.Error()
					if p.Attempts >= maxAttempts || ctx.Err() != nil {
						p.Status = JobFailed
					}
				}
				_ = r.persist(s)
				retry := err != nil && p.Status != JobFailed
				r.mu.Unlock()
				if !retry {
					return
				}
			}
		}(i, id)
	}
	wg.Wait()
}

func (r *LocalRuntime) finish(ctx context.Context, s *JobState) (JobState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var failed []string
	for _, p := range s.Partitions {
		if p.Status != JobSucceeded {
			failed = append(failed, p.ID)
		}
	}
	switch {
	case s.Status == JobCancelled || (ctx.Err() != nil && len(failed) > 0):
		s.Status = JobCancelled
		_ = r.persist(s)
		return r.snapshot(s), fmt.Errorf("%w: %s", ErrCancelled, s.ID)
	case len(failed) > 0:
		s.Status = JobFailed
		_ = r.persist(s)
		return r.snapshot(s), fmt.Errorf("heavy job %s: %d partitions failed: %v", s.ID, len(failed), failed)
	}
	s.Status = JobSucceeded
	if err := r.persist(s); err != nil {
		return JobState{}, err
	}
	return r.snapshot(s), nil
}
