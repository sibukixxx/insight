package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"insight-lab/internal/domain"
)

func TestResearchRepositorySurvivesReloadAndPreservesIterations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "research.db")
	ctx := context.Background()
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO projects (id, name, created_at) VALUES ('p1','policy',?)`, formatTime(now)); err != nil {
		t.Fatal(err)
	}
	repo := NewResearchRepository(db)
	run := &domain.ResearchRun{ID: "run1", ProjectID: "p1", Question: "did treatment cause the increase?", CreatedAt: now, Iterations: []domain.ResearchIteration{{ID: "it1", Sequence: 1, Question: "did treatment cause the increase?", CreatedAt: now}}}
	if err := repo.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	second := domain.ResearchIteration{ID: "it2", Sequence: 2, Question: run.Question, AddedEvidence: []string{"comparison group"}, CreatedAt: now.Add(time.Hour)}
	if err := repo.AppendResearchIteration(ctx, run.ID, second); err != nil {
		t.Fatal(err)
	}
	evaluation := &domain.HumanEvaluation{ResearchRunID: run.ID, IterationID: second.ID, Novelty: domain.NoveltyPartiallyNew, ObservationGrounding: 4, SurpriseUsefulness: 4, HypothesisDiversity: 5, CounterEvidenceQuality: 3, MissingEvidenceQuality: 4, IdentificationHonesty: 5, NextDataUsefulness: 4, OverallUsefulness: 4, EvaluatedAt: now}
	if err := repo.SaveHumanEvaluation(ctx, evaluation); err != nil {
		t.Fatal(err)
	}
	db.Close()

	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo = NewResearchRepository(db)
	got, err := repo.GetResearchRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Iterations) != 2 || got.Iterations[0].ID != "it1" || got.Iterations[1].ID != "it2" {
		t.Fatalf("append-only history lost: %+v", got.Iterations)
	}
	if got.Iterations[0].AddedEvidence != nil {
		t.Fatal("appending changed the first iteration")
	}
	gotEval, err := repo.GetHumanEvaluation(ctx, run.ID, second.ID)
	if err != nil || gotEval.Novelty != domain.NoveltyPartiallyNew {
		t.Fatalf("evaluation did not survive reload: %+v %v", gotEval, err)
	}
}

func TestResearchRepositoryRejectsDuplicateSequence(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := db.ExecContext(ctx, `INSERT INTO projects (id, name, created_at) VALUES ('p1','p',?)`, formatTime(now)); err != nil {
		t.Fatal(err)
	}
	repo := NewResearchRepository(db)
	if err := repo.CreateRun(ctx, &domain.ResearchRun{ID: "r1", ProjectID: "p1", Question: "q", CreatedAt: now, Iterations: []domain.ResearchIteration{{ID: "i1", Sequence: 1, CreatedAt: now}}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.AppendResearchIteration(ctx, "r1", domain.ResearchIteration{ID: "i2", Sequence: 1, CreatedAt: now}); err == nil {
		t.Fatal("duplicate sequence must not overwrite iteration 1")
	}
}
