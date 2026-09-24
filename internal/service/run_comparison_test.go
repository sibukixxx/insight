package service

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
)

func execSnapshot(t *testing.T, commit, model string) (string, string) {
	t.Helper()
	cfg := ExecutionConfig{EngineVersion: "v1", GitCommit: commit, ExecutionMode: ExecutionModeDeterministic, RuleVersions: map[string]string{"grounding": "g1"}}
	if model != "" {
		cfg.ExecutionMode = ExecutionModeModelBacked
		cfg.LLM = &LLMExecution{ProviderHost: "api.example", Models: []ModelBinding{{Stage: "all", Provider: "p", Model: model}}}
	}
	fp, _ := Fingerprint(cfg)
	raw, _ := json.Marshal(ExecutionSnapshot{ExecutionConfig: cfg, ExecutionFingerprint: fp})
	return string(raw), fp
}

func inputSnapshot(t *testing.T, contents ...string) (string, string) {
	t.Helper()
	var docs []*domain.Document
	for i, c := range contents {
		docs = append(docs, &domain.Document{ID: string(rune('a' + i)), Source: domain.SourceInterview, Content: c})
	}
	s := BuildInputSnapshot(docs, time.Unix(0, 0))
	raw, _ := json.Marshal(s)
	return string(raw), s.InputFingerprint
}

func run(t *testing.T, id, commit, model string, contents ...string) *domain.Analysis {
	a := &domain.Analysis{ID: id, Status: domain.AnalysisCompleted, Metrics: `{"finalInsightCount":2,"evidenceCoverage":0.5}`}
	a.ExecutionSnapshot, a.ExecutionFingerprint = execSnapshot(t, commit, model)
	a.InputSnapshot, a.InputFingerprint = inputSnapshot(t, contents...)
	return a
}

func TestCompareAnalysisRunsAttribution(t *testing.T) {
	legacy := &domain.Analysis{ID: "legacy", Status: domain.AnalysisCompleted}
	tests := []struct {
		name string
		a, b *domain.Analysis
		want RunAttribution
	}{
		{"same config is SAME_CONFIGURATION", run(t, "a", "c1", "m1", "x"), run(t, "b", "c1", "m1", "x"), AttributionSameConfiguration},
		{"model only is EXECUTION_CHANGE", run(t, "a", "c1", "m1", "x"), run(t, "b", "c1", "m2", "x"), AttributionExecutionChange},
		{"code only is EXECUTION_CHANGE", run(t, "a", "c1", "", "x"), run(t, "b", "c2", "", "x"), AttributionExecutionChange},
		{"input only is INPUT_CHANGE", run(t, "a", "c1", "m1", "x"), run(t, "b", "c1", "m1", "x", "y"), AttributionInputChange},
		{"both is CONFOUNDED", run(t, "a", "c1", "m1", "x"), run(t, "b", "c2", "m1", "y"), AttributionConfounded},
		{"legacy snapshot is ATTRIBUTION_UNAVAILABLE", legacy, run(t, "b", "c1", "m1", "x"), AttributionUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := CompareAnalysisRuns(RunComparisonInput{Analysis: tt.a}, RunComparisonInput{Analysis: tt.b}, nil)
			if c.Attribution != tt.want {
				t.Fatalf("attribution = %s, want %s (input %s, execution %s)", c.Attribution, tt.want, c.Input.State, c.Execution.State)
			}
			if c.Explanation[0] != RunComparisonNonCausalNote {
				t.Fatalf("explanation must start with the non-causal note: %v", c.Explanation)
			}
		})
	}
}

func TestCompareAnalysisRunsReturnsFieldLevelExecutionAndInputDiff(t *testing.T) {
	a, b := run(t, "a", "c1", "m1", "x"), run(t, "b", "c1", "m2", "x", "y")
	c := CompareAnalysisRuns(RunComparisonInput{Analysis: a}, RunComparisonInput{Analysis: b}, nil)
	want := []FieldChange{{Field: "models.all", From: "p/m1", To: "p/m2"}}
	if !reflect.DeepEqual(c.Execution.Changes, want) {
		t.Fatalf("execution changes = %+v", c.Execution.Changes)
	}
	if len(c.Input.DocumentsAdded) != 1 || len(c.Input.DocumentsRemoved) != 0 {
		t.Fatalf("input diff = %+v", c.Input)
	}
}

