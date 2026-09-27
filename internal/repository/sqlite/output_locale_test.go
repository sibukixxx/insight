package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"insight-lab/internal/domain"
)

// The requested output locale is generation context (#125): it must survive
// persistence, and a run that did not request one must stay unset.
func TestAnalysisOutputLocaleRoundTripsWhenRequestedAndStaysEmptyWhenOmitted(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "analysis-locale.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := NewProjectRepository(db).Create(ctx, &domain.Project{ID: "p-locale", Name: "Locale", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	repo := NewAnalysisRepository(db)
	for id, locale := range map[string]domain.OutputLocale{"a-ja": domain.OutputLocaleJaJP, "a-omitted": ""} {
		a := &domain.Analysis{ID: id, ProjectID: "p-locale", Status: domain.AnalysisQueued, OutputLocale: locale, CreatedAt: time.Now().UTC()}
		if err := repo.Create(ctx, a); err != nil {
			t.Fatal(err)
		}
		a.Status = domain.AnalysisCompleted
		if err := repo.Update(ctx, a); err != nil {
			t.Fatal(err)
		}
		got, err := repo.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if got.OutputLocale != locale {
			t.Fatalf("%s: output locale = %q, want %q", id, got.OutputLocale, locale)
		}
	}
}
