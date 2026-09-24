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

type PublicRepository struct{ db *DB }

func NewPublicRepository(db *DB) *PublicRepository { return &PublicRepository{db: db} }

func (r *PublicRepository) CreateSubject(ctx context.Context, project *domain.Project, subject *repository.PublicSubject) error {
	metadata, err := encodeMetadata(subject.Metadata)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO projects (id, name, created_at) VALUES (?, ?, ?)`,
		project.ID, project.Name, formatTime(project.CreatedAt)); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO public_subjects (project_id, namespace, external_id, subject_type, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		subject.ProjectID, subject.Namespace, subject.ExternalID, nullableStringLiteral(subject.Type), metadata, formatTime(subject.CreatedAt)); err != nil {
		tx.Rollback()
		if isUniqueViolation(err) {
			return repository.ErrConflict
		}
		return err
	}
	return tx.Commit()
}

const publicSubjectColumns = `project_id, namespace, external_id, subject_type, metadata, created_at`

func (r *PublicRepository) GetSubject(ctx context.Context, projectID string) (*repository.PublicSubject, error) {
	return scanPublicSubject(r.db.QueryRowContext(ctx, `SELECT `+publicSubjectColumns+` FROM public_subjects WHERE project_id = ?`, projectID))
}

func (r *PublicRepository) FindSubject(ctx context.Context, namespace, externalID string) (*repository.PublicSubject, error) {
	return scanPublicSubject(r.db.QueryRowContext(ctx,
		`SELECT `+publicSubjectColumns+` FROM public_subjects WHERE namespace = ? AND external_id = ?`, namespace, externalID))
}

func scanPublicSubject(row *sql.Row) (*repository.PublicSubject, error) {
	var s repository.PublicSubject
	var subjectType, metadata sql.NullString
	var createdAt string
	if err := row.Scan(&s.ProjectID, &s.Namespace, &s.ExternalID, &subjectType, &metadata, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	s.Type = subjectType.String
	decoded, err := decodeMetadata(metadata)
	if err != nil {
		return nil, fmt.Errorf("decode subject metadata: %w", err)
	}
	s.Metadata = decoded
	t, err := parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	s.CreatedAt = t
	return &s, nil
}

func (r *PublicRepository) GetResponse(ctx context.Context, key string) (*repository.IdempotentResponse, error) {
	var resp repository.IdempotentResponse
	var body, createdAt string
	err := r.db.QueryRowContext(ctx,
		`SELECT idempotency_key, request_hash, status_code, body, created_at FROM public_idempotent_responses WHERE idempotency_key = ?`, key).
		Scan(&resp.Key, &resp.RequestHash, &resp.StatusCode, &body, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	resp.Body = []byte(body)
	if resp.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (r *PublicRepository) SaveResponse(ctx context.Context, resp *repository.IdempotentResponse) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO public_idempotent_responses (idempotency_key, request_hash, status_code, body, created_at) VALUES (?, ?, ?, ?, ?)`,
		resp.Key, resp.RequestHash, resp.StatusCode, string(resp.Body), formatTime(resp.CreatedAt))
	if isUniqueViolation(err) {
		return repository.ErrConflict
	}
	return err
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
