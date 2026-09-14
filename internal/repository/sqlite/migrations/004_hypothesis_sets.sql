ALTER TABLE insights ADD COLUMN hypothesis_set_id TEXT;
ALTER TABLE insights ADD COLUMN hypothesis_role TEXT NOT NULL DEFAULT 'PRIMARY';
CREATE INDEX idx_insights_hypothesis_set ON insights(hypothesis_set_id);
