package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

type InsightRepository struct{ db *DB }

func NewInsightRepository(db *DB) *InsightRepository {
	return &InsightRepository{db: db}
}

func (r *InsightRepository) Create(ctx context.Context, insight *domain.Insight) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO insights (id, project_id, analysis_id, title, observation, stated_need, latent_need, jtbd,
		 rationale, interpretation, alternative_interpretation, product_opportunity, monetization_angle, confidence, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		insight.ID, insight.ProjectID, nullableString(insight.AnalysisID), insight.Title, insight.Observation,
		insight.StatedNeed, insight.LatentNeed, insight.JTBD, insight.Rationale, insight.Interpretation,
		insight.AlternativeInterpretation, insight.ProductOpportunity, insight.MonetizationAngle, insight.Confidence,
		formatTime(insight.CreatedAt))
	return err
}

func (r *InsightRepository) Get(ctx context.Context, id string) (*domain.Insight, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, analysis_id, title, observation, stated_need, latent_need, jtbd,
		 rationale, interpretation, alternative_interpretation, product_opportunity, monetization_angle, confidence, created_at
		 FROM insights WHERE id = ?`, id)
	return scanInsight(row)
}

func (r *InsightRepository) ListByProject(ctx context.Context, projectID string) ([]*domain.Insight, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, analysis_id, title, observation, stated_need, latent_need, jtbd,
		 rationale, interpretation, alternative_interpretation, product_opportunity, monetization_angle, confidence, created_at
		 FROM insights WHERE project_id = ? ORDER BY confidence DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Insight
	for rows.Next() {
		i, err := scanInsight(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func nullableString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func scanInsight(s scanner) (*domain.Insight, error) {
	var i domain.Insight
	var createdAt string
	var analysisID sql.NullString
	var rationale sql.NullString
	var monetizationAngle sql.NullString
	if err := s.Scan(&i.ID, &i.ProjectID, &analysisID, &i.Title, &i.Observation, &i.StatedNeed, &i.LatentNeed,
		&i.JTBD, &rationale, &i.Interpretation, &i.AlternativeInterpretation, &i.ProductOpportunity, &monetizationAngle,
		&i.Confidence, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	if analysisID.Valid {
		v := analysisID.String
		i.AnalysisID = &v
	}
	i.Rationale = rationale.String
	i.MonetizationAngle = monetizationAngle.String
	t, err := parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	i.CreatedAt = t
	return &i, nil
}
