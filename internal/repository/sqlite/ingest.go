package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

// IngestRepository persists large-CSV ingest jobs (#132) and their
// not-yet-visible documents.
type IngestRepository struct{ db *DB }

func NewIngestRepository(db *DB) *IngestRepository { return &IngestRepository{db: db} }

const ingestColumns = `id, project_id, kind, request_key, file_name, file_sha256, size_bytes,
	manifest, manifest_hash, state, stage, bytes_read, rows_read, rows_skipped,
	documents_created, error_count, error_examples, failure, created_at, updated_at, finished_at`

const liveIngestStates = `('QUEUED','VALIDATING','READY')`

func (r *IngestRepository) CreateIngest(ctx context.Context, job *domain.Ingest) error {
	examples, err := encodeErrorExamples(job.ErrorExamples)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO ingest_jobs (`+ingestColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.ProjectID, job.Kind, job.RequestKey, job.FileName, job.FileSHA256, job.SizeBytes,
		emptyAsNull(job.Manifest), job.ManifestHash, string(job.State), job.Stage,
		job.BytesRead, job.RowsRead, job.RowsSkipped, job.DocumentsCreated, job.ErrorCount, examples,
		emptyAsNull(job.Failure), formatTime(job.CreatedAt), formatTime(job.UpdatedAt), nullableTime(job.FinishedAt))
	if isUniqueViolation(err) {
		return repository.ErrConflict
	}
	return err
}

func (r *IngestRepository) FindLiveIngest(ctx context.Context, job *domain.Ingest) (*domain.Ingest, error) {
	return scanIngest(r.db.QueryRowContext(ctx, `SELECT `+ingestColumns+` FROM ingest_jobs
		WHERE project_id = ? AND kind = ? AND file_sha256 = ? AND manifest_hash = ? AND request_key = ?
		  AND state IN `+liveIngestStates,
		job.ProjectID, job.Kind, job.FileSHA256, job.ManifestHash, job.RequestKey))
}

func (r *IngestRepository) GetIngest(ctx context.Context, id string) (*domain.Ingest, error) {
	return scanIngest(r.db.QueryRowContext(ctx, `SELECT `+ingestColumns+` FROM ingest_jobs WHERE id = ?`, id))
}

func (r *IngestRepository) ListIngests(ctx context.Context, projectID string) ([]*domain.Ingest, error) {
	return r.queryIngests(ctx, `SELECT `+ingestColumns+` FROM ingest_jobs
		WHERE project_id = ? ORDER BY created_at DESC, id DESC`, projectID)
}

func (r *IngestRepository) ListIngestsByState(ctx context.Context, states ...domain.IngestState) ([]*domain.Ingest, error) {
	if len(states) == 0 {
		return nil, nil
	}
	args := make([]any, len(states))
	for i, s := range states {
		args[i] = string(s)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(states)), ",")
	return r.queryIngests(ctx, `SELECT `+ingestColumns+` FROM ingest_jobs
		WHERE state IN (`+placeholders+`) ORDER BY created_at ASC, id ASC`, args...)
}

func (r *IngestRepository) ListIngestIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM ingest_jobs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *IngestRepository) StagedBytes(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(size_bytes), 0) FROM ingest_jobs
		WHERE state IN ('QUEUED','VALIDATING')`).Scan(&total)
	return total, err
}

