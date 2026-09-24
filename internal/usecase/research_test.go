package usecase

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository/sqlite"
)

func newResearchTestApp(t *testing.T) (*Application, context.Context) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "research.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	app := New(Repositories{
		Projects: sqlite.NewProjectRepository(db), Documents: sqlite.NewDocumentRepository(db),
		Observations: sqlite.NewObservationRepository(db), Patterns: sqlite.NewPatternRepository(db),
		Analyses: sqlite.NewAnalysisRepository(db), Insights: sqlite.NewInsightRepository(db),
		Evidence: sqlite.NewEvidenceRepository(db), Research: sqlite.NewResearchRepository(db),
		Scenarios: sqlite.NewScenarioRepository(db),
	})
	return app, context.Background()
}

// seedAnalysis creates a completed analysis with the given insights and
// returns its ID. createdAt must strictly increase across calls for the
// same project so LatestByProject picks the intended analysis.
func seedAnalysis(t *testing.T, app *Application, ctx context.Context, projectID, analysisID string, createdAt time.Time, insights []*domain.Insight) {
	t.Helper()
	if err := app.repos.Analyses.Create(ctx, &domain.Analysis{ID: analysisID, ProjectID: projectID, Status: domain.AnalysisCompleted, CreatedAt: createdAt}); err != nil {
		t.Fatal(err)
	}
	for _, insight := range insights {
		insight.ProjectID, insight.AnalysisID = projectID, &analysisID
		if insight.CreatedAt.IsZero() {
			insight.CreatedAt = createdAt
		}
		if err := app.repos.Insights.Create(ctx, insight); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCreateResearchRunFirstIterationIsExploratoryOnlyEvenWhenSupported(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	fixed := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return fixed }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "policy", CreatedAt: fixed}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p1", "a1", fixed, []*domain.Insight{
		{ID: "h1", Title: "Policy effect", SurprisingFact: "designations rose", ValidationStatus: domain.ValidationSupported, IdentificationStatus: domain.IdentificationNotIdentified},
	})

	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "did the policy cause it?"})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Iterations) != 1 {
		t.Fatalf("expected exactly one iteration: %+v", run)
	}
	got := run.Iterations[0]
	if got.Readiness.State != domain.ReadinessExploratoryOnly {
		t.Fatalf("a single pass must not be decision-ready even with a supported hypothesis: %+v", got.Readiness)
	}
	if got.Stop != nil {
		t.Fatalf("first iteration must not stop: %+v", got.Stop)
	}
	if got.Readiness.AssessedAt != fixed {
		t.Fatal("readiness must be assessed using the application clock")
	}
}

func TestAppendResearchIterationWithoutAddedEvidenceStaysExploratory(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "policy", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p1", "a1", now, []*domain.Insight{
		{ID: "h1", Title: "Policy effect", SurprisingFact: "designations rose", ValidationStatus: domain.ValidationPartiallySupported, IdentificationStatus: domain.IdentificationNotIdentified},
	})
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "did the policy cause it?"})
	if err != nil {
		t.Fatal(err)
	}

	second, err := app.AppendResearchIteration(ctx, AppendResearchIterationInput{RunID: run.ID})
	if err != nil {
		t.Fatal(err)
	}
	if second.Readiness.State != domain.ReadinessExploratoryOnly {
		t.Fatalf("re-analysis without added evidence is still exploratory: %+v", second.Readiness)
	}
}

