CREATE TABLE research_runs (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    question TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_research_runs_project ON research_runs(project_id, created_at);

CREATE TABLE research_iterations (
    id TEXT PRIMARY KEY,
    research_run_id TEXT NOT NULL REFERENCES research_runs(id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL,
    payload TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE(research_run_id, sequence)
);
CREATE INDEX idx_research_iterations_run ON research_iterations(research_run_id, sequence);

CREATE TABLE human_evaluations (
    research_run_id TEXT NOT NULL REFERENCES research_runs(id) ON DELETE CASCADE,
    iteration_id TEXT NOT NULL REFERENCES research_iterations(id) ON DELETE CASCADE,
    payload TEXT NOT NULL,
    evaluated_at TEXT NOT NULL,
    PRIMARY KEY(research_run_id, iteration_id)
);
