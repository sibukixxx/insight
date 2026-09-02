package repository

import (
	"context"

	"insight-lab/internal/domain"
)

type ProjectRepository interface {
	Create(ctx context.Context, p *domain.Project) error
	Get(ctx context.Context, id string) (*domain.Project, error)
	List(ctx context.Context) ([]*domain.Project, error)
	Delete(ctx context.Context, id string) error
	// UpdateIntakeProfile replaces the project's intake profile.
	UpdateIntakeProfile(ctx context.Context, id string, profile domain.IntakeProfile) error
}
