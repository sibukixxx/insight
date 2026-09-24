package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

func TestTriageRepositoryKeepsPlanVersionsImmutableAndOrdered(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "triage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	if err := NewProjectRepository(db).Create(ctx, &domain.Project{ID: "p1", Name: "p", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	repo := NewTriageRepository(db)
	if err := repo.CreateProfile(ctx, &repository.StoredDatasetProfile{ProfileID: "prof1", ProjectID: "p1", Body: []byte(`{"a":1}`), CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetProfile(ctx, "prof1")
	if err != nil || string(got.Body) != `{"a":1}` {
		t.Fatalf("profile = %+v, %v", got, err)
	}
	for v, parent := range map[int]string{1: "", 2: "plan1"} {
		id := map[int]string{1: "plan1", 2: "plan2"}[v]
		if err := repo.CreatePlan(ctx, &repository.StoredSelectionPlan{PlanID: id, ProjectID: "p1", ProfileID: "prof1", Version: v, ParentPlanID: parent, Body: []byte(`{}`), CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	err = repo.CreatePlan(ctx, &repository.StoredSelectionPlan{PlanID: "plan3", ProjectID: "p1", ProfileID: "prof1", Version: 2, Body: []byte(`{}`), CreatedAt: now})
	if !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("duplicate version: %v", err)
	}
	plans, err := repo.ListPlans(ctx, "prof1")
	if err != nil || len(plans) != 2 || plans[0].PlanID != "plan1" || plans[1].ParentPlanID != "plan1" {
		t.Fatalf("plans = %+v, %v", plans, err)
	}
	if _, err := repo.GetPlan(ctx, "missing"); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("missing plan: %v", err)
	}
}
