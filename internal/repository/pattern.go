package repository

import (
	"context"

	"insight-lab/internal/domain"
)

type PatternRepository interface {
	// CreateBatch persists patterns along with their pattern_observations
	// links. Callers must have already filtered ObservationIDs down to
	// IDs that actually exist (see internal/service/pipeline.go
	// buildPatterns) - a pattern citing an observation that was never
	// grounded would reintroduce exactly the hallucination risk grounding
	// exists to prevent.
	CreateBatch(ctx context.Context, patterns []*domain.Pattern) error
	ListByProject(ctx context.Context, projectID string) ([]*domain.Pattern, error)
	// LinkInsight records which patterns fed into an insight's hypothesis,
	// making the reasoning chain (Observation -> Pattern -> Insight)
	// traceable end to end.
	LinkInsight(ctx context.Context, insightID string, patternIDs []string) error
	ListByInsight(ctx context.Context, insightID string) ([]*domain.Pattern, error)
}
