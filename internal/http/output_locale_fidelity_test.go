package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/usecase"
)

// #125 fidelity invariants: requesting ja-JP instead of en-US may change the
// language of generated text, never the evidence or the research state. The
// same evidence must yield identical quotes, offsets, findings, statuses,
// uncertainty and limitations in both locales. The scripted stand-in model
// follows the locale instruction, so this runs without a paid LLM.

const fidelityEvidence = "The observed measure rose from 120 to 150 units in 2024-03.\nA second site reported 45 units, which contradicts the first trend."

var japaneseScript = regexp.MustCompile(`[\p{Hiragana}\p{Katakana}\p{Han}]`)

type localeRun struct {
	Results struct {
		Observations []struct {
			Quote       string `json:"quote"`
			StartOffset int    `json:"startOffset"`
			EndOffset   int    `json:"endOffset"`
			Behavior    string `json:"behavior"`
		} `json:"observations"`
		Findings []struct {
			Kind           string   `json:"kind"`
			ObservationIDs []string `json:"observationIds"`
		} `json:"findings"`
	}
	Artifact usecase.ResearchArtifact
}

func publicCall(t *testing.T, method, url string, body any, out any) int {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, url, reader)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatalf("%s %s: decode: %v", method, url, err)
		}
	}
	return res.StatusCode
}

func runWithLocale(t *testing.T, base, locale string) localeRun {
	t.Helper()
	api := base + "/api/public/v1"
	var subject struct {
		SubjectID string `json:"subjectId"`
	}
	publicCall(t, http.MethodPost, api+"/subjects", map[string]any{"contractVersion": "1", "idempotencyKey": "s-" + locale, "subject": map[string]string{"namespace": "fidelity", "id": locale}}, &subject)
	publicCall(t, http.MethodPost, api+"/subjects/"+subject.SubjectID+"/evidence", map[string]any{"contractVersion": "1", "idempotencyKey": "e-" + locale,
		"documents": []map[string]string{{"externalRef": "e1", "source": "document", "content": fidelityEvidence}}}, nil)
	var run struct {
		AnalysisID string `json:"analysisId"`
		Status     string `json:"status"`
	}
	if code := publicCall(t, http.MethodPost, api+"/subjects/"+subject.SubjectID+"/analyses", map[string]any{"contractVersion": "1", "idempotencyKey": "a-" + locale,
		"researchQuestion": "What explanations are consistent with the observed change?", "outputLocale": locale}, &run); code != http.StatusAccepted {
		t.Fatalf("%s: start analysis: %d", locale, code)
	}
	deadline := time.Now().Add(20 * time.Second)
	for run.Status != "completed" {
		if run.Status == "failed" || time.Now().After(deadline) {
			t.Fatalf("%s: analysis did not complete: %s", locale, run.Status)
		}
		time.Sleep(20 * time.Millisecond)
		publicCall(t, http.MethodGet, api+"/subjects/"+subject.SubjectID+"/analyses/"+run.AnalysisID, nil, &run)
	}
	var out localeRun
	publicCall(t, http.MethodGet, api+"/subjects/"+subject.SubjectID+"/analyses/"+run.AnalysisID+"/results", nil, &out.Results)
	var research struct {
		Artifact usecase.ResearchArtifact `json:"artifact"`
	}
	if code := publicCall(t, http.MethodPost, api+"/subjects/"+subject.SubjectID+"/research-runs", map[string]any{"contractVersion": "1", "idempotencyKey": "r-" + locale,
		"question": "What explanations are consistent with the observed change?", "analysisId": run.AnalysisID}, &research); code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("%s: create research run: %d", locale, code)
	}
	out.Artifact = research.Artifact
	return out
}

// structure keeps everything that must not depend on the output locale.
func structure(r localeRun) string {
	var b strings.Builder
	// The API does not promise an observation order; compare by offset.
	obs := append(r.Results.Observations[:0:0], r.Results.Observations...)
	sort.Slice(obs, func(i, j int) bool { return obs[i].StartOffset < obs[j].StartOffset })
	for _, o := range obs {
		fmt.Fprintf(&b, "obs %q %d-%d\n", o.Quote, o.StartOffset, o.EndOffset)
	}
	for _, f := range r.Results.Findings {
		fmt.Fprintf(&b, "finding %s %d\n", f.Kind, len(f.ObservationIDs))
	}
	for _, i := range r.Artifact.Insights {
		var warnings []string
		for _, w := range i.QualityWarnings {
			warnings = append(warnings, string(w.Code))
		}
		var support, counter []string
		for _, e := range i.SupportingEvidence {
			support = append(support, e.Quote)
		}
		for _, e := range i.CounterEvidence {
			counter = append(counter, e.Quote)
		}
		fmt.Fprintf(&b, "insight role=%s causal=%s validation=%s identification=%s basis=%s missing=%d falsification=%d competing=%d warnings=%v support=%q counter=%q\n",
			i.HypothesisRole, i.CausalStatus, i.ValidationStatus, i.IdentificationStatus, i.ExpectationBasis,
			len(i.MissingEvidence), len(i.FalsificationCriteria), len(i.CompetingHypotheses), warnings, support, counter)
	}
	fmt.Fprintf(&b, "gaps=%d requirements=%d cannotConclude=%d readiness=%s\n",
		len(r.Artifact.ResearchGaps), len(r.Artifact.NextDataRequirements), len(r.Artifact.WhatWeCannotConclude), r.Artifact.EffectiveReadiness)
	return b.String()
}

func TestOutputLocaleChangesGeneratedLanguageButNotEvidenceOrResearchState(t *testing.T) {
	server := newPublicServer(t, true)
	ja := runWithLocale(t, server.URL, "ja-JP")
	en := runWithLocale(t, server.URL, "en-US")

	if len(ja.Results.Observations) == 0 || len(ja.Artifact.Insights) == 0 {
		t.Fatalf("scripted run produced no research state: %+v", ja)
	}
	for _, o := range ja.Results.Observations {
		if o.Quote != fidelityEvidence[o.StartOffset:o.EndOffset] {
			t.Fatalf("ja-JP quote is not the verbatim source span: %q vs %q", o.Quote, fidelityEvidence[o.StartOffset:o.EndOffset])
		}
		if japaneseScript.MatchString(o.Quote) {
			t.Fatalf("ja-JP translated a quote: %q", o.Quote)
		}
	}
	if got, want := structure(ja), structure(en); got != want {
		t.Fatalf("locale changed evidence or research state:\n--- ja-JP ---\n%s--- en-US ---\n%s", got, want)
	}
	if !japaneseScript.MatchString(ja.Artifact.Insights[0].LatentNeed) || japaneseScript.MatchString(en.Artifact.Insights[0].LatentNeed) {
		t.Fatalf("generated text did not follow the locale: ja=%q en=%q", ja.Artifact.Insights[0].LatentNeed, en.Artifact.Insights[0].LatentNeed)
	}
}
