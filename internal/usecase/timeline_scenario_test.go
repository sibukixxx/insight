package usecase

import (
	"testing"
	"time"

	"insight-lab/internal/domain"
)

// #71: scenario expectations (#66) evaluated by later observations appear on
// the timeline as their own lane, never merged into evidence or instrument.
func TestResearchTimelineIncludesScenarioEvaluationLane(t *testing.T) {
	app, ctx, run := startReEvaluationRun(t)
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	set := &domain.ScenarioSet{ID: "set-1", ResearchRunID: run.ID, Version: 1, Question: "q", Origin: domain.ScenarioOriginHuman,
		Scenarios: []domain.Scenario{{ID: "s1", Title: "s1"}}}
	if err := app.repos.Scenarios.CreateScenarioSet(ctx, set); err != nil {
		t.Fatal(err)
	}
	eval := &domain.ScenarioEvaluation{ID: "eval-1", ScenarioSetID: "set-1", SetVersion: 1, ResearchRunID: run.ID, IterationID: run.Iterations[0].ID,
		EvaluatedAt: at, Delta: domain.ScenarioDelta{ToEvaluationID: "eval-1",
			Weakened:            []domain.ScenarioStatusChange{{ScenarioID: "s1", From: domain.ScenarioStatus("UNTESTED"), To: domain.ScenarioStatus("WEAKENED")}},
			FalsificationsFired: []domain.FiredFalsification{{ScenarioID: "s1", ExpectationID: "e1", Condition: "exports fall", EvidenceRef: "doc-9"}}}}
	if err := app.repos.Scenarios.CreateScenarioEvaluation(ctx, eval); err != nil {
		t.Fatal(err)
	}
	tl, err := app.GetResearchTimeline(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tl.ScenarioEvents) != 1 {
		t.Fatalf("scenario lane = %+v", tl.ScenarioEvents)
	}
	ev := tl.ScenarioEvents[0]
	if ev.EvaluationID != "eval-1" || len(ev.Weakened) != 1 || len(ev.FalsificationsFired) != 1 || !ev.EvaluatedAt.Equal(at) {
		t.Fatalf("scenario event = %+v", ev)
	}
	for _, e := range tl.EvidenceEvents {
		if e.Reference == "doc-9" {
			t.Fatal("scenario evidence must not be merged into the evidence lane")
		}
	}
}
