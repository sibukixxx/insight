-- migration: 021_add_analysis_output_locale
-- purpose: persist the output locale a caller explicitly requested for
--   model-generated text (#125) as generation context of the analysis run.
-- safety: additive nullable column. Existing rows stay NULL, meaning "not
--   requested" (generated text follows the source language), never a locale.
-- rollback: forward-only.

ALTER TABLE analyses ADD COLUMN output_locale TEXT;
