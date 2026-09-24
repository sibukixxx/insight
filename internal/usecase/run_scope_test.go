package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/buildinfo"
	"insight-lab/internal/domain"
	"insight-lab/internal/service"
)

// seedRunScopeProject creates project p1 with two completed analyses (a1
// older, a2 newer), each carrying one insight, one pattern and its own
// provenance, so run-scoped reads can be checked for leakage between runs.
func seedRunScopeProject(t *testing.T) (*Application, context.Context, time.Time) {
	t.Helper()
	app, ctx := newResearchTestApp(t)
	base := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return base.Add(24 * time.Hour) }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "runs", CreatedAt: base}); err != nil {
		t.Fatal(err)
	}
	for i, run := range []struct{ id, model string }{{"a1", "model-one"}, {"a2", "model-two"}} {
		at := base.Add(time.Duration(i) * time.Hour)
		seedAnalysis(t, app, ctx, "p1", run.id, at, []*domain.Insight{{ID: "ins_" + run.id, Title: "Insight from " + run.id}})
		setAnalysisMetrics(t, app, ctx, run.id, at.Add(time.Minute), service.RunProvenance{
			Mode: service.ExecutionModeModelBacked, Model: run.model, PromptFingerprint: "fp_" + run.id, RuleVersion: "dataset-preanalysis/v1",
		})
		if err := app.repos.Patterns.CreateBatch(ctx, []*domain.Pattern{{ID: "pat_" + run.id, ProjectID: "p1", AnalysisID: run.id, Title: "Pattern from " + run.id, CreatedAt: at}}); err != nil {
			t.Fatal(err)
		}
	}
	return app, ctx, base
}

func setAnalysisMetrics(t *testing.T, app *Application, ctx context.Context, analysisID string, finished time.Time, prov service.RunProvenance) {
	t.Helper()
	analysis, err := app.repos.Analyses.Get(ctx, analysisID)
	if err != nil {
		t.Fatal(err)
	}
	analysis.Metrics = metricsJSON(t, prov)
	analysis.StartedAt, analysis.FinishedAt = &finished, &finished
	if err := app.repos.Analyses.Update(ctx, analysis); err != nil {
		t.Fatal(err)
	}
}

func addFailedAnalysis(t *testing.T, app *Application, ctx context.Context, id string, at time.Time) {
	t.Helper()
	if err := app.repos.Analyses.Create(ctx, &domain.Analysis{ID: id, ProjectID: "p1", Status: domain.AnalysisFailed, Error: "boom", FinishedAt: &at, CreatedAt: at}); err != nil {
		t.Fatal(err)
	}
}

func TestListInsightsDefaultsToLatestCompletedRunOnly(t *testing.T) {
	app, ctx, _ := seedRunScopeProject(t)
	got, err := app.ListInsights(ctx, "p1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "ins_a2" {
		t.Fatalf("ListInsights = %v, want only the latest completed run's insight", insightIDs(got))
	}
}

func TestListInsightsReturnsTheSelectedRun(t *testing.T) {
	app, ctx, _ := seedRunScopeProject(t)
	got, err := app.ListInsights(ctx, "p1", "a1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "ins_a1" {
		t.Fatalf("ListInsights(a1) = %v, want only ins_a1", insightIDs(got))
	}
}

func TestListPatternsReturnsOnlyTheSelectedRun(t *testing.T) {
	app, ctx, _ := seedRunScopeProject(t)
	for analysisID, want := range map[string]string{"": "pat_a2", "a1": "pat_a1"} {
		got, err := app.ListPatterns(ctx, "p1", analysisID)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Pattern.ID != want {
			t.Fatalf("ListPatterns(%q) returned %d patterns, want only %s", analysisID, len(got), want)
		}
	}
}

func TestListInsightsFallsBackToPreviousCompletedRunWhenLatestFailed(t *testing.T) {
	app, ctx, base := seedRunScopeProject(t)
	addFailedAnalysis(t, app, ctx, "a3", base.Add(5*time.Hour))
	got, err := app.ListInsights(ctx, "p1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "ins_a2" {
		t.Fatalf("ListInsights = %v, want the previous completed run a2", insightIDs(got))
	}
}

func TestListInsightsIsEmptyWhenNoRunHasCompleted(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "empty", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	got, err := app.ListInsights(ctx, "p1", "")
	if err != nil || len(got) != 0 {
		t.Fatalf("ListInsights = %v, %v, want an empty list and no error", got, err)
	}
}

func TestRunScopedReadsRejectAnAnalysisFromAnotherProject(t *testing.T) {
	app, ctx, base := seedRunScopeProject(t)
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p2", Name: "other", CreatedAt: base}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ListInsights(ctx, "p2", "a1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ListInsights(p2, a1) err = %v, want ErrNotFound", err)
	}
	if _, err := app.ListPatterns(ctx, "p2", "a1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ListPatterns(p2, a1) err = %v, want ErrNotFound", err)
	}
	if _, err := app.ResolveAnalysis(ctx, "p2", "a1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ResolveAnalysis(p2, a1) err = %v, want ErrNotFound", err)
	}
	if _, err := app.ExportProjectMarkdown(ctx, "p2", "a1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ExportProjectMarkdown(p2, a1) err = %v, want ErrNotFound", err)
	}
}

