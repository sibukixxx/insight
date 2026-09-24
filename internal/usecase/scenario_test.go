package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository/sqlite"
)

func newScenarioTestApp(t *testing.T) (*Application, context.Context, *domain.ResearchRun) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "scenario.db"))
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
	ctx := context.Background()
	fixed := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return fixed }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "export", CreatedAt: fixed}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p1", "a1", fixed, []*domain.Insight{{ID: "h1", Title: "FX pass-through raises quantity",
		Rationale: "exporters cut EUR prices", FalsificationCriteria: []string{"EUR unit value unchanged"},
		CompetingHypotheses: []domain.CompetingHypothesis{
			{Title: "Freight absorbs FX", FalsificationCriteria: []string{"landed price falls"}},
			{Title: "Category demand unrelated to FX", FalsificationCriteria: []string{"only Japan origin rises"}},
		}}})
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "where can Japanese exports grow?"})
	if err != nil {
		t.Fatal(err)
	}
	return app, ctx, run
}

func scaffoldInput(runID string) ScaffoldScenariosInput {
	return ScaffoldScenariosInput{RunID: runID,
		Baseline: domain.ScenarioBaseline{AsOf: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC), Description: "trade as of June 2026"},
		Horizon:  domain.ScenarioHorizon{Label: "12m", End: time.Date(2027, 6, 30, 0, 0, 0, 0, time.UTC)}}
}

func TestScaffoldScenariosBranchesOneBaselineIntoThreeScenariosFromHypotheses(t *testing.T) {
	app, ctx, run := newScenarioTestApp(t)
	res, err := app.ScaffoldScenarios(ctx, scaffoldInput(run.ID))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Set.Scenarios) != 3 || res.Set.Version != 1 || res.Set.FromIterationID != run.Iterations[0].ID {
		t.Fatalf("set = %+v", res.Set)
	}
	view, err := app.GetResearchRunView(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	scenarios := view.Iterations[0].Scenarios
	if len(scenarios) != 3 || scenarios[0].Status != domain.ScenarioUntested || scenarios[0].Kind != "SCENARIO" {
		t.Fatalf("iteration scenarios = %+v", scenarios)
	}
}

func TestScaffoldScenariosRequiresHorizon(t *testing.T) {
	app, ctx, run := newScenarioTestApp(t)
	in := scaffoldInput(run.ID)
	in.Horizon = domain.ScenarioHorizon{}
	if _, err := app.ScaffoldScenarios(ctx, in); !errors.Is(err, ErrScenarioInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestEvaluateScenariosIsAppendOnlyAndExportedInResearchArtifact(t *testing.T) {
	app, ctx, run := newScenarioTestApp(t)
	res, err := app.ScaffoldScenarios(ctx, scaffoldInput(run.ID))
	if err != nil {
		t.Fatal(err)
	}
	app.now = func() time.Time { return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) }
	first, err := app.EvaluateScenarios(ctx, EvaluateScenariosInput{RunID: run.ID, ScenarioSetID: res.Set.ID, IterationID: run.Iterations[0].ID,
		Input: domain.NewObservationInput{Observations: []domain.IndicatorObservation{
			{ExpectationID: "S2-E1", Outcome: domain.OutcomeContradicts, EvidenceRef: "comext:2026-09", ObservedAt: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)}}}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.EvaluateScenarios(ctx, EvaluateScenariosInput{RunID: run.ID, ScenarioSetID: res.Set.ID,
		Input: domain.NewObservationInput{Observations: []domain.IndicatorObservation{
			{ExpectationID: "S1-E1", Outcome: domain.OutcomeConsistent, EvidenceRef: "comext:2026-10", ObservedAt: time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)}}}})
	if err != nil {
		t.Fatal(err)
	}
	if second.Delta.FromEvaluationID != first.ID || len(second.Observations) != 2 {
		t.Fatalf("second evaluation must build on the first: %+v", second.Delta)
	}
	analysis, err := app.GetScenarioAnalysis(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(analysis.Evaluations) != 2 || analysis.Evaluations[0].States[1].Status != domain.ScenarioContradicted || analysis.CurrentEvaluation.ID != second.ID {
		t.Fatalf("history = %+v", analysis)
	}
	artifact, err := app.GetResearchArtifact(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(artifact)
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	if _, ok := decoded["scenarioAnalysis"]; !ok {
		t.Fatal("scenario analysis missing from research artifact export")
	}
	view, _ := app.GetResearchRunView(ctx, run.ID)
	if view.Iterations[0].ScenarioDelta == nil || len(view.Iterations[0].ScenarioDelta.Contradicted) != 1 {
		t.Fatalf("iteration view delta = %+v", view.Iterations[0].ScenarioDelta)
	}
}

func TestResearchArtifactOmitsScenarioAnalysisWhenRunHasNone(t *testing.T) {
	app, ctx, run := newScenarioTestApp(t)
	artifact, err := app.GetResearchArtifact(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.ScenarioAnalysis != nil {
		t.Fatal("scenarios were synthesized for a run that has none")
	}
}