func TestAppendResearchIterationWithAddedEvidenceReachesDecisionReadyAndStops(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	first := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)
	app.now = func() time.Time { return first }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "policy", CreatedAt: first}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p1", "a1", first, []*domain.Insight{
		{ID: "h1", Title: "Policy effect", SurprisingFact: "designations rose", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified, MissingEvidence: []string{"comparison trend before treatment"}},
		{ID: "h2", Title: "Macro trend", SurprisingFact: "designations rose", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified, MissingEvidence: []string{"comparison trend before treatment"}},
	})
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "did the policy cause it?"})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Iterations[0].ResearchGaps) != 1 {
		t.Fatalf("expected one merged gap across both hypotheses: %+v", run.Iterations[0].ResearchGaps)
	}

	app.now = func() time.Time { return second }
	seedAnalysis(t, app, ctx, "p1", "a2", second, []*domain.Insight{
		{ID: "h1b", Title: "Policy effect", SurprisingFact: "designations rose", ValidationStatus: domain.ValidationSupported, IdentificationStatus: domain.IdentificationNotIdentified},
		{ID: "h2b", Title: "Macro trend", SurprisingFact: "designations rose", ValidationStatus: domain.ValidationContradicted, IdentificationStatus: domain.IdentificationNotIdentified},
	})

	got, err := app.AppendResearchIteration(ctx, AppendResearchIterationInput{RunID: run.ID, AddedEvidence: []string{"comparison series"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.HypothesisChanges) != 2 {
		t.Fatalf("hypothesis history must compare against the previous iteration: %+v", got.HypothesisChanges)
	}
	if len(got.ResearchGaps) != 1 || !got.ResearchGaps[0].Resolved || got.ResearchGaps[0].ID != run.Iterations[0].ResearchGaps[0].ID {
		t.Fatalf("the prior gap must be carried forward and marked resolved by the added evidence: %+v", got.ResearchGaps)
	}
	if got.Readiness.State != domain.ReadinessDecisionReadyWithLimitation {
		t.Fatalf("one surviving supported hypothesis with no open discriminating gap should be decision-ready with limitations: %+v", got.Readiness)
	}
	if got.Stop == nil || got.Stop.Reason != domain.StopHypothesesDistinguished || got.Stop.Source != domain.StopSourceSystem {
		t.Fatalf("the system must record why it stopped: %+v", got.Stop)
	}

	persisted, err := app.GetResearchRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Iterations) != 2 || persisted.Iterations[1].Stop == nil {
		t.Fatalf("the stop decision must be persisted: %+v", persisted.Iterations)
	}
}

func TestApplyResearchHumanOverrideAppendsHistoryAndPersists(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "policy", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p1", "a1", now, []*domain.Insight{
		{ID: "h1", Title: "Policy effect", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified},
	})
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "q"})
	if err != nil {
		t.Fatal(err)
	}
	iterationID := run.Iterations[0].ID

	overridden, err := app.ApplyResearchHumanOverride(ctx, ApplyResearchHumanOverrideInput{
		RunID: run.ID, IterationID: iterationID, StopReason: domain.StopExternalBudgetBoundary, Note: "sprint ended",
	})
	if err != nil {
		t.Fatal(err)
	}
	if overridden.Stop == nil || overridden.Stop.Reason != domain.StopExternalBudgetBoundary || overridden.Stop.Source != domain.StopSourceHuman {
		t.Fatalf("human stop was not recorded: %+v", overridden.Stop)
	}
	if len(overridden.ResearchGaps) != len(run.Iterations[0].ResearchGaps) {
		t.Fatal("a human stop must not remove research gaps")
	}
	if len(overridden.HumanOverrides) != 1 || overridden.HumanOverrides[0].Note != "sprint ended" {
		t.Fatalf("override history missing: %+v", overridden.HumanOverrides)
	}

	persisted, err := app.GetResearchIteration(ctx, run.ID, iterationID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Stop == nil || persisted.Stop.Source != domain.StopSourceHuman {
		t.Fatalf("override was not persisted: %+v", persisted)
	}
}

func TestApplyResearchHumanOverrideRejectsOverridingAnEarlierIteration(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "policy", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p1", "a1", now, []*domain.Insight{
		{ID: "h1", Title: "Policy effect", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified},
	})
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "q"})
	if err != nil {
		t.Fatal(err)
	}
	firstIterationID := run.Iterations[0].ID
	if _, err := app.AppendResearchIteration(ctx, AppendResearchIterationInput{RunID: run.ID}); err != nil {
		t.Fatal(err)
	}

	if _, err := app.ApplyResearchHumanOverride(ctx, ApplyResearchHumanOverrideInput{
		RunID: run.ID, IterationID: firstIterationID, StopReason: domain.StopHumanChoice,
	}); err == nil || !strings.Contains(err.Error(), "latest") {
		t.Fatalf("expected an error naming the latest-iteration rule, got %v", err)
	}
}

func TestApplyResearchHumanOverrideRejectsUnknownIteration(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "policy", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p1", "a1", now, []*domain.Insight{{ID: "h1", Title: "x", ValidationStatus: domain.ValidationPlausible}})
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "q"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.ApplyResearchHumanOverride(ctx, ApplyResearchHumanOverrideInput{RunID: run.ID, IterationID: "missing", StopReason: domain.StopHumanChoice}); err == nil {
		t.Fatal("expected an error for an iteration that does not exist")
	}
	if _, err := app.ApplyResearchHumanOverride(ctx, ApplyResearchHumanOverrideInput{RunID: "missing-run", IterationID: run.Iterations[0].ID, StopReason: domain.StopHumanChoice}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for a missing run, got %v", err)
	}
}

