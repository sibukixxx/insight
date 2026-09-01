CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE documents (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source TEXT NOT NULL CHECK (source IN ('interview','review','support','sales','survey','job_posting','social_post')),
    title TEXT,
    content TEXT NOT NULL,
    metadata TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_documents_project ON documents(project_id);

CREATE TABLE observations (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    quote TEXT NOT NULL,
    start_offset INTEGER NOT NULL,
    end_offset INTEGER NOT NULL,
    behavior TEXT NOT NULL,
    topic TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_observations_document ON observations(document_id);

CREATE TABLE patterns (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    analysis_id TEXT REFERENCES analyses(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    description TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_patterns_project ON patterns(project_id);

CREATE TABLE pattern_observations (
    pattern_id TEXT NOT NULL REFERENCES patterns(id) ON DELETE CASCADE,
    observation_id TEXT NOT NULL REFERENCES observations(id) ON DELETE CASCADE,
    PRIMARY KEY (pattern_id, observation_id)
);

CREATE TABLE analyses (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('queued','running','completed','failed')),
    current_step TEXT,
    progress INTEGER NOT NULL DEFAULT 0,
    error TEXT,
    metrics TEXT,
    started_at TEXT,
    finished_at TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_analyses_project ON analyses(project_id);

CREATE TABLE insights (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    analysis_id TEXT REFERENCES analyses(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    observation TEXT,
    stated_need TEXT,
    latent_need TEXT,
    jtbd TEXT,
    rationale TEXT,
    interpretation TEXT,
    alternative_interpretation TEXT,
    product_opportunity TEXT,
    monetization_angle TEXT,
    confidence REAL,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_insights_project ON insights(project_id);

CREATE TABLE insight_patterns (
    insight_id TEXT NOT NULL REFERENCES insights(id) ON DELETE CASCADE,
    pattern_id TEXT NOT NULL REFERENCES patterns(id) ON DELETE CASCADE,
    PRIMARY KEY (insight_id, pattern_id)
);

CREATE TABLE evidence (
    id TEXT PRIMARY KEY,
    insight_id TEXT NOT NULL REFERENCES insights(id) ON DELETE CASCADE,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    observation_id TEXT REFERENCES observations(id) ON DELETE SET NULL,
    quote TEXT NOT NULL,
    evidence_type TEXT NOT NULL CHECK (evidence_type IN ('support','counter','neutral')),
    relevance_score REAL,
    start_offset INTEGER NOT NULL,
    end_offset INTEGER NOT NULL
);
CREATE INDEX idx_evidence_insight ON evidence(insight_id);
