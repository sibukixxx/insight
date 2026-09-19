package repository

import (
	"context"

	"insight-lab/internal/domain"
)

type ResearchRepository interface {
	CreateRun(ctx context.Context, run *domain.ResearchRun) error
	GetResearchRun(ctx context.Context, id string) (*domain.ResearchRun, error)
	ListResearchRuns(ctx context.Context, projectID string) ([]*domain.ResearchRun, error)
	GetResearchIteration(ctx context.Context, runID, iterationID string) (*domain.ResearchIteration, error)
	ListResearchIterations(ctx context.Context, runID string) ([]domain.ResearchIteration, error)
	AppendResearchIteration(ctx context.Context, runID string, iteration domain.ResearchIteration) error
	UpdateResearchIteration(ctx context.Context, runID string, iteration domain.ResearchIteration) error
	SaveHumanEvaluation(ctx context.Context, evaluation *domain.HumanEvaluation) error
	GetHumanEvaluation(ctx context.Context, runID, iterationID string) (*domain.HumanEvaluation, error)
}
