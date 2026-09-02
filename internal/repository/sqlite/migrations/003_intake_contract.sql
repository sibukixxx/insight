-- Intake contract: provenance (firsthand / secondhand), speaker spans,
-- pre-masking original text, and a per-project intake profile (speaker
-- role mapping, column mapping, mask dictionary). See
-- docs/detailed-design.md §24.

ALTER TABLE documents ADD COLUMN provenance TEXT NOT NULL DEFAULT 'firsthand';
ALTER TABLE documents ADD COLUMN spans TEXT;
ALTER TABLE documents ADD COLUMN raw_content TEXT;

ALTER TABLE projects ADD COLUMN intake_profile TEXT;
