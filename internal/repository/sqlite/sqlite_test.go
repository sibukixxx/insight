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

func TestDocumentRepositoryIntakeFieldsRoundTrip(t *testing.T) {
	db := openTestDB(t)
	repo := NewDocumentRepository(db)
	ctx := context.Background()
	p := &domain.Project{ID: "proj_1", Name: "p", CreatedAt: time.Now().UTC()}
	if err := NewProjectRepository(db).Create(ctx, p); err != nil {
		t.Fatalf("create project: %v", err)
	}

	withSpans := &domain.Document{
		ID: "doc_spans", ProjectID: p.ID, Source: domain.SourceInterview, Title: "t",
		Content: "面接官: Q\n回答者: A", RawContent: "面接官: Q\n回答者: A(山田)",
		Spans: []domain.Span{
			{Start: 0, End: 6, Speaker: "面接官", Role: domain.RoleInterviewer},
			{Start: 7, End: 13, Speaker: "回答者", Role: domain.RoleCustomer},
		},
		Metadata: map[string]string{domain.MetaRole: "経理"}, CreatedAt: time.Now().UTC(),
	}
	// Provenance left empty: the repository must fall back to the
	// source's default (sales -> secondhand).
	salesNote := &domain.Document{ID: "doc_sales", ProjectID: p.ID, Source: domain.SourceSales, Content: "営業メモ", CreatedAt: time.Now().UTC()}
	if err := repo.CreateBatch(ctx, []*domain.Document{withSpans, salesNote}); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	got, err := repo.Get(ctx, "doc_spans")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Provenance != domain.ProvenanceFirsthand {
		t.Errorf("Provenance = %q, want firsthand", got.Provenance)
	}
	if got.RawContent != withSpans.RawContent {
		t.Errorf("RawContent did not round-trip: %q", got.RawContent)
	}
	if len(got.Spans) != 2 || got.Spans[1].Role != domain.RoleCustomer || got.Spans[1].Speaker != "回答者" || got.Spans[1].Start != 7 {
		t.Errorf("Spans did not round-trip: %+v", got.Spans)
	}

	got, err = repo.Get(ctx, "doc_sales")
	if err != nil {
		t.Fatalf("Get sales: %v", err)
	}
	if got.Provenance != domain.ProvenanceSecondhand || !got.IsSecondhand() {
		t.Errorf("sales note should default to secondhand, got %q", got.Provenance)
	}
	if got.RawContent != "" || got.Spans != nil {
		t.Errorf("plain document should have empty intake fields: %+v", got)
	}
}

func TestProjectRepositoryIntakeProfile(t *testing.T) {
	db := openTestDB(t)
	repo := NewProjectRepository(db)
	ctx := context.Background()
	p := &domain.Project{ID: "proj_1", Name: "p", CreatedAt: time.Now().UTC()}
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.IntakeProfile.SpeakerRoles != nil || got.IntakeProfile.MaskTerms != nil {
		t.Errorf("fresh project should have an empty profile: %+v", got.IntakeProfile)
	}

	profile := domain.IntakeProfile{
		SpeakerRoles:  map[string]domain.SpeakerRole{"田中": domain.RoleInterviewer, "佐藤": domain.RoleCustomer},
		MaskTerms:     []string{"株式会社サンプル"},
		ColumnMapping: &domain.ColumnMapping{ContentColumn: "自由記述", DefaultSource: domain.SourceSurvey, MetadataColumns: map[string]string{"役職": domain.MetaRole}},
	}
	if err := repo.UpdateIntakeProfile(ctx, p.ID, profile); err != nil {
		t.Fatalf("UpdateIntakeProfile: %v", err)
	}
	got, err = repo.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got.IntakeProfile.SpeakerRoles["佐藤"] != domain.RoleCustomer || len(got.IntakeProfile.MaskTerms) != 1 ||
		got.IntakeProfile.ColumnMapping == nil || got.IntakeProfile.ColumnMapping.MetadataColumns["役職"] != domain.MetaRole {
		t.Errorf("profile did not round-trip: %+v", got.IntakeProfile)
	}
	if err := repo.UpdateIntakeProfile(ctx, "missing", profile); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("update of missing project = %v, want ErrNotFound", err)
	}
}
