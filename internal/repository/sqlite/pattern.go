package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

type PatternRepository struct{ db *DB }

func NewPatternRepository(db *DB) *PatternRepository {
	return &PatternRepository{db: db}
}

func (r *PatternRepository) CreateBatch(ctx context.Context, patterns []*domain.Pattern) error {
	if len(patterns) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, p := range patterns {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO patterns (id, project_id, analysis_id, title, description, created_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			p.ID, p.ProjectID, nullableStringLiteral(p.AnalysisID), p.Title, p.Description, formatTime(p.CreatedAt)); err != nil {
			tx.Rollback()
			return err
		}
		for _, oid := range p.ObservationIDs {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO pattern_observations (pattern_id, observation_id) VALUES (?, ?)`,
				p.ID, oid); err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	return tx.Commit()
}

func (r *PatternRepository) ListByProject(ctx context.Context, projectID string) ([]*domain.Pattern, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, analysis_id, title, description, created_at
		 FROM patterns WHERE project_id = ? ORDER BY created_at ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	patterns, err := scanPatterns(rows)
	if err != nil {
		return nil, err
	}
	if err := r.attachObservationIDs(ctx, patterns); err != nil {
		return nil, err
	}
	return patterns, nil
}

func (r *PatternRepository) LinkInsight(ctx context.Context, insightID string, patternIDs []string) error {
	if len(patternIDs) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, pid := range patternIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO insight_patterns (insight_id, pattern_id) VALUES (?, ?)`,
			insightID, pid); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (r *PatternRepository) ListByInsight(ctx context.Context, insightID string) ([]*domain.Pattern, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT p.id, p.project_id, p.analysis_id, p.title, p.description, p.created_at
		 FROM patterns p
		 JOIN insight_patterns ip ON ip.pattern_id = p.id
		 WHERE ip.insight_id = ?
		 ORDER BY p.created_at ASC`, insightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	patterns, err := scanPatterns(rows)
	if err != nil {
		return nil, err
	}
	if err := r.attachObservationIDs(ctx, patterns); err != nil {
		return nil, err
	}
	return patterns, nil
}

func (r *PatternRepository) attachObservationIDs(ctx context.Context, patterns []*domain.Pattern) error {
	if len(patterns) == 0 {
		return nil
	}
	byID := make(map[string]*domain.Pattern, len(patterns))
	placeholders := make([]string, len(patterns))
	args := make([]any, len(patterns))
	for i, p := range patterns {
		byID[p.ID] = p
		placeholders[i] = "?"
		args[i] = p.ID
	}

	query := fmt.Sprintf(`SELECT pattern_id, observation_id FROM pattern_observations WHERE pattern_id IN (%s)`,
		strings.Join(placeholders, ","))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var patternID, observationID string
		if err := rows.Scan(&patternID, &observationID); err != nil {
			return err
		}
		if p, ok := byID[patternID]; ok {
			p.ObservationIDs = append(p.ObservationIDs, observationID)
		}
	}
	return rows.Err()
}

func nullableStringLiteral(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func scanPatterns(rows *sql.Rows) ([]*domain.Pattern, error) {
	var out []*domain.Pattern
	for rows.Next() {
		p, err := scanPattern(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanPattern(s scanner) (*domain.Pattern, error) {
	var p domain.Pattern
	var analysisID sql.NullString
	var description sql.NullString
	var createdAt string
	if err := s.Scan(&p.ID, &p.ProjectID, &analysisID, &p.Title, &description, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	p.AnalysisID = analysisID.String
	p.Description = description.String
	t, err := parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	p.CreatedAt = t
	return &p, nil
}
