-- Expand the document source vocabulary for domain-neutral research.
//
-- The original schema reflected the project's customer-research roots.
-- Preserve every existing row and dependent observation/evidence relationship
-- while rebuilding the CHECK constraint with generic evidence categories.

CREATE TEMP TABLE _017_observations_backup AS
SELECT id, analysis_id, document_id, quote, start_offset, end_offset, behavior, topic, created_at, temporal_evidence
FROM observations;

CREATE TEMP TABLE _017_pattern_observations_backup AS
SELECT pattern_id, observation_id FROM pattern_observations;

CREATE TEMP TABLE _017_evidence_backup AS
SELECT id, insight_id, document_id, observation_id, quote, evidence_type, relevance_score, start_offset, end_offset, temporal_evidence
FROM evidence;

DROP TABLE pattern_observations;
DROP TABLE evidence;
DROP TABLE observations;
ALTER TABLE documents RENAME TO _017_documents_old;

CREATE TABLE documents (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source TEXT NOT NULL CHECK (source IN (
        'interview','review','support','sales','survey','job_posting','social_post','dataset',
        'document','report','paper','web','record','other'
    )),
    title TEXT,
    content TEXT NOT NULL,
    metadata TEXT,
    created_at TEXT NOT NULL
);
INSERT INTO documents (id, project_id, source, title, content, metadata, created_at)
SELECT id, project_id, source, title, content, metadata, created_at FROM _017_documents_old;
DROP TABLE _017_documents_old;
CREATE INDEX idx_documents_project ON documents(project_id);

CREATE TABLE observations (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    quote TEXT NOT NULL,
    start_offset INTEGER NOT NULL,
    end_offset INTEGER NOT NULL,
    behavior TEXT NOT NULL,
    topic TEXT,
    created_at TEXT NOT NULL,
    analysis_id TEXT REFERENCES analyses(id) ON DELETE SET NULL,
    temporal_evidence TEXT
);
INSERT INTO observations (id, analysis_id, document_id, quote, start_offset, end_offset, behavior, topic, created_at, temporal_evidence)
SELECT id, analysis_id, document_id, quote, start_offset, end_offset, behavior, topic, created_at, temporal_evidence
FROM _017_observations_backup;
DROP TABLE _017_observations_backup;
CREATE INDEX idx_observations_document ON observations(document_id);
CREATE INDEX idx_observations_analysis ON observations(analysis_id);

CREATE TABLE pattern_observations (
    pattern_id TEXT NOT NULL REFERENCES patterns(id) ON DELETE CASCADE,
    observation_id TEXT NOT NULL REFERENCES observations(id) ON DELETE CASCADE,
    PRIMARY KEY (pattern_id, observation_id)
);
INSERT INTO pattern_observations (pattern_id, observation_id)
SELECT pattern_id, observation_id FROM _017_pattern_observations_backup;
DROP TABLE _017_pattern_observations_backup;

CREATE TABLE evidence (
    id TEXT PRIMARY KEY,
    insight_id TEXT NOT NULL REFERENCES insights(id) ON DELETE CASCADE,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    observation_id TEXT REFERENCES observations(id) ON DELETE SET NULL,
    quote TEXT NOT NULL,
    evidence_type TEXT NOT NULL CHECK (evidence_type IN ('support','counter','neutral')),
    relevance_score REAL,
    start_offset INTEGER NOT NULL,
    end_offset INTEGER NOT NULL,
    temporal_evidence TEXT
);
INSERT INTO evidence (id, insight_id, document_id, observation_id, quote, evidence_type, relevance_score, start_offset, end_offset, temporal_evidence)
SELECT id, insight_id, document_id, observation_id, quote, evidence_type, relevance_score, start_offset, end_offset, temporal_evidence
FROM _017_evidence_backup;
DROP TABLE _017_evidence_backup;
CREATE INDEX idx_evidence_insight ON evidence(insight_id);
