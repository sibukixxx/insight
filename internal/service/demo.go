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
const DemoProjectID = "demo-invoicing-saas"

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
		Name:      "Demo: Invoicing SaaS interviews",
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
