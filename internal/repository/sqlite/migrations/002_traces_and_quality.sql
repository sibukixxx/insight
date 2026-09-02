-- Traces of desire (deviation-from-expectation patterns) and app-side
-- insight quality flags. See docs/detailed-design.md §23.

ALTER TABLE patterns ADD COLUMN kind TEXT NOT NULL DEFAULT 'repetition';
ALTER TABLE patterns ADD COLUMN expectation TEXT;
ALTER TABLE patterns ADD COLUMN deviation_type TEXT;

ALTER TABLE insights ADD COLUMN expectation TEXT;
ALTER TABLE insights ADD COLUMN surprising_fact TEXT;
ALTER TABLE insights ADD COLUMN quality_flags TEXT;
