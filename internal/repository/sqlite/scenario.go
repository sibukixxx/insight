package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

type ScenarioRepository struct{ db *DB }

func NewScenarioRepository(db *DB) *ScenarioRepository { return &ScenarioRepository{db: db} }

func (r *ScenarioRepository) CreateScenarioSet(ctx context.Context, set *domain.ScenarioSet) error {
	payload, err := json.Marshal(set)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO scenario_sets (id, research_run_id, version, payload, created_at) VALUES (?, ?, ?, ?, ?)`,
		set.ID, set.ResearchRunID, set.Version, string(payload), formatTime(set.CreatedAt))
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return repository.ErrConflict
	}
	return err
}

func (r *ScenarioRepository) GetScenarioSet(ctx context.Context, id string) (*domain.ScenarioSet, error) {
	var payload string
	if err := r.db.QueryRowContext(ctx, `SELECT payload FROM scenario_sets WHERE id = ?`, id).Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	var set domain.ScenarioSet
	return &set, json.Unmarshal([]byte(payload), &set)
}

func (r *ScenarioRepository) ListScenarioSets(ctx context.Context, researchRunID string) ([]*domain.ScenarioSet, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT payload FROM scenario_sets WHERE research_run_id = ? ORDER BY version`, researchRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*domain.ScenarioSet{}
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var set domain.ScenarioSet
		if err := json.Unmarshal([]byte(payload), &set); err != nil {
			return nil, err
		}
		out = append(out, &set)
	}
	return out, rows.Err()
}

func (r *ScenarioRepository) CreateScenarioEvaluation(ctx context.Context, e *domain.ScenarioEvaluation) error {
	payload, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO scenario_evaluations (id, research_run_id, scenario_set_id, iteration_id, payload, evaluated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		e.ID, e.ResearchRunID, e.ScenarioSetID, nullString(e.IterationID), string(payload), formatTime(e.EvaluatedAt))
	return err
}

func (r *ScenarioRepository) ListScenarioEvaluations(ctx context.Context, researchRunID string) ([]*domain.ScenarioEvaluation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT payload FROM scenario_evaluations WHERE research_run_id = ? ORDER BY evaluated_at, rowid`, researchRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*domain.ScenarioEvaluation{}
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var e domain.ScenarioEvaluation
		if err := json.Unmarshal([]byte(payload), &e); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