func TestResolveAnalysisReportsNoCompletedRun(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "empty", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ResolveAnalysis(ctx, "p1", ""); !errors.Is(err, ErrNoCompletedAnalysis) {
		t.Fatalf("err = %v, want ErrNoCompletedAnalysis", err)
	}
}

func TestProjectReportBindsInsightsMetricsAndProvenanceToOneRun(t *testing.T) {
	app, ctx, base := seedRunScopeProject(t)
	addFailedAnalysis(t, app, ctx, "a3", base.Add(5*time.Hour))

	latest, err := app.ExportProjectMarkdown(ctx, "p1", "")
	if err != nil {
		t.Fatal(err)
	}
	assertReportRun(t, string(latest), "a2", "a1")

	selected, err := app.ExportProjectMarkdown(ctx, "p1", "a1")
	if err != nil {
		t.Fatal(err)
	}
	assertReportRun(t, string(selected), "a1", "a2")
}

func assertReportRun(t *testing.T, got, want, other string) {
	t.Helper()
	for _, s := range []string{"Analysis ID: `" + want + "`", "Status: `completed`", "Insight from " + want, "fp_" + want} {
		if !strings.Contains(got, s) {
			t.Errorf("report for %s is missing %q:\n%s", want, s, got)
		}
	}
	for _, s := range []string{"Insight from " + other, "fp_" + other, "Analysis ID: `" + other + "`"} {
		if strings.Contains(got, s) {
			t.Errorf("report for %s leaks %q from run %s", want, s, other)
		}
	}
}

func TestProjectReportMarksMissingProvenanceAsNotRecorded(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	at := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "legacy", CreatedAt: at}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p1", "legacy", at, []*domain.Insight{{ID: "ins_legacy", Title: "Legacy insight"}})

	got, err := app.ExportProjectMarkdown(ctx, "p1", "")
	if err != nil {
		t.Fatal(err)
	}
	report := string(got)
	for _, s := range []string{"Analysis ID: `legacy`", "Mode: not recorded", "Model: not recorded", "Prompt fingerprint (v1): not recorded", "Rule version: not recorded",
		"Engine version: not recorded", "Git commit: not recorded", "Execution fingerprint: not recorded", "Input fingerprint: not recorded"} {
		if !strings.Contains(report, s) {
			t.Errorf("legacy report is missing %q:\n%s", s, report)
		}
	}
}

func TestResearchReportAndArtifactBindToTheSameRunAfterALaterAnalysis(t *testing.T) {
	app, ctx, base := seedRunScopeProject(t)
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "what changed?"})
	if err != nil {
		t.Fatal(err)
	}
	// A newer completed analysis must not rewrite the existing iteration's report.
	at := base.Add(10 * time.Hour)
	seedAnalysis(t, app, ctx, "p1", "a9", at, []*domain.Insight{{ID: "ins_a9", Title: "Insight from a9"}})
	setAnalysisMetrics(t, app, ctx, "a9", at.Add(time.Minute), service.RunProvenance{Mode: service.ExecutionModeModelBacked, Model: "model-nine", PromptFingerprint: "fp_a9", RuleVersion: "dataset-preanalysis/v1"})

	report, err := app.ExportResearchMarkdown(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertReportRun(t, string(report), "a2", "a9")

	artifact, err := app.GetResearchArtifact(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.PromptFingerprint != "fp_a2" || artifact.ModelVersion != "model-two" {
		t.Fatalf("artifact provenance = %q/%q, want the iteration's run a2", artifact.ModelVersion, artifact.PromptFingerprint)
	}
}

func TestResearchReportFailsClosedWhenIterationInsightsSpanRuns(t *testing.T) {
	app, ctx, base := seedRunScopeProject(t)
	run := &domain.ResearchRun{ID: "run_mixed", ProjectID: "p1", Question: "mixed", CreatedAt: base,
		Iterations: []domain.ResearchIteration{{ID: "it_1", Sequence: 1, Question: "mixed", InsightIDs: []string{"ins_a1", "ins_a2"}, CreatedAt: base}}}
	if err := app.repos.Research.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ExportResearchMarkdown(ctx, run.ID); !errors.Is(err, ErrMixedAnalysisRuns) {
		t.Fatalf("err = %v, want ErrMixedAnalysisRuns", err)
	}
}

func insightIDs(list []*domain.Insight) []string {
	out := make([]string, 0, len(list))
	for _, i := range list {
		out = append(out, i.ID)
	}
	return out
}

func TestResearchIterationRecordsTheAnalysisItWasBuiltFrom(t *testing.T) {
	app, ctx, _ := seedRunScopeProject(t)
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "q"})
	if err != nil {
		t.Fatal(err)
	}
	if got := run.Iterations[0].AnalysisID; got != "a2" {
		t.Fatalf("iteration analysisId = %q, want a2", got)
	}
}

