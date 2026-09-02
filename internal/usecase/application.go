// Package usecase contains application-specific orchestration.
package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
	"insight-lab/internal/service"
)

// ErrNotFound is the transport-facing sentinel for a missing aggregate.
// Keeping it at this boundary prevents handlers from importing persistence
// ports solely to classify an application error.
var ErrNotFound = repository.ErrNotFound

// Repositories groups the domain repository ports required by Application.
type Repositories struct {
	Projects     repository.ProjectRepository
	Documents    repository.DocumentRepository
	Observations repository.ObservationRepository
	Patterns     repository.PatternRepository
	Analyses     repository.AnalysisRepository
	Insights     repository.InsightRepository
	Evidence     repository.EvidenceRepository
}

// Application implements synchronous user-facing use cases. Transport layers
// depend on this type instead of coordinating repositories directly.
type Application struct {
	repos Repositories
	now   func() time.Time
}

func New(repos Repositories) *Application {
	return &Application{repos: repos, now: func() time.Time { return time.Now().UTC() }}
}

func (a *Application) RequireProject(ctx context.Context, projectID string) error {
	_, err := a.repos.Projects.Get(ctx, projectID)
	return err
}

func (a *Application) ListProjects(ctx context.Context) ([]*domain.Project, error) {
	return a.repos.Projects.List(ctx)
}

func (a *Application) CreateProject(ctx context.Context, name string) (*domain.Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	p := &domain.Project{ID: newID("proj"), Name: name, CreatedAt: a.now()}
	if err := a.repos.Projects.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return p, nil
}

func (a *Application) GetProject(ctx context.Context, id string) (*domain.Project, error) {
	return a.repos.Projects.Get(ctx, id)
}

func (a *Application) DeleteProject(ctx context.Context, id string) error {
	return a.repos.Projects.Delete(ctx, id)
}

func (a *Application) ListDocuments(ctx context.Context, projectID string) ([]*domain.Document, error) {
	if err := a.RequireProject(ctx, projectID); err != nil {
		return nil, err
	}
	return a.repos.Documents.ListByProject(ctx, projectID)
}

type CreateDocumentInput struct {
	ProjectID string
	Source    domain.SourceType
	Title     string
	Content   string
	Metadata  map[string]string
}

func (a *Application) CreateDocument(ctx context.Context, in CreateDocumentInput) (*domain.Document, error) {
	if err := a.RequireProject(ctx, in.ProjectID); err != nil {
		return nil, err
	}
	if !in.Source.Valid() {
		return nil, fmt.Errorf("invalid source type")
	}
	if strings.TrimSpace(in.Content) == "" {
		return nil, fmt.Errorf("content is required")
	}
	d := &domain.Document{ID: newID("doc"), ProjectID: in.ProjectID, Source: in.Source,
		Title: in.Title, Content: in.Content, Metadata: in.Metadata, CreatedAt: a.now()}
	if err := a.repos.Documents.Create(ctx, d); err != nil {
		return nil, fmt.Errorf("create document: %w", err)
	}
	return d, nil
}

func (a *Application) GetDocument(ctx context.Context, id string) (*domain.Document, error) {
	return a.repos.Documents.Get(ctx, id)
}

func (a *Application) ImportDocumentsCSV(ctx context.Context, projectID string, r io.Reader) (*service.ImportResult, error) {
	if err := a.RequireProject(ctx, projectID); err != nil {
		return nil, err
	}
	return service.ImportCSV(ctx, a.repos.Documents, projectID, r)
}

func (a *Application) GetAnalysis(ctx context.Context, id string) (*domain.Analysis, error) {
	return a.repos.Analyses.Get(ctx, id)
}

func (a *Application) ListAnalyses(ctx context.Context, projectID string) ([]*domain.Analysis, error) {
	if err := a.RequireProject(ctx, projectID); err != nil {
		return nil, err
	}
	return a.repos.Analyses.ListByProject(ctx, projectID)
}

func (a *Application) LatestAnalysis(ctx context.Context, projectID string) (*domain.Analysis, error) {
	if err := a.RequireProject(ctx, projectID); err != nil {
		return nil, err
	}
	return a.repos.Analyses.LatestByProject(ctx, projectID)
}

type InsightDetail struct {
	Insight  *domain.Insight
	Evidence []*domain.Evidence
	Patterns []PatternDetail
}

type PatternDetail struct {
	Pattern      *domain.Pattern
	Observations []*domain.Observation
}

func (a *Application) ListInsights(ctx context.Context, projectID string) ([]*domain.Insight, error) {
	if err := a.RequireProject(ctx, projectID); err != nil {
		return nil, err
	}
	return a.repos.Insights.ListByProject(ctx, projectID)
}

func (a *Application) GetInsight(ctx context.Context, id string) (*InsightDetail, error) {
	insight, err := a.repos.Insights.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	evidence, err := a.repos.Evidence.ListByInsight(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list insight evidence: %w", err)
	}
	patterns, err := a.repos.Patterns.ListByInsight(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list insight patterns: %w", err)
	}
	details, err := a.resolvePatterns(ctx, patterns)
	if err != nil {
		return nil, err
	}
	return &InsightDetail{Insight: insight, Evidence: evidence, Patterns: details}, nil
}

func (a *Application) GetInsightEvidence(ctx context.Context, id string) ([]*domain.Evidence, error) {
	if _, err := a.repos.Insights.Get(ctx, id); err != nil {
		return nil, err
	}
	return a.repos.Evidence.ListByInsight(ctx, id)
}

func (a *Application) ListPatterns(ctx context.Context, projectID string) ([]PatternDetail, error) {
	if err := a.RequireProject(ctx, projectID); err != nil {
		return nil, err
	}
	patterns, err := a.repos.Patterns.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return a.resolvePatterns(ctx, patterns)
}

func (a *Application) resolvePatterns(ctx context.Context, patterns []*domain.Pattern) ([]PatternDetail, error) {
	var ids []string
	for _, p := range patterns {
		ids = append(ids, p.ObservationIDs...)
	}
	observations, err := a.repos.Observations.ListByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("resolve pattern observations: %w", err)
	}
	byID := make(map[string]*domain.Observation, len(observations))
	for _, observation := range observations {
		byID[observation.ID] = observation
	}
	result := make([]PatternDetail, 0, len(patterns))
	for _, p := range patterns {
		detail := PatternDetail{Pattern: p}
		for _, id := range p.ObservationIDs {
			if observation := byID[id]; observation != nil {
				detail.Observations = append(detail.Observations, observation)
			}
		}
		result = append(result, detail)
	}
	return result, nil
}

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}
