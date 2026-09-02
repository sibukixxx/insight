package usecase

import (
	"context"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

type projectRepositoryStub struct {
	projects map[string]*domain.Project
}

func (r *projectRepositoryStub) Create(_ context.Context, p *domain.Project) error {
	r.projects[p.ID] = p
	return nil
}
func (r *projectRepositoryStub) Get(_ context.Context, id string) (*domain.Project, error) {
	p := r.projects[id]
	if p == nil {
		return nil, repository.ErrNotFound
	}
	return p, nil
}
func (r *projectRepositoryStub) List(context.Context) ([]*domain.Project, error) { return nil, nil }
func (r *projectRepositoryStub) Delete(context.Context, string) error            { return nil }

type documentRepositoryStub struct {
	created *domain.Document
}

func (r *documentRepositoryStub) Create(_ context.Context, d *domain.Document) error {
	r.created = d
	return nil
}
func (r *documentRepositoryStub) CreateBatch(context.Context, []*domain.Document) error { return nil }
func (r *documentRepositoryStub) Get(context.Context, string) (*domain.Document, error) {
	return nil, repository.ErrNotFound
}
func (r *documentRepositoryStub) ListByProject(context.Context, string) ([]*domain.Document, error) {
	return nil, nil
}

func TestCreateProjectOwnsValidationAndEntityConstruction(t *testing.T) {
	projects := &projectRepositoryStub{projects: map[string]*domain.Project{}}
	app := New(Repositories{Projects: projects})
	fixed := time.Date(2026, 9, 2, 1, 2, 3, 0, time.UTC)
	app.now = func() time.Time { return fixed }

	p, err := app.CreateProject(context.Background(), "  Research  ")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Research" || p.CreatedAt != fixed || p.ID == "" {
		t.Fatalf("unexpected project: %#v", p)
	}
	if projects.projects[p.ID] != p {
		t.Fatal("project was not persisted")
	}
	if _, err := app.CreateProject(context.Background(), " \t "); err == nil {
		t.Fatal("expected empty name to be rejected")
	}
}

func TestCreateDocumentChecksParentAndInput(t *testing.T) {
	projects := &projectRepositoryStub{projects: map[string]*domain.Project{
		"proj_1": {ID: "proj_1", Name: "Research"},
	}}
	documents := &documentRepositoryStub{}
	app := New(Repositories{Projects: projects, Documents: documents})

	d, err := app.CreateDocument(context.Background(), CreateDocumentInput{
		ProjectID: "proj_1", Source: domain.SourceInterview, Content: "customer quote",
	})
	if err != nil {
		t.Fatal(err)
	}
	if documents.created != d || d.ProjectID != "proj_1" {
		t.Fatalf("unexpected document: %#v", d)
	}
	if _, err := app.CreateDocument(context.Background(), CreateDocumentInput{
		ProjectID: "missing", Source: domain.SourceInterview, Content: "text",
	}); err == nil {
		t.Fatal("expected missing project to be rejected")
	}
	if _, err := app.CreateDocument(context.Background(), CreateDocumentInput{
		ProjectID: "proj_1", Source: "invalid", Content: "text",
	}); err == nil {
		t.Fatal("expected invalid source to be rejected")
	}
}
