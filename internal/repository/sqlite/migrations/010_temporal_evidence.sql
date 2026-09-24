-- Additive metadata only: retain the Analysis ownership and legacy backfill from 008/009.
ALTER TABLE observations ADD COLUMN temporal_evidence TEXT;
ALTER TABLE evidence ADD COLUMN temporal_evidence TEXT;
