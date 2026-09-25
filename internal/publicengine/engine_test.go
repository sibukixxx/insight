package publicengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/buildinfo"
	"insight-lab/internal/domain"
	"insight-lab/internal/repository/sqlite"
	"insight-lab/internal/service"
	"insight-lab/internal/usecase"
)

type enqueueStub struct{ analyses *sqlite.AnalysisRepository }

func (s enqueueStub) Enqueue(ctx context.Context, req service.EnqueueRequest) (*domain.Analysis, error) {
	a := &domain.Analysis{ID: newID("ana"), ProjectID: req.ProjectID, Status: domain.AnalysisQueued, SemanticAnalysisMode: req.SemanticAnalysisMode, ResearchQuestion: req.ResearchQuestion, CreatedAt: time.Now().UTC()}
	return a, s.analyses.Create(ctx, a)
}

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "engine.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	documents, analyses := sqlite.NewDocumentRepository(db), sqlite.NewAnalysisRepository(db)
	app := usecase.New(usecase.Repositories{
		Projects: sqlite.NewProjectRepository(db), Documents: documents, Observations: sqlite.NewObservationRepository(db),
		Patterns: sqlite.NewPatternRepository(db), Analyses: analyses, Insights: sqlite.NewInsightRepository(db),
		Evidence: sqlite.NewEvidenceRepository(db), Research: sqlite.NewResearchRepository(db),
	})
	return New(app, sqlite.NewPublicRepository(db), documents, enqueueStub{analyses}, buildinfo.Info{Version: "v", Commit: "c", Dirty: "false"})
}

func createTestSubject(t *testing.T, e *Engine) string {
	t.Helper()
	_, body, err := e.CreateSubject(context.Background(), CreateSubjectRequest{ContractVersion: "1", IdempotencyKey: "subject", Subject: SubjectRef{Namespace: "ns", ID: "id"}})
	if err != nil {
		t.Fatal(err)
	}
	var subject Subject
	if err := json.Unmarshal(body, &subject); err != nil {
		t.Fatal(err)
	}
	return subject.SubjectID
}

func artifactFixture(t *testing.T) json.RawMessage {
	t.Helper()
	data, err := os.ReadFile("../../contracts/analytical-artifact/v1/fixtures/trade-time-series.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func doc(ref string) EvidenceDocument {
	return EvidenceDocument{ExternalRef: ref, Source: "interview", Content: "content of " + ref}
}

func manyDocs(n int) []EvidenceDocument {
	out := make([]EvidenceDocument, n)
	for i := range out {
		out[i] = doc(fmt.Sprintf("d%d", i))
	}
	return out
}

func metadataEntries(n int) map[string]string {
	out := map[string]string{}
	for i := 0; i < n; i++ {
		out[fmt.Sprintf("k%d", i)] = "v"
	}
	return out
}

func TestEngineRejectsInvalidRequestsWithTypedCodes(t *testing.T) {
	ctx := context.Background()
	e := newTestEngine(t)
	subjectID := createTestSubject(t, e)
	evidence := func(key string, docs []EvidenceDocument, artifacts ...json.RawMessage) error {
		_, _, err := e.AddEvidence(ctx, subjectID, AddEvidenceRequest{ContractVersion: "1", IdempotencyKey: key, Documents: docs, AnalyticalArtifacts: artifacts})
		return err
	}
	withMetadata := func(m map[string]string) []EvidenceDocument {
		d := doc("meta")
		d.Metadata = m
		return []EvidenceDocument{d}
	}

	cases := []struct {
		name string
		call func() error
		want Code
	}{
		{"missing idempotency key", func() error {
			_, _, err := e.CreateSubject(ctx, CreateSubjectRequest{ContractVersion: "1", Subject: SubjectRef{Namespace: "a", ID: "b"}})
			return err
		}, CodeInvalidRequest},
		{"subject id too long", func() error {
			_, _, err := e.CreateSubject(ctx, CreateSubjectRequest{ContractVersion: "1", IdempotencyKey: "long", Subject: SubjectRef{Namespace: "a", ID: strings.Repeat("x", 129)}})
			return err
		}, CodeInvalidRequest},
		{"too many metadata entries", func() error { return evidence("meta-count", withMetadata(metadataEntries(33))) }, CodeInvalidRequest},
		{"metadata key outside the pattern", func() error { return evidence("meta-key", withMetadata(map[string]string{"bad key": "v"})) }, CodeInvalidRequest},
		{"metadata value too long", func() error {
			return evidence("meta-value", withMetadata(map[string]string{"k": strings.Repeat("v", 1025)}))
		}, CodeInvalidRequest},
		{"reserved public_ key", func() error {
			return evidence("reserved-public", withMetadata(map[string]string{"public_external_ref": "x"}))
		}, CodeInvalidRequest},
		{"claimed dataset hash", func() error { return evidence("reserved-hash", withMetadata(map[string]string{"dataset_hash": "abc"})) }, CodeInvalidRequest},
		{"claimed acquisition manifest", func() error {
			return evidence("reserved-manifest", withMetadata(map[string]string{"acquisition_manifest": "{}"}))
		}, CodeInvalidRequest},
		{"no evidence at all", func() error { return evidence("empty", nil) }, CodeInvalidRequest},
		{"duplicate externalRef in one request", func() error { return evidence("dup", []EvidenceDocument{doc("same"), doc("same")}) }, CodeInvalidRequest},
		{"more than 500 documents", func() error { return evidence("many", manyDocs(501)) }, CodeInvalidRequest},
		{"unsupported source", func() error {
			d := doc("src")
			d.Source = "email"
			return evidence("source", []EvidenceDocument{d})
		}, CodeInvalidRequest},
		{"invalid analytical artifact", func() error { return evidence("bad-artifact", nil, json.RawMessage(`{"id":"x"}`)) }, CodeInvalidRequest},
		{"unknown subject", func() error {
			_, _, err := e.AddEvidence(ctx, "missing", AddEvidenceRequest{ContractVersion: "1", IdempotencyKey: "missing", Documents: []EvidenceDocument{doc("x")}})
			return err
		}, CodeNotFound},
		{"unknown semantic analysis mode", func() error {
			_, _, err := e.StartAnalysis(ctx, subjectID, StartAnalysisRequest{ContractVersion: "1", IdempotencyKey: "mode", SemanticAnalysisMode: "GUESS"})
			return err
		}, CodeInvalidRequest},
		{"question too long", func() error {
			_, _, err := e.CreateResearchRun(ctx, subjectID, CreateResearchRunRequest{ContractVersion: "1", IdempotencyKey: "question", Question: strings.Repeat("q", 2001), AnalysisID: "a"})
			return err
		}, CodeInvalidRequest},
		{"research on an unknown analysis", func() error {
			_, _, err := e.CreateResearchRun(ctx, subjectID, CreateResearchRunRequest{ContractVersion: "1", IdempotencyKey: "unknown-analysis", Question: "q", AnalysisID: "missing"})
			return err
		}, CodeNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AsError(tc.call()); got.Code != tc.want {
				t.Fatalf("code = %s (%s), want %s", got.Code, got.Message, tc.want)
			}
		})
	}
}

