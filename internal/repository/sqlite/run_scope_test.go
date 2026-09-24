package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

// seedTwoRuns creates two analyses of the same project, each with one
// insight and one pattern, so a run-scoped query can be checked for leakage.
func seedTwoRuns(t *testing.T, db *DB) (projectID string) {
	t.Helper()
	ctx := context.Background()
	p, _ := seedProjectAndDocument(t, db)
	analyses := NewAnalysisRepository(db)
	insights := NewInsightRepository(db)
	patterns := NewPatternRepository(db)
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i, id := range []string{"ana_1", "ana_2"} {
		created := base.Add(time.Duration(i) * time.Hour)
		finished := created.Add(time.Minute)
		if err := analyses.Create(ctx, &domain.Analysis{ID: id, ProjectID: p.ID, Status: domain.AnalysisCompleted, Progress: 100, FinishedAt: &finished, CreatedAt: created}); err != nil {
			t.Fatalf("create analysis: %v", err)
		}
		aid := id
		if err := insights.Create(ctx, &domain.Insight{ID: "ins_" + id, ProjectID: p.ID, AnalysisID: &aid, Title: "same title", CreatedAt: created}); err != nil {
			t.Fatalf("create insight: %v", err)
		}
		if err := patterns.CreateBatch(ctx, []*domain.Pattern{{ID: "pat_" + id, ProjectID: p.ID, AnalysisID: id, Title: "same pattern", CreatedAt: created}}); err != nil {
			t.Fatalf("create pattern: %v", err)
		}
	}
	return p.ID
}

func TestInsightRepositoryListByAnalysisReturnsOnlyThatRunsInsights(t *testing.T) {
	db := openTestDB(t)
	seedTwoRuns(t, db)
	got, err := NewInsightRepository(db).ListByAnalysis(context.Background(), "ana_2")
	if err != nil {
		t.Fatalf("ListByAnalysis: %v", err)
	}
	if len(got) != 1 || got[0].ID != "ins_ana_2" {
		t.Fatalf("ListByAnalysis(ana_2) = %v, want only ins_ana_2", ids(got))
	}
}

func TestPatternRepositoryListByAnalysisReturnsOnlyThatRunsPatterns(t *testing.T) {
	db := openTestDB(t)
	seedTwoRuns(t, db)
	got, err := NewPatternRepository(db).ListByAnalysis(context.Background(), "ana_1")
	if err != nil {
		t.Fatalf("ListByAnalysis: %v", err)
	}
	if len(got) != 1 || got[0].ID != "pat_ana_1" {
		t.Fatalf("ListByAnalysis(ana_1) returned %d patterns, want only pat_ana_1", len(got))
	}
}

func TestAnalysisRepositoryLatestCompletedByProjectSkipsNewerFailedAndRunning(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	projectID := seedTwoRuns(t, db)
	repo := NewAnalysisRepository(db)
	later := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	for _, a := range []*domain.Analysis{
		{ID: "ana_failed", ProjectID: projectID, Status: domain.AnalysisFailed, Error: "boom", FinishedAt: &later, CreatedAt: later},
		{ID: "ana_running", ProjectID: projectID, Status: domain.AnalysisRunning, CreatedAt: later.Add(time.Hour)},
	} {
		if err := repo.Create(ctx, a); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	got, err := repo.LatestCompletedByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("LatestCompletedByProject: %v", err)
	}
	if got.ID != "ana_2" {
		t.Fatalf("LatestCompletedByProject = %s, want ana_2", got.ID)
	}
}

func TestAnalysisRepositoryLatestCompletedByProjectReturnsNotFoundWithoutCompletedRun(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p, _ := seedProjectAndDocument(t, db)
	repo := NewAnalysisRepository(db)
	if err := repo.Create(ctx, &domain.Analysis{ID: "ana_q", ProjectID: p.ID, Status: domain.AnalysisQueued, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.LatestCompletedByProject(ctx, p.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func ids(list []*domain.Insight) []string {
	out := make([]string, 0, len(list))
	for _, i := range list {
		out = append(out, i.ID)
	}
	return out
}
