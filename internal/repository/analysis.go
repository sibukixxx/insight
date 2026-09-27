package repository

import (
	"context"

	"insight-lab/internal/domain"
)

type AnalysisRepository interface {
	Create(ctx context.Context, a *domain.Analysis) error
	Get(ctx context.Context, id string) (*domain.Analysis, error)
	Update(ctx context.Context, a *domain.Analysis) error
	ListByProject(ctx context.Context, projectID string) ([]*domain.Analysis, error)
	// LatestByProject is the most recently created analysis in any status.
	// Result views must not use it: a queued, running or failed run has no
	// results. Use LatestCompletedByProject instead.
	LatestByProject(ctx context.Context, projectID string) (*domain.Analysis, error)
	// LatestCompletedByProject is the most recently finished completed
	// analysis, or ErrNotFound when the project has none.
	LatestCompletedByProject(ctx context.Context, projectID string) (*domain.Analysis, error)
	// FailInterrupted marks any analysis left "running" by a process that
	// stopped mid-run as failed with FailureInterrupted, and returns how many
	// rows it touched. Queued analyses stay queued: the table is the queue
	// of record. Called once at startup.
	FailInterrupted(ctx context.Context) (int, error)
	// CountQueued counts analyses waiting for a worker.
	CountQueued(ctx context.Context) (int, error)
	// ClaimNextQueued moves the oldest queued analysis to running and
	// returns it, or ErrNotFound when none is queued.
	ClaimNextQueued(ctx context.Context) (*domain.Analysis, error)
	// CancelQueued fails a still-queued analysis with FailureCancelled.
	// It reports false when the analysis is no longer queued.
	CancelQueued(ctx context.Context, id, reason string) (bool, error)
	// RequestCancel records a cancel request on a running analysis. It
	// reports false when the analysis is not running or already has one.
	RequestCancel(ctx context.Context, id string) (bool, error)
	// FinishRunning writes a's terminal status only while the stored run is
	// still running; completing additionally requires that no cancel was
	// requested. It reports false when the guard did not hold.
	FinishRunning(ctx context.Context, a *domain.Analysis) (bool, error)
	// FindRetry returns the analysis retrying id, or ErrNotFound.
	FindRetry(ctx context.Context, id string) (*domain.Analysis, error)
}