func TestAddEvidenceRejectsAnArtifactWhoseHashChangedUnderTheSameID(t *testing.T) {
	ctx := context.Background()
	e := newTestEngine(t)
	subjectID := createTestSubject(t, e)
	original := artifactFixture(t)
	if _, _, err := e.AddEvidence(ctx, subjectID, AddEvidenceRequest{ContractVersion: "1", IdempotencyKey: "a1", AnalyticalArtifacts: []json.RawMessage{original}}); err != nil {
		t.Fatal(err)
	}
	var changed map[string]any
	if err := json.Unmarshal(original, &changed); err != nil {
		t.Fatal(err)
	}
	changed["artifactHash"] = map[string]any{"algorithm": "sha256", "value": strings.Repeat("b", 64)}
	encoded, _ := json.Marshal(changed)
	_, _, err := e.AddEvidence(ctx, subjectID, AddEvidenceRequest{ContractVersion: "1", IdempotencyKey: "a2", AnalyticalArtifacts: []json.RawMessage{encoded}})
	if got := AsError(err); got.Code != CodeIdentityConflict {
		t.Fatalf("code = %s, want IDENTITY_CONFLICT", got.Code)
	}
}

func TestAddEvidenceWritesNothingWhenOneItemConflicts(t *testing.T) {
	ctx := context.Background()
	e := newTestEngine(t)
	subjectID := createTestSubject(t, e)
	if _, _, err := e.AddEvidence(ctx, subjectID, AddEvidenceRequest{ContractVersion: "1", IdempotencyKey: "first", Documents: []EvidenceDocument{doc("kept")}}); err != nil {
		t.Fatal(err)
	}
	edited := doc("kept")
	edited.Content = "different"
	_, _, err := e.AddEvidence(ctx, subjectID, AddEvidenceRequest{ContractVersion: "1", IdempotencyKey: "second", Documents: []EvidenceDocument{doc("new"), edited}})
	if got := AsError(err); got.Code != CodeIdentityConflict {
		t.Fatalf("code = %s, want IDENTITY_CONFLICT", got.Code)
	}
	docs, err := e.app.ListDocuments(ctx, subjectID)
	if err != nil || len(docs) != 1 {
		t.Fatalf("documents after a rejected request = %d, %v; want only the first document", len(docs), err)
	}
}

func TestUnrecognisedErrorsDoNotLeakDetail(t *testing.T) {
	got := AsError(errors.New("sqlite: disk I/O error at /secret/path"))
	if got.Code != CodeInternal || strings.Contains(got.Message, "secret") {
		t.Fatalf("AsError = %+v", got)
	}
}

func TestResearchIsNotReachableForProjectsCreatedOutsideThePublicContract(t *testing.T) {
	ctx := context.Background()
	e := newTestEngine(t)
	project, err := e.app.CreateProject(ctx, "ui project")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.StartAnalysis(ctx, project.ID, StartAnalysisRequest{ContractVersion: "1", IdempotencyKey: "ui"}); AsError(err).Code != CodeNotFound {
		t.Fatalf("a UI project must not be addressable as a subject: %v", err)
	}
}
