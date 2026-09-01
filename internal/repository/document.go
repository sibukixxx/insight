package repository

import (
	"context"

	"insight-lab/internal/domain"
)

type DocumentRepository interface {
	Create(ctx context.Context, d *domain.Document) error
	CreateBatch(ctx context.Context, docs []*domain.Document) error
	Get(ctx context.Context, id string) (*domain.Document, error)
	ListByProject(ctx context.Context, projectID string) ([]*domain.Document, error)
}
