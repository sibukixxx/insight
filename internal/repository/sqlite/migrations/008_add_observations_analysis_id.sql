-- migration: 008_add_observations_analysis_id
-- purpose: make each observation belong to the analysis run that produced it,
--          like patterns and insights already do. Issue #82.
-- safety: additive nullable column. Existing rows stay NULL until 009.
-- rollback: forward-only.

ALTER TABLE observations ADD COLUMN analysis_id TEXT REFERENCES analyses(id) ON DELETE SET NULL;

CREATE INDEX idx_observations_analysis ON observations(analysis_id);
