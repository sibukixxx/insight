-- migration: 016_create_data_triage_tables
-- purpose: persistence for AI-assisted Data Triage (Issue #92). Dataset
--          profiles and immutable, versioned Selection Plans. Plans are never
--          updated in place: a revision is a new row with a parent.
-- safety: new tables only. Existing data and routes are unaffected.
-- rollback: forward-only.

CREATE TABLE dataset_profiles (
    profile_id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE selection_plans (
    plan_id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    profile_id TEXT NOT NULL REFERENCES dataset_profiles(profile_id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    parent_plan_id TEXT,
    body TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (profile_id, version)
);

CREATE INDEX idx_selection_plans_profile ON selection_plans(profile_id, version);