func (r *IngestRepository) ClaimNextIngest(ctx context.Context) (*domain.Ingest, error) {
	// A single connection serialises this read and the conditional update;
	// the WHERE state = 'QUEUED' guard still keeps a cancelled job unclaimed.
	var id string
	err := r.db.QueryRowContext(ctx, `SELECT id FROM ingest_jobs WHERE state = 'QUEUED'
		ORDER BY created_at ASC, id ASC LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	res, err := r.db.ExecContext(ctx, `UPDATE ingest_jobs SET state = 'VALIDATING', stage = ?, updated_at = ?
		WHERE id = ? AND state = 'QUEUED'`, domain.IngestStageParsing, formatTime(time.Now().UTC()), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, repository.ErrNotFound
	}
	return r.GetIngest(ctx, id)
}

func (r *IngestRepository) AppendIngestBatch(ctx context.Context, ingestID string, batch repository.IngestBatch) ([]int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var duplicates []int
	for i, doc := range batch.Documents {
		if key := batch.RowKeys[i]; key != "" {
			res, err := tx.ExecContext(ctx, `INSERT INTO ingest_row_keys (ingest_id, row_key) VALUES (?, ?)
				ON CONFLICT DO NOTHING`, ingestID, key)
			if err != nil {
				return nil, err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				duplicates = append(duplicates, i)
				continue
			}
		}
		meta, err := encodeMetadata(doc.Metadata)
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO documents (id, project_id, source, title, content, metadata, created_at, ingest_id)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			doc.ID, doc.ProjectID, string(doc.Source), doc.Title, doc.Content, meta, formatTime(doc.CreatedAt), ingestID); err != nil {
			return nil, err
		}
	}
	return duplicates, tx.Commit()
}

func (r *IngestRepository) SaveIngestProgress(ctx context.Context, job *domain.Ingest) error {
	examples, err := encodeErrorExamples(job.ErrorExamples)
	if err != nil {
		return err
	}
	job.UpdatedAt = time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `UPDATE ingest_jobs SET stage = ?, bytes_read = ?, rows_read = ?, rows_skipped = ?,
		documents_created = ?, error_count = ?, error_examples = ?, updated_at = ?
		WHERE id = ? AND state = 'VALIDATING'`,
		job.Stage, job.BytesRead, job.RowsRead, job.RowsSkipped, job.DocumentsCreated, job.ErrorCount, examples,
		formatTime(job.UpdatedAt), job.ID)
	return err
}

func (r *IngestRepository) FinishIngest(ctx context.Context, job *domain.Ingest) error {
	examples, err := encodeErrorExamples(job.ErrorExamples)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	job.UpdatedAt, job.FinishedAt = now, &now
	res, err := r.db.ExecContext(ctx, `UPDATE ingest_jobs SET state = ?, stage = ?, bytes_read = ?, rows_read = ?,
		rows_skipped = ?, documents_created = ?, error_count = ?, error_examples = ?, failure = ?,
		updated_at = ?, finished_at = ?
		WHERE id = ? AND state IN ('QUEUED','VALIDATING')`,
		string(job.State), job.Stage, job.BytesRead, job.RowsRead, job.RowsSkipped, job.DocumentsCreated,
		job.ErrorCount, examples, emptyAsNull(job.Failure), formatTime(now), formatTime(now), job.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return repository.ErrConflict
	}
	return nil
}

func (r *IngestRepository) RequeueIngest(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE ingest_jobs SET state = 'QUEUED', stage = ?, bytes_read = 0,
		rows_read = 0, rows_skipped = 0, documents_created = 0, error_count = 0, error_examples = NULL,
		updated_at = ? WHERE id = ? AND state = 'VALIDATING'`,
		domain.IngestStageStaged, formatTime(time.Now().UTC()), id)
	return err
}

func (r *IngestRepository) DiscardIngestRows(ctx context.Context, id string, limit int) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM documents WHERE rowid IN (
		SELECT d.rowid FROM documents d WHERE d.ingest_id = ?
		  AND NOT EXISTS (SELECT 1 FROM ingest_jobs j WHERE j.id = d.ingest_id AND j.state = 'READY')
		LIMIT ?)`, id, limit)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return n, nil
	}
	res, err = r.db.ExecContext(ctx, `DELETE FROM ingest_row_keys WHERE (ingest_id, row_key) IN (
		SELECT ingest_id, row_key FROM ingest_row_keys WHERE ingest_id = ? LIMIT ?)`, id, limit)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *IngestRepository) ListIngestDocuments(ctx context.Context, id string, limit int) ([]*domain.Document, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, project_id, source, title, content, metadata, created_at
		FROM documents WHERE ingest_id = ? ORDER BY rowid ASC LIMIT ?`, id, limit)
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

func (r *IngestRepository) queryIngests(ctx context.Context, query string, args ...any) ([]*domain.Ingest, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Ingest
	for rows.Next() {
		job, err := scanIngest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, rows.Err()
}

func scanIngest(s scanner) (*domain.Ingest, error) {
	var job domain.Ingest
	var state, createdAt, updatedAt string
	var manifest, examples, failure, finishedAt sql.NullString
	err := s.Scan(&job.ID, &job.ProjectID, &job.Kind, &job.RequestKey, &job.FileName, &job.FileSHA256, &job.SizeBytes,
		&manifest, &job.ManifestHash, &state, &job.Stage, &job.BytesRead, &job.RowsRead, &job.RowsSkipped,
		&job.DocumentsCreated, &job.ErrorCount, &examples, &failure, &createdAt, &updatedAt, &finishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	job.State = domain.IngestState(state)
	job.Manifest, job.Failure = manifest.String, failure.String
	if examples.Valid && examples.String != "" {
		if err := json.Unmarshal([]byte(examples.String), &job.ErrorExamples); err != nil {
			return nil, err
		}
	}
	if job.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	if job.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return nil, err
	}
	if finishedAt.Valid {
		t, err := parseTime(finishedAt.String)
		if err != nil {
			return nil, err
		}
		job.FinishedAt = &t
	}
	return &job, nil
}

func encodeErrorExamples(examples []domain.IngestRowError) (any, error) {
	if len(examples) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(examples)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func emptyAsNull(s string) any {
	if s == "" {
		return nil
	}
	return s
}
