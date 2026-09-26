-- migration: 020_create_engine_state
-- purpose: persist an opaque engine-state identity (#116) so Public Engine
--   consumers can tell a restarted engine from a fresh or restored database.
--   The row lives inside the database, so it survives restarts, is new for a
--   fresh database and travels with a backup/restore of this file.
-- safety: additive. created_at is the earliest recorded migration, i.e. the
--   original initialization time of an upgraded database.
-- rollback: forward-only. Never rewrite state_id in place.

CREATE TABLE engine_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    state_id TEXT NOT NULL,
    created_at TEXT NOT NULL
);

INSERT INTO engine_state (id, state_id, created_at)
SELECT 1, lower(hex(randomblob(16))),
    COALESCE((SELECT MIN(applied_at) FROM schema_migrations), strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
