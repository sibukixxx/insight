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

const patternColumns = `id, project_id, analysis_id, kind, title, description, expectation, deviation_type, created_at`

func (r *PatternRepository) CreateBatch(ctx context.Context, patterns []*domain.Pattern) error {
	if len(patterns) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, p := range patterns {
		kind := p.Kind
		if kind == "" {
			kind = domain.PatternRepetition
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO patterns (id, project_id, analysis_id, kind, title, description, expectation, deviation_type, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.ID, p.ProjectID, nullableStringLiteral(p.AnalysisID), string(kind), p.Title, p.Description,
			nullableStringLiteral(p.Expectation), nullableStringLiteral(string(p.DeviationType)), formatTime(p.CreatedAt)); err != nil {
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

// ListByProject returns every pattern in the project, deviation (trace)
// patterns first so the "what surprised us" layer is read before the
// "what repeated" layer, then oldest first within each kind.
func (r *PatternRepository) ListByProject(ctx context.Context, projectID string) ([]*domain.Pattern, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+patternColumns+`
		 FROM patterns WHERE project_id = ?
		 ORDER BY CASE kind WHEN 'deviation' THEN 0 ELSE 1 END, created_at ASC`, projectID)
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
		`SELECT p.id, p.project_id, p.analysis_id, p.kind, p.title, p.description, p.expectation, p.deviation_type, p.created_at
		 FROM patterns p
		 JOIN insight_patterns ip ON ip.pattern_id = p.id
		 WHERE ip.insight_id = ?
		 ORDER BY CASE p.kind WHEN 'deviation' THEN 0 ELSE 1 END, p.created_at ASC`, insightID)
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
	var analysisID, description, kind, expectation, deviationType sql.NullString
	var createdAt string
	if err := s.Scan(&p.ID, &p.ProjectID, &analysisID, &kind, &p.Title, &description, &expectation, &deviationType, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	p.AnalysisID = analysisID.String
	p.Description = description.String
	p.Kind = domain.PatternKind(kind.String)
	if p.Kind == "" {
		p.Kind = domain.PatternRepetition
	}
	p.Expectation = expectation.String
	p.DeviationType = domain.DeviationType(deviationType.String)
	t, err := parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	p.CreatedAt = t
	return &p, nil
}
