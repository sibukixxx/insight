-- migration: 007_analysis_run_snapshot
-- purpose: record per analysis run what it read (input snapshot) and which
--          engine, rules, prompts and model ran it (execution snapshot). Issue #82.
-- safety: additive nullable columns only. Runs recorded before this migration
--         keep NULL, which the application reports as "not recorded".
-- rollback: forward-only. Retract with a later migration.

ALTER TABLE analyses ADD COLUMN label TEXT;
ALTER TABLE analyses ADD COLUMN note TEXT;
ALTER TABLE analyses ADD COLUMN semantic_analysis_mode TEXT;
ALTER TABLE analyses ADD COLUMN execution_snapshot TEXT;
ALTER TABLE analyses ADD COLUMN input_snapshot TEXT;
ALTER TABLE analyses ADD COLUMN execution_fingerprint TEXT;
ALTER TABLE analyses ADD COLUMN input_fingerprint TEXT;

CREATE INDEX idx_analyses_project_status_finished ON analyses(project_id, status, finished_at);
CREATE INDEX idx_analyses_project_fingerprints ON analyses(project_id, input_fingerprint, execution_fingerprint);
CREATE INDEX idx_insights_analysis ON insights(analysis_id);
CREATE INDEX idx_patterns_analysis ON patterns(analysis_id);
