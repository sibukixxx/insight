package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/service"
)

// seedFullySupportedRun creates a project, an analysis whose Metrics report
// a fully-grounded deterministic run with no compatibility warnings, and a
// research run over one insight that carries every mechanically verifiable
// promotion signal: an expectation basis, disclosed missing evidence,
// falsification criteria, and both support and counter evidence.
func seedFullySupportedRun(t *testing.T, app *Application, ctx context.Context, now time.Time) *domain.ResearchRun {
	t.Helper()
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "policy", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	metrics := service.Metrics{
		TotalObservationCandidates: 4, GroundedObservations: 4,
		Provenance: service.RunProvenance{Mode: service.ExecutionModeDeterministic, RuleVersion: "v1", DatasetHashes: []string{"sha256:abc"}},
	}
	metricsJSON, err := json.Marshal(metrics)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.repos.Analyses.Create(ctx, &domain.Analysis{ID: "a1", ProjectID: "p1", Status: domain.AnalysisCompleted, Metrics: string(metricsJSON), CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := app.repos.Documents.Create(ctx, &domain.Document{ID: "doc1", ProjectID: "p1", Source: domain.SourceDataset, Title: "source", Content: "content", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	analysisID := "a1"
	insight := &domain.Insight{
		ID: "h1", ProjectID: "p1", AnalysisID: &analysisID, Title: "Policy effect",
		SurprisingFact: "designations rose", ExpectationBasis: domain.ExpectationPrior,
		ValidationStatus: domain.ValidationSupported, IdentificationStatus: domain.IdentificationNotIdentified,
		MissingEvidence: []string{"pre-period trend"}, FalsificationCriteria: []string{"no rise without the policy"},
		CreatedAt: now,
	}
	if err := app.repos.Insights.Create(ctx, insight); err != nil {
		t.Fatal(err)
	}
	if err := app.repos.Evidence.CreateBatch(ctx, []*domain.Evidence{
		{ID: "ev1", InsightID: "h1", Type: domain.EvidenceSupport, DocumentID: "doc1", Quote: "designations rose after the policy"},
		{ID: "ev2", InsightID: "h1", Type: domain.EvidenceCounter, DocumentID: "doc1", Quote: "a neighboring region rose too"},
	}); err != nil {
		t.Fatal(err)
	}
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "q"})
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func fullHumanJudgment() SubmitPromotionReviewInput {
	return SubmitPromotionReviewInput{
		Contribution: domain.ContributionCorrection, HumanReviewCompleted: true,
		CompetingHypothesisConsidered: true, IndependentValidationStatusAccurate: true, DecisionReadinessHonestlyStated: true,
	}
}

func TestSubmitPromotionReviewStopsAtHumanReviewRequiredWhenHumanReviewIsNotYetCompleted(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	run := seedFullySupportedRun(t, app, ctx, now)

	in := fullHumanJudgment()
	in.RunID, in.IterationID, in.HumanReviewCompleted = run.ID, run.Iterations[0].ID, false
	updated, err := app.SubmitPromotionReview(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Promotion.State != domain.PromotionHumanReviewRequired {
		t.Fatalf("expected HUMAN_REVIEW_REQUIRED without human review, got %s (reasons: %v)", updated.Promotion.State, updated.Promotion.Reasons)
	}
	if len(updated.Promotion.Reasons) == 0 {
		t.Fatal("blocked promotion must state why")
	}
}

func TestSubmitPromotionReviewAdvancesToReadyWhenEveryGateIsSatisfied(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	run := seedFullySupportedRun(t, app, ctx, now)

	in := fullHumanJudgment()
	in.RunID, in.IterationID = run.ID, run.Iterations[0].ID
	updated, err := app.SubmitPromotionReview(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Promotion.State != domain.PromotionPublicationReady {
		t.Fatalf("expected PUBLICATION_READY, got %s (reasons: %v)", updated.Promotion.State, updated.Promotion.Reasons)
	}

	persisted, err := app.GetResearchRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.CurrentPromotionState() != domain.PromotionPublicationReady {
		t.Fatalf("promotion state was not persisted: %+v", persisted.Iterations)
	}
}

func TestSubmitPromotionReviewRejectsAnEarlierIteration(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	run := seedFullySupportedRun(t, app, ctx, now)
	firstIterationID := run.Iterations[0].ID
	if _, err := app.AppendResearchIteration(ctx, AppendResearchIterationInput{RunID: run.ID}); err != nil {
		t.Fatal(err)
	}

	in := fullHumanJudgment()
	in.RunID, in.IterationID = run.ID, firstIterationID
	if _, err := app.SubmitPromotionReview(ctx, in); err == nil {
		t.Fatal("expected an error rejecting a non-latest iteration")
	}
}

func TestTransitionPromotionStateReusesTheLastSubmittedReviewEvidence(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	run := seedFullySupportedRun(t, app, ctx, now)

	in := fullHumanJudgment()
	in.RunID, in.IterationID, in.HumanReviewCompleted = run.ID, run.Iterations[0].ID, false
	if _, err := app.SubmitPromotionReview(ctx, in); err != nil {
		t.Fatal(err)
	}

	if _, err := app.TransitionPromotionState(ctx, TransitionPromotionStateInput{
		RunID: run.ID, IterationID: run.Iterations[0].ID, TargetState: domain.PromotionPublicationReady,
	}); !errors.Is(err, domain.ErrPromotionHumanReviewRequired) {
		t.Fatalf("expected ErrPromotionHumanReviewRequired using the stored review evidence, got %v", err)
	}
}

func TestTransitionPromotionStateAllowsRejectionFromDraftWithoutAnyReview(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	run := seedFullySupportedRun(t, app, ctx, now)

	rejected, err := app.TransitionPromotionState(ctx, TransitionPromotionStateInput{
		RunID: run.ID, IterationID: run.Iterations[0].ID, TargetState: domain.PromotionRejectedForPublication,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Promotion.State != domain.PromotionRejectedForPublication {
		t.Fatalf("expected REJECTED_FOR_PUBLICATION, got %s", rejected.Promotion.State)
	}
}

func TestTransitionPromotionStateRejectsSkippingAheadOfPublicationReady(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	run := seedFullySupportedRun(t, app, ctx, now)

	if _, err := app.TransitionPromotionState(ctx, TransitionPromotionStateInput{
		RunID: run.ID, IterationID: run.Iterations[0].ID, TargetState: domain.PromotionPublished,
	}); err == nil {
		t.Fatal("expected an error: PUBLISHED cannot be reached directly from DRAFT")
	}
}

func TestApprovedArtifactRemainsIdenticalThroughPublicationAndLaterAnalysis(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	run := seedFullySupportedRun(t, app, ctx, now)
	if _, err := app.GetApprovedResearchArtifact(ctx, run.ID); err == nil {
		t.Fatal("draft must not have approved artifact")
	}
	in := fullHumanJudgment()
	in.RunID, in.IterationID = run.ID, run.Iterations[0].ID
	if _, err := app.SubmitPromotionReview(ctx, in); err != nil {
		t.Fatal(err)
	}
	approved, err := app.GetApprovedResearchArtifact(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	original := string(approved.Artifact)
	if approved.Reference != fmt.Sprintf("sha256:%x", sha256.Sum256(approved.Artifact)) {
		t.Fatal("incorrect content reference")
	}
	now = now.Add(time.Hour)
	if err := app.repos.Analyses.Create(ctx, &domain.Analysis{ID: "unrelated", ProjectID: "p1", Status: domain.AnalysisCompleted, CreatedAt: now, Metrics: metricsJSON(t, service.RunProvenance{DatasetHashes: []string{"unrelated"}})}); err != nil {
		t.Fatal(err)
	}
	current, err := app.GetResearchArtifact(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(current.DatasetHashes) != 1 || current.DatasetHashes[0] != "sha256:abc" {
		t.Fatalf("provenance drifted: %v", current.DatasetHashes)
	}
	updated, err := app.TransitionPromotionState(ctx, TransitionPromotionStateInput{RunID: run.ID, IterationID: in.IterationID, TargetState: domain.PromotionPublished})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Promotion.State != domain.PromotionPublished {
		t.Fatal("explicit publish failed")
	}
	after, err := app.GetApprovedResearchArtifact(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(after.Artifact) != original || after.Reference != approved.Reference {
		t.Fatal("approved content changed")
	}
	if _, err := app.SubmitPromotionReview(ctx, in); !errors.Is(err, domain.ErrPromotionTransitionNotAllowed) {
		t.Fatalf("published review must be immutable: %v", err)
	}
}

func TestReviewRechecksAllGatesAndRevokesApproval(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	run := seedFullySupportedRun(t, app, ctx, time.Now())
	in := fullHumanJudgment()
	in.RunID, in.IterationID = run.ID, run.Iterations[0].ID
	if _, err := app.SubmitPromotionReview(ctx, in); err != nil {
		t.Fatal(err)
	}
	in.HumanReviewCompleted = false
	updated, err := app.SubmitPromotionReview(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Promotion.State != domain.PromotionHumanReviewRequired || updated.ApprovedArtifact != nil {
		t.Fatal("revoked review retained approval")
	}
	if _, err := app.GetApprovedResearchArtifact(ctx, run.ID); err == nil {
		t.Fatal("revoked artifact is accessible")
	}
	if _, err := app.TransitionPromotionState(ctx, TransitionPromotionStateInput{RunID: run.ID, IterationID: in.IterationID, TargetState: domain.PromotionPublished}); err == nil {
		t.Fatal("revoked artifact published")
	}
}

func TestFalsificationPlanAloneIsNotCounterEvidenceSearch(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Now()
	run := seedFullySupportedRun(t, app, ctx, now)
	analysisID := "a1"
	if err := app.repos.Insights.Create(ctx, &domain.Insight{ID: "planned", ProjectID: "p1", AnalysisID: &analysisID, Title: "Planned test", ExpectationBasis: domain.ExpectationPrior, FalsificationCriteria: []string{"future test"}, MissingEvidence: []string{"control group"}, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := app.repos.Evidence.CreateBatch(ctx, []*domain.Evidence{{ID: "only-support", InsightID: "planned", Type: domain.EvidenceSupport, DocumentID: "doc1", Quote: "supports the claim"}}); err != nil {
		t.Fatal(err)
	}
	iteration := run.Iterations[0]
	iteration.InsightIDs = []string{"planned"}
	if err := app.repos.Research.UpdateResearchIteration(ctx, run.ID, iteration); err != nil {
		t.Fatal(err)
	}
	in := fullHumanJudgment()
	in.RunID, in.IterationID = run.ID, iteration.ID
	reviewed, err := app.SubmitPromotionReview(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.PromotionGateInput.Checklist.CounterEvidenceSearched || reviewed.Promotion.State == domain.PromotionPublicationReady {
		t.Fatal("unexecuted falsification plan granted approval")
	}
}
