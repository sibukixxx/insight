-- Persist the optional domain-neutral research question as semantic analysis input.
-- Empty/NULL means open-ended discovery and keeps older runs compatible.

ALTER TABLE analyses ADD COLUMN research_question TEXT;
