-- SQLite cannot alter a CHECK constraint in place. Preserve the complete
-- document-dependent graph while rebuilding documents with the dataset source.
CREATE TEMP TABLE _observations_backup AS SELECT * FROM observations;
CREATE TEMP TABLE _pattern_observations_backup AS SELECT * FROM pattern_observations;
CREATE TEMP TABLE _evidence_backup AS SELECT * FROM evidence;

DROP TABLE pattern_observations;
DROP TABLE evidence;
DROP TABLE observations;
ALTER TABLE documents RENAME TO _documents_old;

CREATE TABLE documents (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source TEXT NOT NULL CHECK (source IN ('interview','review','support','sales','survey','job_posting','social_post','dataset')),
    title TEXT,
    content TEXT NOT NULL,
    metadata TEXT,
    created_at TEXT NOT NULL
);
INSERT INTO documents SELECT * FROM _documents_old;
DROP TABLE _documents_old;
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
INSERT INTO observations SELECT * FROM _observations_backup;
DROP TABLE _observations_backup;
CREATE INDEX idx_observations_document ON observations(document_id);

CREATE TABLE pattern_observations (
    pattern_id TEXT NOT NULL REFERENCES patterns(id) ON DELETE CASCADE,
    observation_id TEXT NOT NULL REFERENCES observations(id) ON DELETE CASCADE,
    PRIMARY KEY (pattern_id, observation_id)
);
INSERT INTO pattern_observations SELECT * FROM _pattern_observations_backup;
DROP TABLE _pattern_observations_backup;

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
INSERT INTO evidence SELECT * FROM _evidence_backup;
DROP TABLE _evidence_backup;
CREATE INDEX idx_evidence_insight ON evidence(insight_id);
