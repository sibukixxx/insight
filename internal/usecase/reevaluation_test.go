package usecase

import (
	"context"
	"errors"
	"testing"

	"insight-lab/internal/domain"
)

func startReEvaluationRun(t *testing.T) (*Application, context.Context, *domain.ResearchRun) {
	t.Helper()
	app, ctx, _ := seedRunScopeProject(t)
	setRunIdentity(t, app, ctx, "a1", "model-one", "dataset v1")
	setRunIdentity(t, app, ctx, "a2", "model-one", "dataset v2")
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "q", AnalysisID: "a1"})
	if err != nil {
		t.Fatal(err)
	}
	return app, ctx, run
}

func reEvalInput(run *domain.ResearchRun, key, analysisID string) ReEvaluateInput {
	return ReEvaluateInput{
		RunID: run.ID, CorrelationKey: key, PreviousIterationID: run.Iterations[0].ID, AnalysisID: analysisID,
		Trigger:         domain.ReEvaluationTrigger{Kind: domain.ReEvaluationScheduled, Source: "nightly"},
		EvidenceChanges: domain.EvidenceChanges{Changed: []string{"dataset@v2"}},
	}
}

func TestReEvaluateChangedDatasetCreatesNewIterationWithAuditRecord(t *testing.T) {
	app, ctx, run := startReEvaluationRun(t)
	out, err := app.ReEvaluate(ctx, reEvalInput(run, "corr-1", "a2"))
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != ReEvaluationNewIteration || len(out.Run.Iterations) != 2 {
		t.Fatalf("status %s, iterations %d", out.Status, len(out.Run.Iterations))
	}
	latest := out.Run.Iterations[1]
	rec := latest.ReEvaluation
	if rec == nil || rec.PreviousIterationID != run.Iterations[0].ID || rec.Scope != domain.ReEvaluationFull ||
		rec.InputFingerprintBefore == rec.InputFingerprintAfter || rec.Trigger.Kind != domain.ReEvaluationScheduled {
		t.Fatalf("audit record = %+v", rec)
	}
	if latest.Delta == nil {
		t.Fatal("re-evaluation must produce an Insight Delta")
	}
	if first := out.Run.Iterations[0]; first.ReEvaluation != nil || first.ID != run.Iterations[0].ID {
		t.Fatal("previous iteration was modified")
	}
}

func TestReEvaluateIsIdempotentForSameCorrelationKey(t *testing.T) {
	app, ctx, run := startReEvaluationRun(t)
	if _, err := app.ReEvaluate(ctx, reEvalInput(run, "corr-1", "a2")); err != nil {
		t.Fatal(err)
	}
	again, err := app.ReEvaluate(ctx, reEvalInput(run, "corr-1", "a2"))
	if err != nil {
		t.Fatal(err)
	}
	if again.Status != ReEvaluationAlreadyEvaluated || len(again.Run.Iterations) != 2 {
		t.Fatalf("status %s, iterations %d", again.Status, len(again.Run.Iterations))
	}
	if _, err := app.ReEvaluate(ctx, reEvalInput(run, "corr-1", "a1")); !errors.Is(err, ErrReEvaluationConflict) {
		t.Fatalf("reused key for another analysis = %v", err)
	}
}

func TestReEvaluateSameInputFingerprintCreatesNoIteration(t *testing.T) {
	app, ctx, run := startReEvaluationRun(t)
	setRunIdentity(t, app, ctx, "a2", "model-two", "dataset v1") // same evidence as a1
	out, err := app.ReEvaluate(ctx, reEvalInput(run, "corr-2", "a2"))
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != ReEvaluationNoEvidenceChange || len(out.Run.Iterations) != 1 {
		t.Fatalf("status %s, iterations %d", out.Status, len(out.Run.Iterations))
	}
}

func TestReEvaluateRejectsStalePreviousIterationAndMissingFields(t *testing.T) {
	app, ctx, run := startReEvaluationRun(t)
	in := reEvalInput(run, "corr-3", "a2")
	in.PreviousIterationID = "it_old"
	if _, err := app.ReEvaluate(ctx, in); !errors.Is(err, ErrStaleIteration) {
		t.Fatalf("stale = %v", err)
	}
	in = reEvalInput(run, "", "a2")
	if _, err := app.ReEvaluate(ctx, in); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("missing key = %v", err)
	}
	in = reEvalInput(run, "k", "a2")
	in.Trigger.Kind = "CRON"
	if _, err := app.ReEvaluate(ctx, in); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("bad trigger = %v", err)
	}
}

func TestReEvaluateRecordsScenariosTouchedByAffectedGapsAndChangedEvidence(t *testing.T) {
	app, ctx, run := startReEvaluationRun(t)
	set := &domain.ScenarioSet{ID: "set-1", ResearchRunID: run.ID, Version: 1, Question: "q", Origin: domain.ScenarioOriginHuman,
		Scenarios: []domain.Scenario{
			{ID: "by-gap", Title: "gap", UnresolvedGapIDs: []string{"g-affected"}},
			{ID: "by-evidence", Title: "evidence", EvidenceRefs: []string{"dataset@v2"}},
			{ID: "untouched", Title: "other", UnresolvedGapIDs: []string{"g-other"}},
		}}
	if err := app.repos.Scenarios.CreateScenarioSet(ctx, set); err != nil {
		t.Fatal(err)
	}
	in := reEvalInput(run, "corr-scn", "a2")
	in.AffectedGapIDs = []string{"g-affected"}
	out, err := app.ReEvaluate(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	rec := out.Record
	if rec.AffectedScenarioSetID != "set-1" || len(rec.AffectedScenarioIDs) != 2 || rec.AffectedScenarioIDs[0] != "by-evidence" || rec.AffectedScenarioIDs[1] != "by-gap" {
		t.Fatalf("affected scenarios = %q in %q", rec.AffectedScenarioIDs, rec.AffectedScenarioSetID)
	}
}

func TestReEvaluateLeavesScenarioRefsEmptyWhenRunHasNoScenarios(t *testing.T) {
	app, ctx, run := startReEvaluationRun(t)
	out, err := app.ReEvaluate(ctx, reEvalInput(run, "corr-none", "a2"))
	if err != nil {
		t.Fatal(err)
	}
	if out.Record.AffectedScenarioSetID != "" || len(out.Record.AffectedScenarioIDs) != 0 {
		t.Fatalf("no scenario set exists, got %+v", out.Record)
	}
}
