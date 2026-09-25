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


func TestAnalysisResearchQuestionRoundTrips(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "analysis-question.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	project := &domain.Project{ID: "p-question", Name: "Question research", CreatedAt: time.Now().UTC()}
	if err := NewProjectRepository(db).Create(ctx, project); err != nil {
		t.Fatal(err)
	}
	repo := NewAnalysisRepository(db)
	analysis := &domain.Analysis{
		ID: "a-question", ProjectID: project.ID, Status: domain.AnalysisQueued,
		ResearchQuestion: "Which explanation best accounts for the observed change?",
		CreatedAt: time.Now().UTC(),
	}
	if err := repo.Create(ctx, analysis); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, analysis.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ResearchQuestion != analysis.ResearchQuestion {
		t.Fatalf("research question = %q, want %q", got.ResearchQuestion, analysis.ResearchQuestion)
	}
}