func TestTransitionResearchStageRejectsForwardMoveWithoutFrozenExpectations(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "policy", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p1", "a1", now, []*domain.Insight{
		{ID: "h1", Title: "Policy effect", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified},
	})
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "q"})
	if err != nil {
		t.Fatal(err)
	}
	if run.Iterations[0].Stage != domain.StageExploratory {
		t.Fatalf("a freshly created run must start EXPLORATORY: %+v", run.Iterations[0])
	}

	_, err = app.TransitionResearchStage(ctx, TransitionResearchStageInput{
		RunID: run.ID, IterationID: run.Iterations[0].ID, TargetStage: domain.StageValidation, IndependentEvidencePlanned: true,
	})
	if !errors.Is(err, domain.ErrStageRequiresFrozenTarget) {
		t.Fatalf("expected ErrStageRequiresFrozenTarget without any frozen expectation, got %v", err)
	}
}

func TestFreezeResearchExpectationPersistsFrozenCopyWithoutChangingProvenance(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "policy", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p1", "a1", now, []*domain.Insight{
		{
			ID: "h1", Title: "Policy effect", Expectation: "registrations rise after the subsidy starts",
			ExpectationBasis: domain.ExpectationPrior, FalsificationCriteria: []string{"registrations flat or falling"},
			ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified,
		},
	})
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "q"})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Iterations[0].Expectations) != 1 {
		t.Fatalf("expected the insight's expectation to be projected onto the iteration: %+v", run.Iterations[0])
	}
	expectationID := run.Iterations[0].Expectations[0].ID

	updated, err := app.FreezeResearchExpectation(ctx, FreezeResearchExpectationInput{
		RunID: run.ID, IterationID: run.Iterations[0].ID, ExpectationID: expectationID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Expectations[0].FrozenForValidation || updated.Expectations[0].FrozenAt == nil {
		t.Fatalf("expectation must be frozen: %+v", updated.Expectations[0])
	}
	if updated.Expectations[0].Provenance != domain.ExpectationPrior {
		t.Fatalf("freezing must not change provenance: %+v", updated.Expectations[0])
	}

	persisted, err := app.GetResearchIteration(ctx, run.ID, run.Iterations[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !persisted.Expectations[0].FrozenForValidation {
		t.Fatalf("frozen expectation must be persisted: %+v", persisted.Expectations[0])
	}
}

func TestFreezeResearchExpectationRejectsUnknownExpectationID(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "policy", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p1", "a1", now, []*domain.Insight{
		{ID: "h1", Title: "Policy effect", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified},
	})
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "q"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := app.FreezeResearchExpectation(ctx, FreezeResearchExpectationInput{
		RunID: run.ID, IterationID: run.Iterations[0].ID, ExpectationID: "missing",
	}); err == nil {
		t.Fatal("expected an error for an unknown expectation id")
	}
}

func TestTransitionResearchStageSucceedsWithAFrozenExpectationAndIndependentEvidencePlanned(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "policy", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p1", "a1", now, []*domain.Insight{
		{
			ID: "h1", Title: "Policy effect", Expectation: "registrations rise after the subsidy starts",
			ExpectationBasis: domain.ExpectationPrior, FalsificationCriteria: []string{"registrations flat or falling"},
			ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified,
		},
	})
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "q"})
	if err != nil {
		t.Fatal(err)
	}
	iterationID := run.Iterations[0].ID
	expectationID := run.Iterations[0].Expectations[0].ID

	if _, err := app.FreezeResearchExpectation(ctx, FreezeResearchExpectationInput{
		RunID: run.ID, IterationID: iterationID, ExpectationID: expectationID,
	}); err != nil {
		t.Fatal(err)
	}

	updated, err := app.TransitionResearchStage(ctx, TransitionResearchStageInput{
		RunID: run.ID, IterationID: iterationID, TargetStage: domain.StageValidation, IndependentEvidencePlanned: true,
	})
	if err != nil {
		t.Fatalf("expected the transition to succeed with a persisted frozen expectation: %v", err)
	}
	if updated.Stage != domain.StageValidation {
		t.Fatalf("expected stage VALIDATION, got %s", updated.Stage)
	}

	persisted, err := app.GetResearchRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.CurrentStage() != domain.StageValidation {
		t.Fatalf("stage transition must be persisted: %+v", persisted.Iterations)
	}
}

