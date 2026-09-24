package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"insight-lab/internal/buildinfo"
	"insight-lab/internal/domain"
	"insight-lab/internal/service"
)

// setRunIdentity records an execution snapshot (for the given model) and an
// input snapshot (over the given contents) on an existing analysis.
func setRunIdentity(t *testing.T, app *Application, ctx context.Context, analysisID, model string, contents ...string) {
	t.Helper()
	a, err := app.repos.Analyses.Get(ctx, analysisID)
	if err != nil {
		t.Fatal(err)
	}
	exec, err := service.BuildExecutionSnapshot(service.Settings{BaseURL: "https://api.example/v1", Model: model}, "", buildInfoForTest(), time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	var docs []*domain.Document
	for i, c := range contents {
		docs = append(docs, &domain.Document{ID: string(rune('a' + i)), Source: domain.SourceInterview, Content: c})
	}
	in := service.BuildInputSnapshot(docs, time.Unix(0, 0))
	a.ExecutionSnapshot, a.ExecutionFingerprint = mustJSON(t, exec), exec.ExecutionFingerprint
	a.InputSnapshot, a.InputFingerprint = mustJSON(t, in), in.InputFingerprint
	if err := app.repos.Analyses.Update(ctx, a); err != nil {
		t.Fatal(err)
	}
}

func TestCompareAnalysesClassifiesExecutionChangeForTwoRunsOfOneProject(t *testing.T) {
	app, ctx, _ := seedRunScopeProject(t)
	setRunIdentity(t, app, ctx, "a1", "model-one", "same evidence")
	setRunIdentity(t, app, ctx, "a2", "model-two", "same evidence")

	c, err := app.CompareAnalyses(ctx, "p1", "a1", "a2")
	if err != nil {
		t.Fatal(err)
	}
	if c.Attribution != service.AttributionExecutionChange || c.Input.State != service.AxisSame {
		t.Fatalf("attribution %s input %s execution %s", c.Attribution, c.Input.State, c.Execution.State)
	}
	if !slices.ContainsFunc(c.Execution.Changes, func(f service.FieldChange) bool { return f.Field == "models.all" }) {
		t.Fatalf("model change missing from %+v", c.Execution.Changes)
	}
	if !slices.Equal(c.Insights.Removed, []string{"ins_a1"}) || !slices.Equal(c.Insights.Added, []string{"ins_a2"}) {
		t.Fatalf("insight diff %+v", c.Insights)
	}
}

func TestCompareAnalysesReportsUnavailableAttributionForLegacyRuns(t *testing.T) {
	app, ctx, _ := seedRunScopeProject(t)
	c, err := app.CompareAnalyses(ctx, "p1", "a1", "a2")
	if err != nil {
		t.Fatal(err)
	}
	if c.Attribution != service.AttributionUnavailable {
		t.Fatalf("legacy runs must not be attributed, got %s", c.Attribution)
	}
}

func TestCompareAnalysesRejectsRunsFromAnotherProject(t *testing.T) {
	app, ctx, base := seedRunScopeProject(t)
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p2", Name: "other", CreatedAt: base}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p2", "b1", base, nil)
	if _, err := app.CompareAnalyses(ctx, "p1", "a1", "b1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CompareAnalyses across projects = %v, want ErrNotFound", err)
	}
	if _, err := app.CompareAnalysesByID(ctx, "a1", "b1"); !errors.Is(err, ErrCrossProjectComparison) {
		t.Fatalf("CompareAnalysesByID across projects = %v, want ErrCrossProjectComparison", err)
	}
}

func TestAppendResearchIterationNotesInstrumentChangeInInsightDelta(t *testing.T) {
	app, ctx, _ := seedRunScopeProject(t)
	setRunIdentity(t, app, ctx, "a1", "model-one", "same evidence")
	setRunIdentity(t, app, ctx, "a2", "model-two", "same evidence")
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "q", AnalysisID: "a1"})
	if err != nil {
		t.Fatal(err)
	}
	it, err := app.AppendResearchIteration(ctx, AppendResearchIterationInput{RunID: run.ID, AnalysisID: "a2"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(it.Delta.Explanation, InstrumentChangeNote) {
		t.Fatalf("delta explanation %v lacks the instrument change note", it.Delta.Explanation)
	}
}

func TestMetricsHistoryListsRunsOldestFirstWithFingerprints(t *testing.T) {
	app, ctx, _ := seedRunScopeProject(t)
	setRunIdentity(t, app, ctx, "a1", "model-one", "x")
	history, err := app.MetricsHistory(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[0].AnalysisID != "a1" || history[0].InputFingerprint == "" || history[1].InputFingerprint != "" {
		t.Fatalf("history = %+v", history)
	}
}

func buildInfoForTest() buildinfo.Info {
	return buildinfo.Info{Version: "v0.9.0", Commit: "abc123", Dirty: "false"}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
