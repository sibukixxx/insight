package sqlite

import (
	"context"
	"testing"
	"time"

	"insight-lab/internal/domain"
)

func TestPatternRepositoryCreateAndList(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p, d := seedProjectAndDocument(t, db)
	observations := NewObservationRepository(db)
	patterns := NewPatternRepository(db)

	if err := observations.CreateBatch(ctx, []*domain.Observation{
		{ID: "obs_1", DocumentID: d.ID, Quote: "a", StartOffset: 0, EndOffset: 1, Behavior: "b1", CreatedAt: time.Now().UTC()},
		{ID: "obs_2", DocumentID: d.ID, Quote: "b", StartOffset: 1, EndOffset: 2, Behavior: "b2", CreatedAt: time.Now().UTC()},
	}); err != nil {
		t.Fatalf("seed observations: %v", err)
	}

	pattern := &domain.Pattern{
		ID: "pat_1", ProjectID: p.ID, Title: "繰り返しの確認行動", Description: "desc",
		ObservationIDs: []string{"obs_1", "obs_2"}, CreatedAt: time.Now().UTC(),
	}
	if err := patterns.CreateBatch(ctx, []*domain.Pattern{pattern}); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	list, err := patterns.ListByProject(ctx, p.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListByProject = %v, %v", list, err)
	}
	if list[0].Title != "繰り返しの確認行動" {
		t.Errorf("Title = %q", list[0].Title)
	}
	if len(list[0].ObservationIDs) != 2 {
		t.Fatalf("ObservationIDs = %v, want 2 entries", list[0].ObservationIDs)
	}
}

func TestPatternRepositoryLinkInsight(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p, d := seedProjectAndDocument(t, db)
	observations := NewObservationRepository(db)
	patterns := NewPatternRepository(db)
	insights := NewInsightRepository(db)

	if err := observations.CreateBatch(ctx, []*domain.Observation{
		{ID: "obs_1", DocumentID: d.ID, Quote: "a", StartOffset: 0, EndOffset: 1, Behavior: "b1", CreatedAt: time.Now().UTC()},
	}); err != nil {
		t.Fatalf("seed observation: %v", err)
	}
	if err := patterns.CreateBatch(ctx, []*domain.Pattern{
		{ID: "pat_1", ProjectID: p.ID, Title: "パターンA", ObservationIDs: []string{"obs_1"}, CreatedAt: time.Now().UTC()},
	}); err != nil {
		t.Fatalf("create pattern: %v", err)
	}
	insight := &domain.Insight{ID: "ins_1", ProjectID: p.ID, Title: "洞察A", Confidence: 0.5, CreatedAt: time.Now().UTC()}
	if err := insights.Create(ctx, insight); err != nil {
		t.Fatalf("create insight: %v", err)
	}

	if err := patterns.LinkInsight(ctx, insight.ID, []string{"pat_1"}); err != nil {
		t.Fatalf("LinkInsight: %v", err)
	}
	// Linking again (e.g. a retried write) must not error or duplicate.
	if err := patterns.LinkInsight(ctx, insight.ID, []string{"pat_1"}); err != nil {
		t.Fatalf("LinkInsight (idempotent retry): %v", err)
	}

	linked, err := patterns.ListByInsight(ctx, insight.ID)
	if err != nil || len(linked) != 1 {
		t.Fatalf("ListByInsight = %v, %v", linked, err)
	}
	if linked[0].ID != "pat_1" {
		t.Errorf("linked pattern ID = %q, want pat_1", linked[0].ID)
	}
}

func TestPatternRepositoryEmptyLinkIsNoop(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p, _ := seedProjectAndDocument(t, db)
	insights := NewInsightRepository(db)
	patterns := NewPatternRepository(db)

	insight := &domain.Insight{ID: "ins_1", ProjectID: p.ID, Title: "洞察", Confidence: 0.5, CreatedAt: time.Now().UTC()}
	if err := insights.Create(ctx, insight); err != nil {
		t.Fatalf("create insight: %v", err)
	}
	if err := patterns.LinkInsight(ctx, insight.ID, nil); err != nil {
		t.Fatalf("LinkInsight with no patterns: %v", err)
	}
	linked, err := patterns.ListByInsight(ctx, insight.ID)
	if err != nil || len(linked) != 0 {
		t.Fatalf("ListByInsight = %v, %v, want empty", linked, err)
	}
}
