-- migration: 022_create_ingest_tables
-- purpose: durable large-CSV ingestion (#132). An ingest job is the receipt
--   of one staged upload. Its documents are written in bounded batches with
--   documents.ingest_id set and stay invisible until the job is READY, so a
--   crash, failure or cancellation never leaves half-imported Evidence.
-- safety: additive. documents.ingest_id is a nullable column. Existing rows
--   stay NULL and remain visible exactly as before.
-- rollback: forward-only.

ALTER TABLE documents ADD COLUMN ingest_id TEXT;

CREATE INDEX idx_documents_ingest ON documents(ingest_id) WHERE ingest_id IS NOT NULL;

CREATE TABLE ingest_jobs (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    request_key TEXT NOT NULL DEFAULT '',
    file_name TEXT NOT NULL DEFAULT '',
    file_sha256 TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    manifest TEXT,
    manifest_hash TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL CHECK (state IN ('QUEUED','VALIDATING','READY','FAILED','CANCELLED')),
    stage TEXT NOT NULL,
    bytes_read INTEGER NOT NULL DEFAULT 0,
    rows_read INTEGER NOT NULL DEFAULT 0,
    rows_skipped INTEGER NOT NULL DEFAULT 0,
    documents_created INTEGER NOT NULL DEFAULT 0,
    error_count INTEGER NOT NULL DEFAULT 0,
    error_examples TEXT,
    failure TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    finished_at TEXT
);

CREATE UNIQUE INDEX idx_ingest_jobs_identity
    ON ingest_jobs(project_id, kind, file_sha256, manifest_hash, request_key)
    WHERE state IN ('QUEUED','VALIDATING','READY');

CREATE INDEX idx_ingest_jobs_project ON ingest_jobs(project_id, created_at);

CREATE INDEX idx_ingest_jobs_state ON ingest_jobs(state, created_at);

CREATE TABLE ingest_row_keys (
    ingest_id TEXT NOT NULL REFERENCES ingest_jobs(id) ON DELETE CASCADE,
    row_key TEXT NOT NULL,
    PRIMARY KEY (ingest_id, row_key)
) WITHOUT ROWID;
