package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

type AnalysisRepository struct{ db *DB }

func NewAnalysisRepository(db *DB) *AnalysisRepository {
	return &AnalysisRepository{db: db}
}

func (r *AnalysisRepository) Create(ctx context.Context, a *domain.Analysis) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO analyses (id, project_id, status, current_step, progress, error, metrics, started_at, finished_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.ProjectID, string(a.Status), a.CurrentStep, a.Progress, a.Error, a.Metrics,
		nullableTime(a.StartedAt), nullableTime(a.FinishedAt), formatTime(a.CreatedAt))
	return err
}

func (r *AnalysisRepository) Update(ctx context.Context, a *domain.Analysis) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE analyses SET status = ?, current_step = ?, progress = ?, error = ?, metrics = ?, started_at = ?, finished_at = ?
		 WHERE id = ?`,
		string(a.Status), a.CurrentStep, a.Progress, a.Error, a.Metrics,
		nullableTime(a.StartedAt), nullableTime(a.FinishedAt), a.ID)
	return err
}

func (r *AnalysisRepository) Get(ctx context.Context, id string) (*domain.Analysis, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, status, current_step, progress, error, metrics, started_at, finished_at, created_at
		 FROM analyses WHERE id = ?`, id)
	return scanAnalysis(row)
}

func (r *AnalysisRepository) ListByProject(ctx context.Context, projectID string) ([]*domain.Analysis, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, status, current_step, progress, error, metrics, started_at, finished_at, created_at
		 FROM analyses WHERE project_id = ? ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Analysis
	for rows.Next() {
		a, err := scanAnalysis(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *AnalysisRepository) LatestByProject(ctx context.Context, projectID string) (*domain.Analysis, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, status, current_step, progress, error, metrics, started_at, finished_at, created_at
		 FROM analyses WHERE project_id = ? ORDER BY created_at DESC LIMIT 1`, projectID)
	return scanAnalysis(row)
}

func (r *AnalysisRepository) FailInterrupted(ctx context.Context) (int, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE analyses SET status = 'failed', error = 'interrupted', finished_at = ?
		 WHERE status IN ('queued', 'running')`, formatTime(time.Now().UTC()))
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}

func scanAnalysis(s scanner) (*domain.Analysis, error) {
	var a domain.Analysis
	var status, createdAt string
	var currentStep, errMsg, metrics sql.NullString
	var startedAt, finishedAt sql.NullString

	if err := s.Scan(&a.ID, &a.ProjectID, &status, &currentStep, &a.Progress, &errMsg, &metrics,
		&startedAt, &finishedAt, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	a.Status = domain.AnalysisStatus(status)
	a.CurrentStep = currentStep.String
	a.Error = errMsg.String
	a.Metrics = metrics.String

	created, err := parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	a.CreatedAt = created

	if startedAt.Valid {
		t, err := parseTime(startedAt.String)
		if err != nil {
			return nil, err
		}
		a.StartedAt = &t
	}
	if finishedAt.Valid {
		t, err := parseTime(finishedAt.String)
		if err != nil {
			return nil, err
		}
		a.FinishedAt = &t
	}
	return &a, nil
}
