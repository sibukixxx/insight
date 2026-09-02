package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

type ProjectRepository struct{ db *DB }

func NewProjectRepository(db *DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) Create(ctx context.Context, p *domain.Project) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO projects (id, name, created_at) VALUES (?, ?, ?)`,
		p.ID, p.Name, formatTime(p.CreatedAt))
	return err
}

func (r *ProjectRepository) Get(ctx context.Context, id string) (*domain.Project, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, intake_profile, created_at FROM projects WHERE id = ?`, id)
	return scanProject(row)
}

func (r *ProjectRepository) List(ctx context.Context) ([]*domain.Project, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, intake_profile, created_at FROM projects ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *ProjectRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *ProjectRepository) UpdateIntakeProfile(ctx context.Context, id string, profile domain.IntakeProfile) error {
	b, err := json.Marshal(profile)
	if err != nil {
		return fmt.Errorf("encode intake profile: %w", err)
	}
	res, err := r.db.ExecContext(ctx, `UPDATE projects SET intake_profile = ? WHERE id = ?`, string(b), id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return repository.ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProject(s scanner) (*domain.Project, error) {
	var p domain.Project
	var createdAt string
	var profile sql.NullString
	if err := s.Scan(&p.ID, &p.Name, &profile, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	if profile.Valid && profile.String != "" {
		if err := json.Unmarshal([]byte(profile.String), &p.IntakeProfile); err != nil {
			return nil, fmt.Errorf("decode intake profile for %s: %w", p.ID, err)
		}
	}
	t, err := parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	p.CreatedAt = t
	return &p, nil
}
