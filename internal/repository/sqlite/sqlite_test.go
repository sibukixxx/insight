package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigrateIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	db1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	db1.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open (re-migrate): %v", err)
	}
	db2.Close()
}

func TestProjectRepositoryCRUD(t *testing.T) {
	db := openTestDB(t)
	repo := NewProjectRepository(db)
	ctx := context.Background()

	p := &domain.Project{ID: "proj_1", Name: "テストプロジェクト", CreatedAt: time.Now().UTC()}
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != p.Name {
		t.Errorf("Name = %q, want %q", got.Name, p.Name)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List returned %d projects, want 1", len(list))
	}

	if err := repo.Delete(ctx, p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, p.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Get after Delete = %v, want ErrNotFound", err)
	}
	if err := repo.Delete(ctx, "nonexistent"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Delete nonexistent = %v, want ErrNotFound", err)
	}
}

func TestDocumentRepositoryCRUDAndCascade(t *testing.T) {
	db := openTestDB(t)
	projects := NewProjectRepository(db)
	documents := NewDocumentRepository(db)
	ctx := context.Background()

	p := &domain.Project{ID: "proj_1", Name: "テスト", CreatedAt: time.Now().UTC()}
	if err := projects.Create(ctx, p); err != nil {
		t.Fatalf("Create project: %v", err)
	}

	d := &domain.Document{
		ID:        "doc_1",
		ProjectID: p.ID,
		Source:    domain.SourceInterview,
		Title:     "Interview #1",
		Content:   "設定を間違えたら怖いんですよね",
		Metadata:  map[string]string{"role": "経理担当"},
		CreatedAt: time.Now().UTC(),
	}
	if err := documents.Create(ctx, d); err != nil {
		t.Fatalf("Create document: %v", err)
	}

	got, err := documents.Get(ctx, d.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Content != d.Content {
		t.Errorf("Content = %q, want %q", got.Content, d.Content)
	}
	if got.Metadata["role"] != "経理担当" {
		t.Errorf("Metadata[role] = %q, want 経理担当", got.Metadata["role"])
	}

	batch := []*domain.Document{
		{ID: "doc_2", ProjectID: p.ID, Source: domain.SourceReview, Content: "a", CreatedAt: time.Now().UTC()},
		{ID: "doc_3", ProjectID: p.ID, Source: domain.SourceSupport, Content: "b", CreatedAt: time.Now().UTC()},
	}
	if err := documents.CreateBatch(ctx, batch); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	list, err := documents.ListByProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("ListByProject returned %d documents, want 3", len(list))
	}

	// ON DELETE CASCADE: deleting the project must remove its documents.
	if err := projects.Delete(ctx, p.ID); err != nil {
		t.Fatalf("Delete project: %v", err)
	}
	if _, err := documents.Get(ctx, d.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Get document after project delete = %v, want ErrNotFound", err)
	}
}

func TestDocumentRepositoryGetNotFound(t *testing.T) {
	db := openTestDB(t)
	documents := NewDocumentRepository(db)

	if _, err := documents.Get(context.Background(), "missing"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Get = %v, want ErrNotFound", err)
	}
}

// TestMigrateUpgradesExistingDatabase simulates a database created by a
// binary that only knew migration 001, then opens it with the current
// binary: 002 must apply on top (ALTER TABLE) and the new columns must be
// usable, with legacy pattern rows reading back as repetitions.
func TestMigrateUpgradesExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	initSQL, err := migrationsFS.ReadFile("migrations/001_init.sql")
	if err != nil {
		t.Fatalf("read 001: %v", err)
	}
	ctx := context.Background()
	if _, err := raw.ExecContext(ctx, `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	tx, err := raw.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := execScript(ctx, tx, string(initSQL)); err != nil {
		t.Fatalf("apply 001: %v", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (1, 'x')`); err != nil {
		t.Fatalf("record 001: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, stmt := range []string{
		`INSERT INTO projects (id, name, created_at) VALUES ('proj_1', 'legacy', '` + now + `')`,
		`INSERT INTO patterns (id, project_id, title, created_at) VALUES ('pat_legacy', 'proj_1', 'old pattern', '` + now + `')`,
		// The columns the 001-era repository always wrote (as "" when empty).
		`INSERT INTO insights (id, project_id, title, observation, stated_need, latent_need, jtbd, rationale, interpretation,
		   alternative_interpretation, product_opportunity, monetization_angle, confidence, created_at)
		 VALUES ('ins_legacy', 'proj_1', 'old insight', '', '', '', '', '', '', '', '', '', 0.5, '` + now + `')`,
	} {
		if _, err := raw.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed legacy row: %v", err)
		}
	}
	raw.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open (upgrade): %v", err)
	}
	defer db.Close()

	patterns, err := NewPatternRepository(db).ListByProject(ctx, "proj_1")
	if err != nil || len(patterns) != 1 {
		t.Fatalf("ListByProject = %v, %v", patterns, err)
	}
	if patterns[0].Kind != domain.PatternRepetition || patterns[0].IsTrace() {
		t.Errorf("legacy pattern should read as repetition, got %q", patterns[0].Kind)
	}
	insight, err := NewInsightRepository(db).Get(ctx, "ins_legacy")
	if err != nil {
		t.Fatalf("Get legacy insight: %v", err)
	}
	if insight.QualityFlags != nil || insight.Expectation != "" {
		t.Errorf("legacy insight should have empty new fields: %+v", insight)
	}
	if err := NewPatternRepository(db).CreateBatch(ctx, []*domain.Pattern{{
		ID: "pat_new", ProjectID: "proj_1", Kind: domain.PatternDeviation, Title: "new", Expectation: "e",
		DeviationType: domain.DeviationAbsence, CreatedAt: time.Now().UTC(),
	}}); err != nil {
		t.Fatalf("insert into upgraded schema: %v", err)
	}
}
