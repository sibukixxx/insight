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
	 label, note, semantic_analysis_mode, execution_snapshot, input_snapshot, execution_fingerprint, input_fingerprint, research_question, reasoning_profile, output_locale,
	 failure_code, cancel_requested_at, retry_of`

func NewAnalysisRepository(db *DB) *AnalysisRepository {
	return &AnalysisRepository{db: db}
}

func (r *AnalysisRepository) Create(ctx context.Context, a *domain.Analysis) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO analyses (`+analysisColumns+`)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.ProjectID, string(a.Status), a.CurrentStep, a.Progress, a.Error, a.Metrics,
		nullableTime(a.StartedAt), nullableTime(a.FinishedAt), formatTime(a.CreatedAt),
		nullableStringLiteral(a.Label), nullableStringLiteral(a.Note), nullableStringLiteral(string(a.SemanticAnalysisMode)),
		nullableStringLiteral(a.ExecutionSnapshot), nullableStringLiteral(a.InputSnapshot),
		nullableStringLiteral(a.ExecutionFingerprint), nullableStringLiteral(a.InputFingerprint), nullableStringLiteral(a.ResearchQuestion), nullableStringLiteral(string(a.ReasoningProfile)), nullableStringLiteral(string(a.OutputLocale)),
		nullableStringLiteral(string(a.FailureCode)), nullableTime(a.CancelRequestedAt), nullableStringLiteral(a.RetryOf))
	if isUniqueViolation(err) {
		return repository.ErrConflict
	}
	return err
}

// Update stores the progress of a run. It never writes the lifecycle
// columns (failure code, cancel request, retry link); terminal transitions
// go through FinishRunning or CancelQueued.
func (r *AnalysisRepository) Update(ctx context.Context, a *domain.Analysis) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE analyses SET status = ?, current_step = ?, progress = ?, error = ?, metrics = ?, started_at = ?, finished_at = ?,
		 label = ?, note = ?, semantic_analysis_mode = ?, execution_snapshot = ?, input_snapshot = ?,
		 execution_fingerprint = ?, input_fingerprint = ?, research_question = ?, reasoning_profile = ?, output_locale = ?
		 WHERE id = ?`,
		string(a.Status), a.CurrentStep, a.Progress, a.Error, a.Metrics,
		nullableTime(a.StartedAt), nullableTime(a.FinishedAt),
		nullableStringLiteral(a.Label), nullableStringLiteral(a.Note), nullableStringLiteral(string(a.SemanticAnalysisMode)),
		nullableStringLiteral(a.ExecutionSnapshot), nullableStringLiteral(a.InputSnapshot),
		nullableStringLiteral(a.ExecutionFingerprint), nullableStringLiteral(a.InputFingerprint), nullableStringLiteral(a.ResearchQuestion), nullableStringLiteral(string(a.ReasoningProfile)), nullableStringLiteral(string(a.OutputLocale)), a.ID)
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

func (r *AnalysisRepository) CountQueued(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM analyses WHERE status = 'queued'`).Scan(&n)
	return n, err
}

func (r *AnalysisRepository) ClaimNextQueued(ctx context.Context) (*domain.Analysis, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `SELECT id FROM analyses WHERE status = 'queued'
		ORDER BY created_at ASC, id ASC LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	res, err := r.db.ExecContext(ctx, `UPDATE analyses SET status = 'running', current_step = 'starting', started_at = ?
		WHERE id = ? AND status = 'queued'`, formatTime(time.Now().UTC()), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, repository.ErrNotFound
	}
	return r.Get(ctx, id)
}

func (r *AnalysisRepository) CancelQueued(ctx context.Context, id, reason string) (bool, error) {
	now := formatTime(time.Now().UTC())
	res, err := r.db.ExecContext(ctx, `UPDATE analyses SET status = 'failed', failure_code = ?, error = ?,
		cancel_requested_at = ?, finished_at = ? WHERE id = ? AND status = 'queued'`,
		string(domain.FailureCancelled), reason, now, now, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (r *AnalysisRepository) RequestCancel(ctx context.Context, id string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE analyses SET cancel_requested_at = ?
		WHERE id = ? AND status = 'running' AND cancel_requested_at IS NULL`, formatTime(time.Now().UTC()), id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (r *AnalysisRepository) FinishRunning(ctx context.Context, a *domain.Analysis) (bool, error) {
	guard := ""
	if a.Status == domain.AnalysisCompleted {
		guard = " AND cancel_requested_at IS NULL"
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE analyses SET status = ?, failure_code = ?, current_step = ?, progress = ?, error = ?, metrics = ?,
		 finished_at = ?, execution_snapshot = ?, input_snapshot = ?, input_fingerprint = ?
		 WHERE id = ? AND status = 'running'`+guard,
		string(a.Status), nullableStringLiteral(string(a.FailureCode)), a.CurrentStep, a.Progress, a.Error, a.Metrics,
		nullableTime(a.FinishedAt), nullableStringLiteral(a.ExecutionSnapshot), nullableStringLiteral(a.InputSnapshot),
		nullableStringLiteral(a.InputFingerprint), a.ID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (r *AnalysisRepository) FindRetry(ctx context.Context, id string) (*domain.Analysis, error) {
	return scanAnalysis(r.db.QueryRowContext(ctx, `SELECT `+analysisColumns+` FROM analyses WHERE retry_of = ?`, id))
}

func (r *AnalysisRepository) FailInterrupted(ctx context.Context) (int, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE analyses SET status = 'failed', failure_code = ?, error = ?, finished_at = ?
		 WHERE status = 'running'`, string(domain.FailureInterrupted), "interrupted", formatTime(time.Now().UTC()))
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
	var label, note, semantic, execution, input, executionFP, inputFP, researchQuestion, reasoningProfile, outputLocale sql.NullString
	var failureCode, cancelRequestedAt, retryOf sql.NullString

	if err := s.Scan(&a.ID, &a.ProjectID, &status, &currentStep, &a.Progress, &errMsg, &metrics,
		&startedAt, &finishedAt, &createdAt,
		&label, &note, &semantic, &execution, &input, &executionFP, &inputFP, &researchQuestion, &reasoningProfile, &outputLocale,
		&failureCode, &cancelRequestedAt, &retryOf); err != nil {
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
	a.ReasoningProfile = domain.ReasoningProfile(reasoningProfile.String)
	a.OutputLocale = domain.OutputLocale(outputLocale.String)
	a.FailureCode = domain.AnalysisFailureCode(failureCode.String)
	a.RetryOf = retryOf.String
	if cancelRequestedAt.Valid {
		t, err := parseTime(cancelRequestedAt.String)
		if err != nil {
			return nil, err
		}
		a.CancelRequestedAt = &t
	}

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
