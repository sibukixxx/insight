package usecase

import (
	"context"
	"strings"
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
func (r *projectRepositoryStub) UpdateIntakeProfile(_ context.Context, id string, profile domain.IntakeProfile) error {
	p := r.projects[id]
	if p == nil {
		return repository.ErrNotFound
	}
	p.IntakeProfile = profile
	return nil
}

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

func TestPreviewIntakeAndRememberedSpeakerRoles(t *testing.T) {
	projects := &projectRepositoryStub{projects: map[string]*domain.Project{}}
	documents := &documentRepositoryStub{}
	app := New(Repositories{Projects: projects, Documents: documents})
	ctx := context.Background()
	p, err := app.CreateProject(ctx, "intake")
	if err != nil {
		t.Fatal(err)
	}

	content := "田中: 普段の業務を教えてください。\n佐藤: 毎月月末に請求書を作っています。必ず電卓で検算します。\n田中: なるほど。\n佐藤: そこだけは外せません。"
	preview, err := app.PreviewIntake(ctx, PreviewIntakeInput{ProjectID: p.ID, Source: domain.SourceInterview, Content: content})
	if err != nil {
		t.Fatalf("PreviewIntake: %v", err)
	}
	if !preview.Transcript.Detected || len(preview.Spans) != 4 {
		t.Fatalf("preview = %+v", preview)
	}
	if preview.CustomerChars == 0 || preview.ExcludedChars == 0 || preview.CustomerChars+preview.ExcludedChars > preview.TotalChars {
		t.Errorf("char accounting: customer=%d excluded=%d total=%d", preview.CustomerChars, preview.ExcludedChars, preview.TotalChars)
	}
	if preview.Provenance != domain.ProvenanceFirsthand {
		t.Errorf("Provenance = %q", preview.Provenance)
	}

	// The user flips the heuristic (say 田中 is actually the customer) and
	// commits; the project must remember that for the next paste.
	roles := map[string]domain.SpeakerRole{"田中": domain.RoleCustomer, "佐藤": domain.RoleInterviewer}
	preview, err = app.PreviewIntake(ctx, PreviewIntakeInput{ProjectID: p.ID, Source: domain.SourceInterview, Content: content, SpeakerRoles: roles})
	if err != nil {
		t.Fatal(err)
	}
	for _, sp := range preview.Transcript.Speakers {
		if sp.Label == "田中" && sp.Role != domain.RoleCustomer {
			t.Errorf("override ignored: %+v", sp)
		}
	}
	doc, err := app.CreateDocument(ctx, CreateDocumentInput{
		ProjectID: p.ID, Source: domain.SourceInterview, Content: content, Spans: preview.Spans, SpeakerRoles: roles,
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	if len(doc.Spans) != 4 || doc.Provenance != domain.ProvenanceFirsthand {
		t.Errorf("stored document = %+v", doc)
	}
	profile, err := app.GetIntakeProfile(ctx, p.ID)
	if err != nil || profile.SpeakerRoles["田中"] != domain.RoleCustomer {
		t.Errorf("profile did not remember roles: %+v, %v", profile, err)
	}

	// A later preview without overrides now uses the remembered mapping.
	preview, err = app.PreviewIntake(ctx, PreviewIntakeInput{ProjectID: p.ID, Source: domain.SourceInterview, Content: content})
	if err != nil {
		t.Fatal(err)
	}
	for _, sp := range preview.Transcript.Speakers {
		if sp.Label == "田中" && (sp.Role != domain.RoleCustomer || sp.Guessed) {
			t.Errorf("remembered mapping not applied: %+v", sp)
		}
	}

	// Spans outside the content are rejected, not stored.
	if _, err := app.CreateDocument(ctx, CreateDocumentInput{
		ProjectID: p.ID, Source: domain.SourceInterview, Content: "短い",
		Spans: []domain.Span{{Start: 0, End: 99, Role: domain.RoleCustomer}},
	}); err == nil {
		t.Error("out-of-range span should be rejected")
	}

	// Prose is not a transcript: no spans, everything is the customer.
	preview, err = app.PreviewIntake(ctx, PreviewIntakeInput{ProjectID: p.ID, Source: domain.SourceReview, Content: "操作は簡単です。でも確認は欠かしません。"})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Transcript.Detected || preview.Spans != nil || preview.CustomerChars != preview.TotalChars {
		t.Errorf("prose preview = %+v", preview)
	}
}

func TestCreateDocumentMasksAndDerivesSpansServerSide(t *testing.T) {
	projects := &projectRepositoryStub{projects: map[string]*domain.Project{}}
	documents := &documentRepositoryStub{}
	app := New(Repositories{Projects: projects, Documents: documents})
	ctx := context.Background()
	p, err := app.CreateProject(ctx, "mask")
	if err != nil {
		t.Fatal(err)
	}
	if err := app.UpdateIntakeProfile(ctx, p.ID, domain.IntakeProfile{MaskTerms: []string{"株式会社サンプル"}}); err != nil {
		t.Fatal(err)
	}

	content := "面接官: 株式会社サンプルの田中さんですね。連絡先は？\n回答者: 090-1234-5678 です。検算は毎回やります。"
	preview, err := app.PreviewIntake(ctx, PreviewIntakeInput{ProjectID: p.ID, Source: domain.SourceInterview, Content: content})
	if err != nil {
		t.Fatal(err)
	}
	if preview.MaskCount != 3 || preview.MaskByKind["term"] != 1 || preview.MaskByKind["name"] != 1 || preview.MaskByKind["phone"] != 1 {
		t.Errorf("mask accounting = %d %v", preview.MaskCount, preview.MaskByKind)
	}
	if strings.Contains(preview.Masked, "田中") || strings.Contains(preview.Masked, "090") {
		t.Errorf("preview should show masked text: %q", preview.Masked)
	}
	// Turns index the masked text.
	runes := []rune(preview.Masked)
	last := preview.Transcript.Turns[len(preview.Transcript.Turns)-1]
	if got := string(runes[last.Start:last.End]); got != "[電話番号] です。検算は毎回やります。" {
		t.Errorf("customer turn on masked text = %q", got)
	}

	doc, err := app.CreateDocument(ctx, CreateDocumentInput{
		ProjectID: p.ID, Source: domain.SourceInterview, Content: content, DetectSpeakers: true,
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	if strings.Contains(doc.Content, "田中") || !strings.Contains(doc.RawContent, "田中") {
		t.Errorf("stored content must be masked and raw kept: content=%q raw=%q", doc.Content, doc.RawContent)
	}
	if len(doc.Spans) != 2 || doc.Spans[1].Role != domain.RoleCustomer {
		t.Errorf("spans not derived server-side: %+v", doc.Spans)
	}
	if got := string([]rune(doc.Content)[doc.Spans[1].Start:doc.Spans[1].End]); !strings.HasPrefix(got, "[電話番号]") {
		t.Errorf("span indexes masked content: %q", got)
	}

	// Client spans computed on unmasked text are refused once masking
	// changed the content.
	if _, err := app.CreateDocument(ctx, CreateDocumentInput{
		ProjectID: p.ID, Source: domain.SourceInterview, Content: content,
		Spans: []domain.Span{{Start: 0, End: 5, Role: domain.RoleCustomer}},
	}); err == nil {
		t.Error("client spans on masked content should be rejected")
	}

	// Opting out keeps the original.
	doc, err = app.CreateDocument(ctx, CreateDocumentInput{ProjectID: p.ID, Source: domain.SourceInterview, Content: content, SkipMask: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc.Content, "田中") || doc.RawContent != "" {
		t.Errorf("skipMask should store the original: %+v", doc)
	}
}
