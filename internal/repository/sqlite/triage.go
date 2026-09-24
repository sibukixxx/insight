package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"insight-lab/internal/repository"
)

// TriageRepository persists Dataset Profiles and Selection Plans (#92).
type TriageRepository struct{ db *DB }

func NewTriageRepository(db *DB) *TriageRepository { return &TriageRepository{db: db} }

func (r *TriageRepository) CreateProfile(ctx context.Context, p *repository.StoredDatasetProfile) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO dataset_profiles (profile_id, project_id, body, created_at) VALUES (?, ?, ?, ?)`,
		p.ProfileID, p.ProjectID, string(p.Body), formatTime(p.CreatedAt))
	if isUniqueViolation(err) {
		return repository.ErrConflict
	}
	return err
}

func (r *TriageRepository) GetProfile(ctx context.Context, profileID string) (*repository.StoredDatasetProfile, error) {
	var p repository.StoredDatasetProfile
	var body, createdAt string
	err := r.db.QueryRowContext(ctx, `SELECT profile_id, project_id, body, created_at FROM dataset_profiles WHERE profile_id = ?`, profileID).
		Scan(&p.ProfileID, &p.ProjectID, &body, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.Body = []byte(body)
	if p.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *TriageRepository) CreatePlan(ctx context.Context, p *repository.StoredSelectionPlan) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO selection_plans (plan_id, project_id, profile_id, version, parent_plan_id, body, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.PlanID, p.ProjectID, p.ProfileID, p.Version, nullableStringLiteral(p.ParentPlanID), string(p.Body), formatTime(p.CreatedAt))
	if isUniqueViolation(err) {
		return repository.ErrConflict
	}
	return err
}

const planColumns = `plan_id, project_id, profile_id, version, parent_plan_id, body, created_at`

func scanPlan(sc interface{ Scan(...any) error }) (*repository.StoredSelectionPlan, error) {
	var p repository.StoredSelectionPlan
	var parent sql.NullString
	var body, createdAt string
	if err := sc.Scan(&p.PlanID, &p.ProjectID, &p.ProfileID, &p.Version, &parent, &body, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	p.ParentPlanID, p.Body = parent.String, []byte(body)
	t, err := parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	p.CreatedAt = t
	return &p, nil
}

func (r *TriageRepository) GetPlan(ctx context.Context, planID string) (*repository.StoredSelectionPlan, error) {
	return scanPlan(r.db.QueryRowContext(ctx, `SELECT `+planColumns+` FROM selection_plans WHERE plan_id = ?`, planID))
}

func (r *TriageRepository) ListPlans(ctx context.Context, profileID string) ([]*repository.StoredSelectionPlan, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+planColumns+` FROM selection_plans WHERE profile_id = ? ORDER BY version`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*repository.StoredSelectionPlan
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