func TestResearchArtifactCarriesTheRunSnapshotsAdditively(t *testing.T) {
	app, ctx, _ := seedRunScopeProject(t)
	analysis, err := app.repos.Analyses.Get(ctx, "a2")
	if err != nil {
		t.Fatal(err)
	}
	analysis.ExecutionSnapshot = `{"engineVersion":"v0.9.0","executionFingerprint":"sha256:exec"}`
	analysis.InputSnapshot = `{"documentCount":3,"inputFingerprint":"sha256:input"}`
	analysis.ExecutionFingerprint, analysis.InputFingerprint = "sha256:exec", "sha256:input"
	if err := app.repos.Analyses.Update(ctx, analysis); err != nil {
		t.Fatal(err)
	}
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "q"})
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := app.GetResearchArtifact(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.AnalysisID != "a2" || artifact.Provenance == nil ||
		string(artifact.Provenance.Execution) != analysis.ExecutionSnapshot || string(artifact.Provenance.Input) != analysis.InputSnapshot {
		t.Fatalf("artifact run binding = %q %+v", artifact.AnalysisID, artifact.Provenance)
	}
	if artifact.PromptFingerprint != "fp_a2" || artifact.ModelVersion != "model-two" {
		t.Fatalf("existing v1 provenance keys changed: %q/%q", artifact.ModelVersion, artifact.PromptFingerprint)
	}
}

func TestResearchArtifactOmitsSnapshotsForALegacyRun(t *testing.T) {
	app, ctx, _ := seedRunScopeProject(t)
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "q"})
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := app.GetResearchArtifact(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.AnalysisID != "a2" || artifact.Provenance != nil {
		t.Fatalf("a run without snapshots must not export empty ones: %q %+v", artifact.AnalysisID, artifact.Provenance)
	}
}

func TestIterationBoundToOneAnalysisRejectsAnInsightFromAnother(t *testing.T) {
	app, ctx, base := seedRunScopeProject(t)
	run := &domain.ResearchRun{ID: "run_guard", ProjectID: "p1", Question: "guard", CreatedAt: base,
		Iterations: []domain.ResearchIteration{{ID: "it_1", Sequence: 1, Question: "guard", AnalysisID: "a2", InsightIDs: []string{"ins_a1"}, CreatedAt: base}}}
	if err := app.repos.Research.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ExportResearchMarkdown(ctx, run.ID); !errors.Is(err, ErrMixedAnalysisRuns) {
		t.Fatalf("err = %v, want ErrMixedAnalysisRuns", err)
	}
}

func TestProjectReportShowsTheRunSnapshot(t *testing.T) {
	app, ctx, _ := seedRunScopeProject(t)
	analysis, err := app.repos.Analyses.Get(ctx, "a2")
	if err != nil {
		t.Fatal(err)
	}
	execution, err := service.BuildExecutionSnapshot(service.Settings{BaseURL: "https://api.example.com/v1", Model: "model-two", APIKey: "sk-report-secret"}, "", buildinfo.Info{Version: "v0.9.0", Commit: "abc123", Dirty: "true"}, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	input := service.BuildInputSnapshot([]*domain.Document{{ID: "d1", Source: domain.SourceInterview, Content: "x"}}, time.Unix(1, 0))
	executionJSON, _ := json.Marshal(execution)
	inputJSON, _ := json.Marshal(input)
	analysis.ExecutionSnapshot, analysis.InputSnapshot = string(executionJSON), string(inputJSON)
	analysis.ExecutionFingerprint, analysis.InputFingerprint = execution.ExecutionFingerprint, input.InputFingerprint
	if err := app.repos.Analyses.Update(ctx, analysis); err != nil {
		t.Fatal(err)
	}

	got, err := app.ExportProjectMarkdown(ctx, "p1", "a2")
	if err != nil {
		t.Fatal(err)
	}
	report := string(got)
	for _, want := range []string{
		"- Engine version: `v0.9.0`",
		"- Git commit: `abc123` (uncommitted changes)",
		"- Execution fingerprint: `" + execution.ExecutionFingerprint + "`",
		"- Prompt version: `prompts/v1` (fingerprint v2 `" + execution.PromptFingerprint + "`)",
		"- Provider host: `api.example.com`",
		"- Input fingerprint: `" + input.InputFingerprint + "` (1 documents)",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report is missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "sk-report-secret") || strings.Contains(report, "https://api.example.com") {
		t.Fatalf("report leaks a credential or the full provider URL:\n%s", report)
	}
}
