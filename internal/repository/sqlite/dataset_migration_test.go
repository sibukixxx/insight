package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"insight-lab/internal/domain"
)

func TestDatasetSourceMigrationPreservesExistingDocumentsAndObservations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pre_dataset.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for version := 1; version <= 4; version++ {
		name := map[int]string{1: "001_init.sql", 2: "002_traces_and_quality.sql", 3: "003_causal_reasoning.sql", 4: "004_hypothesis_sets.sql"}[version]
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
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`, version, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := raw.Exec(`INSERT INTO projects(id,name,created_at) VALUES ('p','existing','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`INSERT INTO documents(id,project_id,source,title,content,created_at) VALUES ('d','p','interview','before','original','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`INSERT INTO observations(id,document_id,quote,start_offset,end_offset,behavior,created_at) VALUES ('o','d','original',0,8,'said original','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := Open(path)
	if err != nil {
		t.Fatalf("open and migrate: %v", err)
	}
	defer db.Close()
	for table, want := range map[string]int{"documents": 1, "observations": 1} {
		var got int
		if err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&got); err != nil || got != want {
			t.Fatalf("%s count = %d, %v; want %d", table, got, err, want)
		}
	}
	repo := NewDocumentRepository(db)
	err = repo.Create(context.Background(), &domain.Document{
		ID: "dataset", ProjectID: "p", Source: domain.SourceDataset,
		Title: "aggregate", Content: "record_count=1", CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("dataset source should be accepted after migration: %v", err)
	}
}
