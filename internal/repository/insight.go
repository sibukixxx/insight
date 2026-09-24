package repository

import (
	"context"

	"insight-lab/internal/domain"
)

type InsightRepository interface {
	Create(ctx context.Context, insight *domain.Insight) error
	Get(ctx context.Context, id string) (*domain.Insight, error)
	// ListByProject mixes every analysis run of the project. Result views
	// and reports must use ListByAnalysis so runs never blend together.
	ListByProject(ctx context.Context, projectID string) ([]*domain.Insight, error)
	ListByAnalysis(ctx context.Context, analysisID string) ([]*domain.Insight, error)
}

type EvidenceRepository interface {
	CreateBatch(ctx context.Context, evidence []*domain.Evidence) error
	ListByInsight(ctx context.Context, insightID string) ([]*domain.Evidence, error)
	CountByProject(ctx context.Context, projectID string) (int, error)
}