func TestCompareAnalysisRunsReportsRepeatGroupRangeAndUnknownVariationForSingleRun(t *testing.T) {
	a, b, c2 := run(t, "a", "c1", "m1", "x"), run(t, "b", "c1", "m1", "x"), run(t, "c", "c2", "m1", "x")
	b.Metrics = `{"finalInsightCount":4,"evidenceCoverage":0.5}`
	c := CompareAnalysisRuns(RunComparisonInput{Analysis: a}, RunComparisonInput{Analysis: c2}, []*domain.Analysis{a, b, c2})
	if len(c.RepeatGroups) != 2 {
		t.Fatalf("groups = %+v", c.RepeatGroups)
	}
	g := c.RepeatGroups[0]
	if g.Runs != 2 || !g.VariationKnown || !reflect.DeepEqual(g.Metrics[1], MetricRange{Metric: "finalInsightCount", Min: 2, Max: 4}) {
		t.Fatalf("first group = %+v", g)
	}
	if c.RepeatGroups[1].VariationKnown || !strings.Contains(strings.Join(c.Explanation, " "), "variation is unknown") {
		t.Fatalf("single-run group must report unknown variation: %+v %v", c.RepeatGroups[1], c.Explanation)
	}
}

func insight(id, title string, spans ...[3]any) *domain.Insight {
	in := &domain.Insight{ID: id, Title: title, ValidationStatus: domain.ValidationPlausible}
	for _, s := range spans {
		in.Evidence = append(in.Evidence, domain.Evidence{DocumentID: s[0].(string), StartOffset: s[1].(int), EndOffset: s[2].(int)})
	}
	return in
}

func TestMatchInsightsUsesEvidenceNotRawIDs(t *testing.T) {
	from := RunComparisonInput{Analysis: run(t, "a", "c1", "", "x"), Insights: []*domain.Insight{
		insight("old-1", "Price sensitivity", [3]any{"d1", 0, 10}),
		insight("old-2", "Onboarding friction", [3]any{"d2", 0, 20}),
		insight("old-3", "Seasonality", [3]any{"d3", 0, 5}),
	}}
	to := RunComparisonInput{Analysis: run(t, "b", "c1", "", "x"), Insights: []*domain.Insight{
		insight("new-9", "Price sensitivity", [3]any{"d1", 0, 10}),
		insight("new-8", "Onboarding friction (revised)", [3]any{"d2", 5, 20}),
		insight("new-7", "Brand trust", [3]any{"d4", 0, 5}),
	}}
	got := matchInsights(from, to)
	want := InsightResultDiff{
		Added: []string{"new-7"}, Removed: []string{"old-3"},
		Matched: []InsightMatch{
			{FromInsightID: "old-1", ToInsightID: "new-9", Method: MatchExactSpans, Score: 1, Changes: []FieldChange{}},
			{FromInsightID: "old-2", ToInsightID: "new-8", Method: MatchSpanOverlap, Score: 0.75, Changes: []FieldChange{{Field: "title", From: "Onboarding friction", To: "Onboarding friction (revised)"}}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("matches =\n%+v\nwant\n%+v", got, want)
	}
	if again := matchInsights(from, to); !reflect.DeepEqual(again, got) {
		t.Fatal("matching must be stable for identical inputs")
	}
}

func TestRunComparisonHasNoWinnerOrRankingField(t *testing.T) {
	c := CompareAnalysisRuns(RunComparisonInput{Analysis: run(t, "a", "c1", "", "x")}, RunComparisonInput{Analysis: run(t, "b", "c2", "", "x")}, nil)
	raw, _ := json.Marshal(c)
	lower := strings.ToLower(string(raw))
	for _, banned := range []string{`"winner`, `"better`, `"best`, `"improved`, `"rank`, `"verdict`} {
		if strings.Contains(lower, banned) {
			t.Fatalf("comparison contains a judgement key %s: %s", banned, raw)
		}
	}
}
