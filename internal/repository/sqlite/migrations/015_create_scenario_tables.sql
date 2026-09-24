-- migration: 015_create_scenario_tables
-- purpose: append-only persistence for multi-scenario prospective analysis
--          (Issue #66): scenario set versions and their evaluations.
-- safety: new tables only. Research runs and iterations are unchanged.
-- rollback: forward-only.

CREATE TABLE scenario_sets (
    id TEXT PRIMARY KEY,
    research_run_id TEXT NOT NULL REFERENCES research_runs(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    payload TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (research_run_id, version)
);

CREATE TABLE scenario_evaluations (
    id TEXT PRIMARY KEY,
    research_run_id TEXT NOT NULL REFERENCES research_runs(id) ON DELETE CASCADE,
    scenario_set_id TEXT NOT NULL REFERENCES scenario_sets(id) ON DELETE CASCADE,
    iteration_id TEXT,
    payload TEXT NOT NULL,
    evaluated_at TEXT NOT NULL
);
CREATE INDEX idx_scenario_evaluations_run ON scenario_evaluations(research_run_id, evaluated_at);