func TestAppendResearchIterationCarriesFrozenExpectationIntoTheNextIterationAsValidationTarget(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	first := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)
	app.now = func() time.Time { return first }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "policy", CreatedAt: first}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p1", "a1", first, []*domain.Insight{
		{
			ID: "h1", Title: "Policy effect", Expectation: "registrations rise after the subsidy starts",
			ExpectationBasis: domain.ExpectationPrior, FalsificationCriteria: []string{"registrations flat or falling"},
			ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified,
		},
	})
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "q"})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := app.FreezeResearchExpectation(ctx, FreezeResearchExpectationInput{
		RunID: run.ID, IterationID: run.Iterations[0].ID, ExpectationID: run.Iterations[0].Expectations[0].ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	app.now = func() time.Time { return second }
	seedAnalysis(t, app, ctx, "p1", "a2", second, []*domain.Insight{
		{ID: "h1b", Title: "Policy effect", ValidationStatus: domain.ValidationSupported, IdentificationStatus: domain.IdentificationNotIdentified},
	})

	got, err := app.AppendResearchIteration(ctx, AppendResearchIterationInput{RunID: run.ID, AddedEvidence: []string{"independent comparison series"}})
	if err != nil {
		t.Fatal(err)
	}

	var carried *domain.Expectation
	for i := range got.Expectations {
		if got.Expectations[i].DerivedFromExpectationID == frozen.Expectations[0].ID {
			carried = &got.Expectations[i]
		}
	}
	if carried == nil {
		t.Fatalf("expected the frozen expectation to be carried into the new iteration: %+v", got.Expectations)
	}
	if carried.FrozenForValidation {
		t.Fatal("carried expectation must start unfrozen so it becomes this iteration's validation target")
	}
	if carried.Provenance != domain.ExpectationDerivedFromPriorRun {
		t.Fatalf("carried expectation must be marked DERIVED_FROM_PRIOR_RUN: %+v", carried)
	}
}

func TestTransitionResearchStageAllowsAnExplicitBackwardMoveAndPersistsIt(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "policy", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p1", "a1", now, []*domain.Insight{
		{ID: "h1", Title: "Policy effect", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified},
	})
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "q"})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := app.TransitionResearchStage(ctx, TransitionResearchStageInput{
		RunID: run.ID, IterationID: run.Iterations[0].ID, TargetStage: domain.StageDiscovery,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Stage != domain.StageDiscovery {
		t.Fatalf("expected the iteration to move back to DISCOVERY, got %s", updated.Stage)
	}

	persisted, err := app.GetResearchRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.CurrentStage() != domain.StageDiscovery {
		t.Fatalf("stage transition was not persisted: %+v", persisted.Iterations)
	}
}

func TestTransitionResearchStageRejectsAnEarlierIteration(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "policy", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p1", "a1", now, []*domain.Insight{
		{ID: "h1", Title: "Policy effect", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified},
	})
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "q"})
	if err != nil {
		t.Fatal(err)
	}
	firstIterationID := run.Iterations[0].ID
	if _, err := app.AppendResearchIteration(ctx, AppendResearchIterationInput{RunID: run.ID}); err != nil {
		t.Fatal(err)
	}

	if _, err := app.TransitionResearchStage(ctx, TransitionResearchStageInput{
		RunID: run.ID, IterationID: firstIterationID, TargetStage: domain.StageDiscovery,
	}); err == nil || !strings.Contains(err.Error(), "latest") {
		t.Fatalf("expected an error naming the latest-iteration rule, got %v", err)
	}
}

func TestGetHumanHandoffSummarizesTheLatestIteration(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "policy", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p1", "a1", now, []*domain.Insight{
		{ID: "h1", Title: "Policy effect", ValidationStatus: domain.ValidationSupported, IdentificationStatus: domain.IdentificationNotIdentified},
	})
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "did the policy cause it?"})
	if err != nil {
		t.Fatal(err)
	}

	handoff, err := app.GetHumanHandoff(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if handoff.ResearchRunID != run.ID || handoff.Question != "did the policy cause it?" || handoff.IterationCount != 1 {
		t.Fatalf("handoff must identify the run it summarizes: %+v", handoff)
	}
	if handoff.Readiness != domain.ReadinessExploratoryOnly {
		t.Fatalf("handoff must reflect the computed readiness: %+v", handoff)
	}
}
