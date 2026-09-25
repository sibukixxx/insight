package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"insight-lab/internal/domain"
)

func TestFreshDatabaseAcceptsGenericEvidenceSources(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "generic-sources.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	projects := NewProjectRepository(db)
	documents := NewDocumentRepository(db)
	project := &domain.Project{ID: "p-generic", Name: "Generic research", CreatedAt: time.Now().UTC()}
	if err := projects.Create(ctx, project); err != nil {
		t.Fatal(err)
	}

	for i, source := range []domain.SourceType{
		domain.SourceDocument, domain.SourceReport, domain.SourcePaper,
		domain.SourceWeb, domain.SourceRecord, domain.SourceOther,
	} {
		doc := &domain.Document{
			ID: "d-" + string(rune('a'+i)), ProjectID: project.ID, Source: source,
			Title: string(source), Content: "Generic evidence.", CreatedAt: time.Now().UTC(),
		}
		if err := documents.Create(ctx, doc); err != nil {
			t.Fatalf("source %s: %v", source, err)
		}
		got, err := documents.Get(ctx, doc.ID)
		if err != nil {
			t.Fatalf("read source %s: %v", source, err)
		}
		if got.Source != source {
			t.Fatalf("source round trip = %q, want %q", got.Source, source)
		}
	}
}
