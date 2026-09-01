package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

type ObservationRepository struct{ db *DB }

func NewObservationRepository(db *DB) *ObservationRepository {
	return &ObservationRepository{db: db}
}

func (r *ObservationRepository) CreateBatch(ctx context.Context, obs []*domain.Observation) error {
	if len(obs) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, o := range obs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO observations (id, document_id, quote, start_offset, end_offset, behavior, topic, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			o.ID, o.DocumentID, o.Quote, o.StartOffset, o.EndOffset, o.Behavior, o.Topic, formatTime(o.CreatedAt)); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (r *ObservationRepository) Get(ctx context.Context, id string) (*domain.Observation, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, document_id, quote, start_offset, end_offset, behavior, topic, created_at
		 FROM observations WHERE id = ?`, id)
	return scanObservation(row)
}

func (r *ObservationRepository) ListByDocument(ctx context.Context, documentID string) ([]*domain.Observation, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, document_id, quote, start_offset, end_offset, behavior, topic, created_at
		 FROM observations WHERE document_id = ? ORDER BY start_offset ASC`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanObservations(rows)
}

func (r *ObservationRepository) ListByProject(ctx context.Context, projectID string) ([]*domain.Observation, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT o.id, o.document_id, o.quote, o.start_offset, o.end_offset, o.behavior, o.topic, o.created_at
		 FROM observations o
		 JOIN documents d ON d.id = o.document_id
		 WHERE d.project_id = ?
		 ORDER BY o.created_at ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanObservations(rows)
}

func scanObservations(rows *sql.Rows) ([]*domain.Observation, error) {
	var out []*domain.Observation
	for rows.Next() {
		o, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func scanObservation(s scanner) (*domain.Observation, error) {
	var o domain.Observation
	var createdAt string
	var topic sql.NullString
	if err := s.Scan(&o.ID, &o.DocumentID, &o.Quote, &o.StartOffset, &o.EndOffset, &o.Behavior, &topic, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	o.Topic = topic.String
	t, err := parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	o.CreatedAt = t
	return &o, nil
}
