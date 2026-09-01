package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

func seedProjectAndDocument(t *testing.T, db *DB) (*domain.Project, *domain.Document) {
	t.Helper()
	ctx := context.Background()
	projects := NewProjectRepository(db)
	documents := NewDocumentRepository(db)

	p := &domain.Project{ID: "proj_1", Name: "テスト", CreatedAt: time.Now().UTC()}
	if err := projects.Create(ctx, p); err != nil {
		t.Fatalf("create project: %v", err)
	}
	d := &domain.Document{
		ID: "doc_1", ProjectID: p.ID, Source: domain.SourceInterview,
		Title: "Interview #1", Content: "設定を間違えたら怖いんですよね", CreatedAt: time.Now().UTC(),
	}
	if err := documents.Create(ctx, d); err != nil {
		t.Fatalf("create document: %v", err)
	}
	return p, d
}

func TestObservationRepository(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p, d := seedProjectAndDocument(t, db)
	repo := NewObservationRepository(db)

	obs := []*domain.Observation{
		{ID: "obs_1", DocumentID: d.ID, Quote: "設定を間違えたら怖い", StartOffset: 0, EndOffset: 10, Behavior: "設定ミスを恐れている", Topic: "anxiety", CreatedAt: time.Now().UTC()},
	}
	if err := repo.CreateBatch(ctx, obs); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	got, err := repo.Get(ctx, "obs_1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Behavior != obs[0].Behavior {
		t.Errorf("Behavior = %q, want %q", got.Behavior, obs[0].Behavior)
	}

	byDoc, err := repo.ListByDocument(ctx, d.ID)
	if err != nil || len(byDoc) != 1 {
		t.Fatalf("ListByDocument = %v, %v, want 1 item", byDoc, err)
	}

	byProject, err := repo.ListByProject(ctx, p.ID)
	if err != nil || len(byProject) != 1 {
		t.Fatalf("ListByProject = %v, %v, want 1 item", byProject, err)
	}
}

func TestAnalysisRepositoryLifecycle(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p, _ := seedProjectAndDocument(t, db)
	repo := NewAnalysisRepository(db)

	a := &domain.Analysis{ID: "ana_1", ProjectID: p.ID, Status: domain.AnalysisQueued, Progress: 0, CreatedAt: time.Now().UTC()}
	if err := repo.Create(ctx, a); err != nil {
		t.Fatalf("Create: %v", err)
	}

	now := time.Now().UTC()
	a.Status = domain.AnalysisRunning
	a.CurrentStep = "extracting_observations"
	a.Progress = 20
	a.StartedAt = &now
	if err := repo.Update(ctx, a); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.Get(ctx, a.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != domain.AnalysisRunning || got.Progress != 20 || got.CurrentStep != "extracting_observations" {
		t.Errorf("got = %+v", got)
	}
	if got.StartedAt == nil {
		t.Error("StartedAt should be set")
	}

	latest, err := repo.LatestByProject(ctx, p.ID)
	if err != nil || latest.ID != a.ID {
		t.Fatalf("LatestByProject = %v, %v", latest, err)
	}
}

func TestAnalysisRepositoryFailInterrupted(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p, _ := seedProjectAndDocument(t, db)
	repo := NewAnalysisRepository(db)

	running := &domain.Analysis{ID: "ana_running", ProjectID: p.ID, Status: domain.AnalysisRunning, CreatedAt: time.Now().UTC()}
	completed := &domain.Analysis{ID: "ana_done", ProjectID: p.ID, Status: domain.AnalysisCompleted, CreatedAt: time.Now().UTC()}
	if err := repo.Create(ctx, running); err != nil {
		t.Fatalf("create running: %v", err)
	}
	if err := repo.Create(ctx, completed); err != nil {
		t.Fatalf("create completed: %v", err)
	}

	n, err := repo.FailInterrupted(ctx)
	if err != nil {
		t.Fatalf("FailInterrupted: %v", err)
	}
	if n != 1 {
		t.Fatalf("FailInterrupted affected %d rows, want 1", n)
	}

	got, err := repo.Get(ctx, "ana_running")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != domain.AnalysisFailed || got.Error != "interrupted" {
		t.Errorf("got = %+v, want failed/interrupted", got)
	}

	stillDone, err := repo.Get(ctx, "ana_done")
	if err != nil || stillDone.Status != domain.AnalysisCompleted {
		t.Errorf("completed analysis should be untouched, got %+v, %v", stillDone, err)
	}
}

func TestInsightAndEvidenceRepository(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p, d := seedProjectAndDocument(t, db)
	observations := NewObservationRepository(db)
	insights := NewInsightRepository(db)
	evidence := NewEvidenceRepository(db)

	if err := observations.CreateBatch(ctx, []*domain.Observation{
		{ID: "obs_1", DocumentID: d.ID, Quote: "設定を間違えたら怖い", StartOffset: 0, EndOffset: 10, Behavior: "b", CreatedAt: time.Now().UTC()},
	}); err != nil {
		t.Fatalf("seed observation: %v", err)
	}

	insight := &domain.Insight{
		ID: "ins_1", ProjectID: p.ID, Title: "確認の裏にある恐怖",
		LatentNeed: "失敗して責任を負うリスクを避けたい", Confidence: 0.82, CreatedAt: time.Now().UTC(),
	}
	if err := insights.Create(ctx, insight); err != nil {
		t.Fatalf("Create insight: %v", err)
	}

	obsID := "obs_1"
	if err := evidence.CreateBatch(ctx, []*domain.Evidence{
		{ID: "ev_1", InsightID: insight.ID, DocumentID: d.ID, ObservationID: &obsID, Quote: "設定を間違えたら怖い", Type: domain.EvidenceSupport, RelevanceScore: 0.9, StartOffset: 0, EndOffset: 10},
	}); err != nil {
		t.Fatalf("CreateBatch evidence: %v", err)
	}

	gotInsight, err := insights.Get(ctx, insight.ID)
	if err != nil {
		t.Fatalf("Get insight: %v", err)
	}
	if gotInsight.LatentNeed != insight.LatentNeed {
		t.Errorf("LatentNeed mismatch: %q", gotInsight.LatentNeed)
	}

	list, err := insights.ListByProject(ctx, p.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListByProject = %v, %v", list, err)
	}

	ev, err := evidence.ListByInsight(ctx, insight.ID)
	if err != nil || len(ev) != 1 {
		t.Fatalf("ListByInsight = %v, %v", ev, err)
	}
	if ev[0].ObservationID == nil || *ev[0].ObservationID != "obs_1" {
		t.Errorf("ObservationID = %v, want obs_1", ev[0].ObservationID)
	}
	if ev[0].Type != domain.EvidenceSupport {
		t.Errorf("Type = %v, want support", ev[0].Type)
	}

	count, err := evidence.CountByProject(ctx, p.ID)
	if err != nil || count != 1 {
		t.Fatalf("CountByProject = %d, %v", count, err)
	}

	if _, err := insights.Get(ctx, "missing"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Get missing insight = %v, want ErrNotFound", err)
	}
}
