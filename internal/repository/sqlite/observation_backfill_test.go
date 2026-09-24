package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

// applyMigrationsThrough creates a database at an older schema version so an
// upgrade can be exercised against pre-existing rows.
func applyMigrationsThrough(t *testing.T, path string, names []string) *sql.DB {
	t.Helper()
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for i, name := range names {
		content, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		tx, err := raw.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := execScript(context.Background(), tx, string(content)); err != nil {
			_ = tx.Rollback()
			t.Fatalf("apply %s: %v", name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`, i+1, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	return raw
}

func TestObservationBackfillAssignsOnlyUnambiguousAnalysisRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pre_snapshot.db")
	raw := applyMigrationsThrough(t, path, []string{
		"001_init.sql", "002_traces_and_quality.sql", "003_causal_reasoning.sql",
		"004_hypothesis_sets.sql", "005_dataset_source.sql", "006_research_loop.sql",
	})
	const at = "2026-01-01T00:00:00Z"
	for _, stmt := range []string{
		`INSERT INTO projects(id,name,created_at) VALUES ('p','legacy','` + at + `')`,
		`INSERT INTO documents(id,project_id,source,content,created_at) VALUES ('d','p','interview','text','` + at + `')`,
		`INSERT INTO analyses(id,project_id,status,created_at) VALUES ('a1','p','completed','` + at + `'), ('a2','p','completed','` + at + `')`,
		`INSERT INTO observations(id,document_id,quote,start_offset,end_offset,behavior,created_at) VALUES
			('o_pattern','d','t',0,1,'b','` + at + `'), ('o_evidence','d','t',0,1,'b','` + at + `'),
			('o_conflict','d','t',0,1,'b','` + at + `'), ('o_orphan','d','t',0,1,'b','` + at + `')`,
		`INSERT INTO patterns(id,project_id,analysis_id,title,created_at) VALUES ('pat1','p','a1','x','` + at + `'), ('pat2','p','a2','x','` + at + `')`,
		`INSERT INTO pattern_observations(pattern_id,observation_id) VALUES ('pat1','o_pattern'), ('pat1','o_conflict')`,
		`INSERT INTO insights(id,project_id,analysis_id,title,created_at) VALUES ('ins2','p','a2','x','` + at + `')`,
		`INSERT INTO evidence(id,insight_id,document_id,observation_id,quote,evidence_type,start_offset,end_offset) VALUES
			('e1','ins2','d','o_evidence','t','support',0,1), ('e2','ins2','d','o_conflict','t','support',0,1)`,
	} {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := Open(path)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	defer db.Close()
	got := map[string]string{}
	rows, err := db.Query(`SELECT id, COALESCE(analysis_id, '') FROM observations`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, analysisID string
		if err := rows.Scan(&id, &analysisID); err != nil {
			t.Fatal(err)
		}
		got[id] = analysisID
	}
	want := map[string]string{"o_pattern": "a1", "o_evidence": "a2", "o_conflict": "", "o_orphan": ""}
	for id, analysisID := range want {
		if got[id] != analysisID {
			t.Errorf("observation %s analysis_id = %q, want %q", id, got[id], analysisID)
		}
	}
}
