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
	LatestByProject(ctx context.Context, projectID string) (*domain.Analysis, error)
	// FailInterrupted marks any analysis left in "queued" or "running" state
	// (e.g. from a process that was killed mid-run) as failed, and returns
	// how many rows it touched. Called once at startup.
	FailInterrupted(ctx context.Context) (int, error)
}
