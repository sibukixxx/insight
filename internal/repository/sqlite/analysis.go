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

const analysisColumns = `id, project_id, status, current_step, progress, error, metrics, started_at, finished_at, created_at,
	 label, note, semantic_analysis_mode, execution_snapshot, input_snapshot, execution_fingerprint, input_fingerprint, research_question`

func NewAnalysisRepository(db *DB) *AnalysisRepository {
	return &AnalysisRepository{db: db}
}

func (r *AnalysisRepository) Create(ctx context.Context, a *domain.Analysis) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO analyses (`+analysisColumns+`)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.ProjectID, string(a.Status), a.CurrentStep, a.Progress, a.Error, a.Metrics,
		nullableTime(a.StartedAt), nullableTime(a.FinishedAt), formatTime(a.CreatedAt),
		nullableStringLiteral(a.Label), nullableStringLiteral(a.Note), nullableStringLiteral(string(a.SemanticAnalysisMode)),
		nullableStringLiteral(a.ExecutionSnapshot), nullableStringLiteral(a.InputSnapshot),
		nullableStringLiteral(a.ExecutionFingerprint), nullableStringLiteral(a.InputFingerprint), nullableStringLiteral(a.ResearchQuestion))
	return err
}

func (r *AnalysisRepository) Update(ctx context.Context, a *domain.Analysis) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE analyses SET status = ?, current_step = ?, progress = ?, error = ?, metrics = ?, started_at = ?, finished_at = ?,
		 label = ?, note = ?, semantic_analysis_mode = ?, execution_snapshot = ?, input_snapshot = ?,
		 execution_fingerprint = ?, input_fingerprint = ?, research_question = ?
		 WHERE id = ?`,
		string(a.Status), a.CurrentStep, a.Progress, a.Error, a.Metrics,
		nullableTime(a.StartedAt), nullableTime(a.FinishedAt),
		nullableStringLiteral(a.Label), nullableStringLiteral(a.Note), nullableStringLiteral(string(a.SemanticAnalysisMode)),
		nullableStringLiteral(a.ExecutionSnapshot), nullableStringLiteral(a.InputSnapshot),
		nullableStringLiteral(a.ExecutionFingerprint), nullableStringLiteral(a.InputFingerprint), nullableStringLiteral(a.ResearchQuestion), a.ID)
	return err
}

func (r *AnalysisRepository) Get(ctx context.Context, id string) (*domain.Analysis, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+analysisColumns+`
		 FROM analyses WHERE id = ?`, id)
	return scanAnalysis(row)
}

func (r *AnalysisRepository) ListByProject(ctx context.Context, projectID string) ([]*domain.Analysis, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+analysisColumns+`
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
		`SELECT `+analysisColumns+`
		 FROM analyses WHERE project_id = ? ORDER BY created_at DESC LIMIT 1`, projectID)
	return scanAnalysis(row)
}

func (r *AnalysisRepository) LatestCompletedByProject(ctx context.Context, projectID string) (*domain.Analysis, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+analysisColumns+`
		 FROM analyses WHERE project_id = ? AND status = 'completed'
		 ORDER BY finished_at DESC, created_at DESC LIMIT 1`, projectID)
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
	var label, note, semantic, execution, input, executionFP, inputFP, researchQuestion sql.NullString

	if err := s.Scan(&a.ID, &a.ProjectID, &status, &currentStep, &a.Progress, &errMsg, &metrics,
		&startedAt, &finishedAt, &createdAt,
		&label, &note, &semantic, &execution, &input, &executionFP, &inputFP, &researchQuestion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	a.Status = domain.AnalysisStatus(status)
	a.CurrentStep = currentStep.String
	a.Error = errMsg.String
	a.Metrics = metrics.String
	a.Label, a.Note = label.String, note.String
	a.SemanticAnalysisMode = domain.AnalysisMode(semantic.String)
	a.ExecutionSnapshot, a.InputSnapshot = execution.String, input.String
	a.ExecutionFingerprint, a.InputFingerprint = executionFP.String, inputFP.String
	a.ResearchQuestion = researchQuestion.String

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
