-- migration: 024_add_project_research_question
-- purpose: keep the free-text research question a question-first project was
--   created from (#158), so the theme survives a reload even when no
--   evidence or model exists yet.
-- safety: additive column with a default. Existing projects get '' meaning
--   "no question recorded". No index, no lock beyond the ALTER.
-- rollback: forward-only.

ALTER TABLE projects ADD COLUMN research_question TEXT NOT NULL DEFAULT '';
