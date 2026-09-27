package service

import (
	"context"
	"errors"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
	"insight-lab/internal/sampledata"
)

// DemoProjectID is fixed so loading the demo dataset is idempotent: a
// second `--demo` run or a second click of "デモを試す" reuses the same
// project instead of creating duplicates.
const DemoProjectID = "demo-research-policy-v2"

type DemoLoader struct {
	Projects  repository.ProjectRepository
	Documents repository.DocumentRepository
}

func (l *DemoLoader) Ensure(ctx context.Context) (*domain.Project, error) {
	existing, err := l.Projects.Get(ctx, DemoProjectID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	docs, err := sampledata.Load()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	project := &domain.Project{
		ID:        DemoProjectID,
		Name:      "Demo: Evidence-grounded policy research",
		CreatedAt: now,
	}
	if err := l.Projects.Create(ctx, project); err != nil {
		return nil, err
	}

	for _, d := range docs {
		d.ProjectID = project.ID
		d.CreatedAt = now
	}
	if err := l.Documents.CreateBatch(ctx, docs); err != nil {
		return nil, err
	}

	return project, nil
}

// ScenarioProjectID is the stable project a sample scenario opens, so a
// second click reuses it instead of creating duplicates.
func ScenarioProjectID(scenarioID string) string {
	return "demo-scenario-" + scenarioID
}

// EnsureScenario creates (or reuses) the empty project for one sample
// scenario. It deliberately imports nothing: the user brings the scenario's
// CSV in through the ordinary preview → import flow, so the sample
// exercises the same importer as their own data.
func (l *DemoLoader) EnsureScenario(ctx context.Context, scenarioID string) (*domain.Project, error) {
	scenario, err := sampledata.FindScenario(scenarioID)
	if err != nil {
		return nil, err
	}
	id := ScenarioProjectID(scenario.ID)
	existing, err := l.Projects.Get(ctx, id)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	project := &domain.Project{ID: id, Name: scenario.ProjectName, CreatedAt: time.Now().UTC()}
	if err := l.Projects.Create(ctx, project); err != nil {
		return nil, err
	}
	return project, nil
}
