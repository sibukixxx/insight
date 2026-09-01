package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

type EvidenceRepository struct{ db *DB }

func NewEvidenceRepository(db *DB) *EvidenceRepository {
	return &EvidenceRepository{db: db}
}

func (r *EvidenceRepository) CreateBatch(ctx context.Context, evidence []*domain.Evidence) error {
	if len(evidence) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, e := range evidence {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO evidence (id, insight_id, document_id, observation_id, quote, evidence_type, relevance_score, start_offset, end_offset)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.ID, e.InsightID, e.DocumentID, nullableString(e.ObservationID), e.Quote, string(e.Type),
			e.RelevanceScore, e.StartOffset, e.EndOffset); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (r *EvidenceRepository) ListByInsight(ctx context.Context, insightID string) ([]*domain.Evidence, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, insight_id, document_id, observation_id, quote, evidence_type, relevance_score, start_offset, end_offset
		 FROM evidence WHERE insight_id = ? ORDER BY relevance_score DESC`, insightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Evidence
	for rows.Next() {
		e, err := scanEvidence(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *EvidenceRepository) CountByProject(ctx context.Context, projectID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM evidence e JOIN insights i ON i.id = e.insight_id WHERE i.project_id = ?`,
		projectID).Scan(&n)
	return n, err
}

func scanEvidence(s scanner) (*domain.Evidence, error) {
	var e domain.Evidence
	var observationID sql.NullString
	var evidenceType string
	if err := s.Scan(&e.ID, &e.InsightID, &e.DocumentID, &observationID, &e.Quote, &evidenceType,
		&e.RelevanceScore, &e.StartOffset, &e.EndOffset); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	e.Type = domain.EvidenceType(evidenceType)
	if observationID.Valid {
		v := observationID.String
		e.ObservationID = &v
	}
	return &e, nil
}
