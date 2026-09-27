-- migration: 023_add_analysis_lifecycle
-- purpose: durable analysis lifecycle (#133). The analyses table becomes the
--   queue of record (workers claim queued rows with a conditional update),
--   and cancellation, interruption and retry are recorded instead of kept
--   in process memory.
-- safety: additive nullable columns. The legacy status values
--   (queued/running/completed/failed) are unchanged, so existing readers and
--   the Public Engine Contract keep working. failure_code NULL on legacy
--   rows means "not recorded", never a specific reason.
-- rollback: forward-only.

ALTER TABLE analyses ADD COLUMN failure_code TEXT;

ALTER TABLE analyses ADD COLUMN cancel_requested_at TEXT;

ALTER TABLE analyses ADD COLUMN retry_of TEXT;

CREATE UNIQUE INDEX idx_analyses_retry_of ON analyses(retry_of) WHERE retry_of IS NOT NULL;

CREATE INDEX idx_analyses_status_created ON analyses(status, created_at);
