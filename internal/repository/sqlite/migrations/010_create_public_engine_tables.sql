-- migration: 010_create_public_engine_tables
-- purpose: persistence for the Public Engine Contract v1 (Issue #59). Opaque
--          external subjects map to projects, and the first successful
--          response to each idempotency key is kept for replay.
-- safety: new tables only. Existing projects and routes are unaffected.
-- rollback: forward-only.

CREATE TABLE public_subjects (
    project_id TEXT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    namespace TEXT NOT NULL,
    external_id TEXT NOT NULL,
    subject_type TEXT,
    metadata TEXT,
    created_at TEXT NOT NULL,
    UNIQUE (namespace, external_id)
);

CREATE TABLE public_idempotent_responses (
    idempotency_key TEXT PRIMARY KEY,
    request_hash TEXT NOT NULL,
    status_code INTEGER NOT NULL,
    body TEXT NOT NULL,
    created_at TEXT NOT NULL
);
