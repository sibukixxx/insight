package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
)

// An omitted output locale is not a new configuration: every run recorded
// before #125 must keep its execution fingerprint byte for byte. The pinned
// values were captured from the engine before OutputLocale existed.
func TestOutputLocaleUnsetKeepsLegacyExecutionFingerprints(t *testing.T) {
	for _, tc := range []struct {
		name     string
		settings Settings
		want     string
	}{
		{"model-backed", modelSettings(), "sha256:1cd2045eaaf7c030c067c3f05771212200c0988edd98b0415df234250cea123a"},
		{"deterministic", Settings{}, "sha256:ddaac309c3234b742b73dee252dbfa392e6d7ef6957581057a4214cf839fd218"},
	} {
		got, err := BuildExecutionSnapshotForRun(tc.settings, domain.AnalysisModeDiscovery, domain.ReasoningGeneralResearch, "", testBuild, time.Unix(1, 0))
		if err != nil {
			t.Fatal(err)
		}
		if got.ExecutionFingerprint != tc.want {
			t.Errorf("%s: fingerprint %s, want legacy %s", tc.name, got.ExecutionFingerprint, tc.want)
		}
		if raw, _ := json.Marshal(got); strings.Contains(string(raw), "outputLocale") {
			t.Errorf("%s: unset locale was serialized: %s", tc.name, raw)
		}
	}
}

// Locale is execution/presentation configuration: the same evidence under
// different locales keeps one input identity while the execution differs,
// and a comparison attributes the difference to outputLocale.
func TestOutputLocaleChangesOnlyExecutionIdentityWhenRequested(t *testing.T) {
	at := time.Unix(1, 0)
	docs := []*domain.Document{{ID: "d1", Source: domain.SourceDocument, Content: "The observed measure changed."}}
	input := BuildInputSnapshotForQuestion(docs, "What changed?", at)
	inputJSON, _ := json.Marshal(input)
	runs := map[domain.OutputLocale]*domain.Analysis{}
	for _, locale := range []domain.OutputLocale{"", domain.OutputLocaleJaJP, domain.OutputLocaleEnUS} {
		exec, err := BuildExecutionSnapshotForRun(modelSettings(), "", domain.ReasoningGeneralResearch, locale, testBuild, at)
		if err != nil {
			t.Fatal(err)
		}
		if exec.OutputLocale != locale {
			t.Fatalf("snapshot recorded %q, want %q", exec.OutputLocale, locale)
		}
		execJSON, _ := json.Marshal(exec)
		runs[locale] = &domain.Analysis{ID: "a-" + string(locale), Status: domain.AnalysisCompleted, OutputLocale: locale,
			ExecutionSnapshot: string(execJSON), ExecutionFingerprint: exec.ExecutionFingerprint,
			InputSnapshot: string(inputJSON), InputFingerprint: input.InputFingerprint, CreatedAt: at}
	}
	if runs[""].ExecutionFingerprint == runs[domain.OutputLocaleJaJP].ExecutionFingerprint ||
		runs[domain.OutputLocaleJaJP].ExecutionFingerprint == runs[domain.OutputLocaleEnUS].ExecutionFingerprint {
		t.Fatal("each requested locale must have its own execution fingerprint")
	}
	c := CompareAnalysisRuns(RunComparisonInput{Analysis: runs[""]}, RunComparisonInput{Analysis: runs[domain.OutputLocaleJaJP]}, nil)
	if c.Input.State != AxisSame || c.Execution.State != AxisChanged || c.Attribution != AttributionExecutionChange {
		t.Fatalf("locale change must be an execution change only: input=%s execution=%s attribution=%s", c.Input.State, c.Execution.State, c.Attribution)
	}
	if len(c.Execution.Changes) != 1 || c.Execution.Changes[0].Field != "outputLocale" || c.Execution.Changes[0].To != "ja-JP" {
		t.Fatalf("execution changes = %+v, want only outputLocale -> ja-JP", c.Execution.Changes)
	}
}

func TestOutputLocaleInstructionIsEmptyWhenOmitted(t *testing.T) {
	if got := outputLocaleInstruction(""); got != "" {
		t.Fatalf("omitted locale must keep the source-language rule, got %q", got)
	}
}

func TestOutputLocaleInstructionNamesLanguageAndKeepsSourceVerbatimWhenRequested(t *testing.T) {
	for locale, language := range map[domain.OutputLocale]string{domain.OutputLocaleJaJP: "Japanese", domain.OutputLocaleEnUS: "English"} {
		got := outputLocaleInstruction(locale)
		for _, want := range []string{"Output locale: " + string(locale), language, "quote", "numbers", "enum", "Do not translate"} {
			if !strings.Contains(got, want) {
				t.Errorf("%s instruction missing %q: %s", locale, want, got)
			}
		}
	}
}

func TestEveryModelStageReceivesOutputLocaleInstructionWhenRequested(t *testing.T) {
	capture := &promptCaptureClient{}
	p := &Pipeline{LLM: capture, OutputLocale: domain.OutputLocaleJaJP}
	for _, step := range pipelineLLMSteps() {
		if _, err := p.generate(context.Background(), step, nil); err != nil {
			t.Fatalf("stage %s: %v", step.Name, err)
		}
	}
	for i, prompt := range capture.prompts {
		if !strings.Contains(prompt, "Output locale: ja-JP") {
			t.Fatalf("stage %s did not receive the output locale", pipelineLLMSteps()[i].Name)
		}
	}
}

func TestEnqueueRecordsRequestedOutputLocaleAndRejectsUnsupportedOne(t *testing.T) {
	h := newJobHarness(t, datasetJobDocs(), modelSettings())
	ctx := context.Background()
	if _, err := h.jobs.Enqueue(ctx, EnqueueRequest{ProjectID: "proj_1", OutputLocale: "ja"}); err == nil {
		t.Fatal("an unsupported output locale must be rejected")
	}
	a, err := h.jobs.Enqueue(ctx, EnqueueRequest{ProjectID: "proj_1", OutputLocale: domain.OutputLocaleJaJP})
	if err != nil {
		t.Fatal(err)
	}
	var exec ExecutionSnapshot
	if err := json.Unmarshal([]byte(a.ExecutionSnapshot), &exec); err != nil {
		t.Fatal(err)
	}
	if a.OutputLocale != domain.OutputLocaleJaJP || exec.OutputLocale != domain.OutputLocaleJaJP {
		t.Fatalf("locale not recorded: analysis=%q snapshot=%q", a.OutputLocale, exec.OutputLocale)
	}
}
