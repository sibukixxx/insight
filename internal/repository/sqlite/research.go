package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

type ResearchRepository struct{ db *DB }

func NewResearchRepository(db *DB) *ResearchRepository { return &ResearchRepository{db: db} }

func (r *ResearchRepository) CreateRun(ctx context.Context, run *domain.ResearchRun) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO research_runs (id, project_id, question, created_at) VALUES (?, ?, ?, ?)`, run.ID, run.ProjectID, run.Question, formatTime(run.CreatedAt)); err != nil {
		return err
	}
	for _, iteration := range run.Iterations {
		if err = insertIteration(ctx, tx, run.ID, iteration); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func insertIteration(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, runID string, iteration domain.ResearchIteration) error {
	payload, err := json.Marshal(iteration)
	if err != nil {
		return err
	}
	_, err = exec.ExecContext(ctx, `INSERT INTO research_iterations (id, research_run_id, sequence, payload, created_at) VALUES (?, ?, ?, ?, ?)`, iteration.ID, runID, iteration.Sequence, string(payload), formatTime(iteration.CreatedAt))
	return err
}

func (r *ResearchRepository) GetResearchRun(ctx context.Context, id string) (*domain.ResearchRun, error) {
	var run domain.ResearchRun
	var created string
	if err := r.db.QueryRowContext(ctx, `SELECT id, project_id, question, created_at FROM research_runs WHERE id = ?`, id).Scan(&run.ID, &run.ProjectID, &run.Question, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	var err error
	run.CreatedAt, err = parseTime(created)
	if err != nil {
		return nil, err
	}
	run.Iterations, err = r.ListResearchIterations(ctx, id)
	return &run, err
}

func (r *ResearchRepository) ListResearchRuns(ctx context.Context, projectID string) ([]*domain.ResearchRun, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM research_runs WHERE project_id = ? ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var out []*domain.ResearchRun
	for _, id := range ids {
		run, err := r.GetResearchRun(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, nil
}

func (r *ResearchRepository) ListResearchIterations(ctx context.Context, runID string) ([]domain.ResearchIteration, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT payload FROM research_iterations WHERE research_run_id = ? ORDER BY sequence`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ResearchIteration
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var it domain.ResearchIteration
		if err := json.Unmarshal([]byte(raw), &it); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *ResearchRepository) GetResearchIteration(ctx context.Context, runID, iterationID string) (*domain.ResearchIteration, error) {
	var raw string
	if err := r.db.QueryRowContext(ctx, `SELECT payload FROM research_iterations WHERE research_run_id = ? AND id = ?`, runID, iterationID).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	var out domain.ResearchIteration
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *ResearchRepository) AppendResearchIteration(ctx context.Context, runID string, iteration domain.ResearchIteration) error {
	return insertIteration(ctx, r.db, runID, iteration)
}

func (r *ResearchRepository) SaveHumanEvaluation(ctx context.Context, evaluation *domain.HumanEvaluation) error {
	payload, err := json.Marshal(evaluation)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO human_evaluations (research_run_id, iteration_id, payload, evaluated_at) VALUES (?, ?, ?, ?) ON CONFLICT(research_run_id, iteration_id) DO UPDATE SET payload=excluded.payload, evaluated_at=excluded.evaluated_at`, evaluation.ResearchRunID, evaluation.IterationID, string(payload), formatTime(evaluation.EvaluatedAt))
	return err
}

func (r *ResearchRepository) GetHumanEvaluation(ctx context.Context, runID, iterationID string) (*domain.HumanEvaluation, error) {
	var raw string
	if err := r.db.QueryRowContext(ctx, `SELECT payload FROM human_evaluations WHERE research_run_id = ? AND iteration_id = ?`, runID, iterationID).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	var out domain.HumanEvaluation
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
