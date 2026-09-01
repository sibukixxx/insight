package repository

import (
	"context"

	"insight-lab/internal/domain"
)

type ObservationRepository interface {
	CreateBatch(ctx context.Context, obs []*domain.Observation) error
	Get(ctx context.Context, id string) (*domain.Observation, error)
	ListByProject(ctx context.Context, projectID string) ([]*domain.Observation, error)
	ListByDocument(ctx context.Context, documentID string) ([]*domain.Observation, error)
}
