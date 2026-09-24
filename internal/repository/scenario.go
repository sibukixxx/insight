package repository

import (
	"context"

	"insight-lab/internal/domain"
)

// ScenarioRepository stores scenario set versions and evaluations. Both are
// append-only: there is no update or delete.
type ScenarioRepository interface {
	CreateScenarioSet(ctx context.Context, set *domain.ScenarioSet) error
	GetScenarioSet(ctx context.Context, id string) (*domain.ScenarioSet, error)
	ListScenarioSets(ctx context.Context, researchRunID string) ([]*domain.ScenarioSet, error)
	CreateScenarioEvaluation(ctx context.Context, evaluation *domain.ScenarioEvaluation) error
	ListScenarioEvaluations(ctx context.Context, researchRunID string) ([]*domain.ScenarioEvaluation, error)
}
