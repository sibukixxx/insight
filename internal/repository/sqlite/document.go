package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

type DocumentRepository struct{ db *DB }

func NewDocumentRepository(db *DB) *DocumentRepository {
	return &DocumentRepository{db: db}
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (r *DocumentRepository) Create(ctx context.Context, d *domain.Document) error {
	return insertDocument(ctx, r.db, d)
}

func (r *DocumentRepository) CreateBatch(ctx context.Context, docs []*domain.Document) error {
	if len(docs) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, d := range docs {
		if err := insertDocument(ctx, tx, d); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func insertDocument(ctx context.Context, ex execer, d *domain.Document) error {
	meta, err := encodeMetadata(d.Metadata)
	if err != nil {
		return err
	}
	_, err = ex.ExecContext(ctx,
		`INSERT INTO documents (id, project_id, source, title, content, metadata, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.ProjectID, string(d.Source), d.Title, d.Content, meta, formatTime(d.CreatedAt))
	return err
}

func (r *DocumentRepository) Get(ctx context.Context, id string) (*domain.Document, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, source, title, content, metadata, created_at
		 FROM documents WHERE id = ?`, id)
	return scanDocument(row)
}

func (r *DocumentRepository) ListByProject(ctx context.Context, projectID string) ([]*domain.Document, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, source, title, content, metadata, created_at
		 FROM documents WHERE project_id = ? ORDER BY created_at ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Document
	for rows.Next() {
		d, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func scanDocument(s scanner) (*domain.Document, error) {
	var d domain.Document
	var source, createdAt string
	var meta sql.NullString
	if err := s.Scan(&d.ID, &d.ProjectID, &source, &d.Title, &d.Content, &meta, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	d.Source = domain.SourceType(source)

	t, err := parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	d.CreatedAt = t

	m, err := decodeMetadata(meta)
	if err != nil {
		return nil, err
	}
	d.Metadata = m

	return &d, nil
}
