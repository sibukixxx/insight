// Package scripted is a deterministic, grounded stand-in for an LLM. It is a
// test tool: it never produces judgement of its own, only structurally valid
// answers derived from the input evidence.
package scripted

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"insight-lab/internal/llm"
)

// scriptedModel is a deterministic stand-in for an LLM, used by the conformance
// tests and by cmd/insight-scripted-llm so SDK repositories can run the
// model-backed fixtures against a real engine without any LLM call. It answers each pipeline step from its input so every quote it
// returns is grounded in the evidence: each evidence line becomes an
// observation, all observations form one deviation, and one primary
// hypothesis with two alternatives names a missing comparison. Observations
// whose text says "contradicts" are returned as counter-evidence.
type Model struct{}

var mu sync.Mutex

func (Model) Generate(_ context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	mu.Lock()
	defer mu.Unlock()
	input := req.Messages[len(req.Messages)-1].Content
	var payload struct {
		Observations []struct {
			ID    string `json:"id"`
			Quote string `json:"quote"`
		} `json:"observations"`
		Patterns []struct {
			ID string `json:"id"`
		} `json:"patterns"`
	}
	_ = json.Unmarshal([]byte(input), &payload)
	// An explicit ja-JP output locale switches generated text to Japanese;
	// quotes are evidence and are never rewritten.
	text := englishText
	if strings.Contains(req.SystemPrompt, "Output locale: ja-JP") {
		text = japaneseText
	}
	var ids, supporting, counter []string
	for _, o := range payload.Observations {
		ids = append(ids, o.ID)
		if strings.Contains(strings.ToLower(o.Quote), "contradicts") {
			counter = append(counter, o.ID)
		} else {
			supporting = append(supporting, o.ID)
		}
	}
	var patternIDs []string
	for _, p := range payload.Patterns {
		patternIDs = append(patternIDs, p.ID)
	}

	var out any
	switch req.Schema.Name {
	case "observation_extraction":
		var observations []map[string]string
		for _, line := range strings.Split(input, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				observations = append(observations, map[string]string{"quote": line, "behavior": text.reports + line, "topic": text.topic})
			}
		}
		out = map[string]any{"observations": nonNil(observations)}
	case "trace_detection":
		out = map[string]any{"traces": []map[string]any{{
			"title": text.traceTitle, "expectation": text.traceExpectation,
			"actualBehavior": text.traceActual, "deviationType": "other", "observationIds": nonNil(ids),
		}}}
	case "pattern_detection":
		out = map[string]any{"patterns": []any{}}
	case "need_hypothesis":
		out = map[string]any{"hypotheses": []map[string]any{{
			"title": text.hypothesisTitle, "statedNeed": "", "latentNeed": text.latentNeed,
			"jtbd": "", "expectation": text.expectation, "surprisingFact": text.surprisingFact,
			"rationale":                text.rationale,
			"supportingObservationIds": nonNil(ids), "basedOnPatternIds": nonNil(patternIDs),
			"expectationBasis":      "MODEL_PROPOSED_POST_HOC",
			"missingEvidence":       []string{text.missingEvidence},
			"falsificationCriteria": []string{text.falsification},
			"alternativeExplanations": []map[string]any{
				{"title": text.altTrendTitle, "explanation": text.altTrend, "missingEvidence": []string{text.altTrendMissing}},
				{"title": text.altMeasureTitle, "explanation": text.altMeasure, "missingEvidence": []string{text.altMeasureMissing}},
			},
		}}}
	case "evidence_retrieval":
		out = map[string]any{"supportingObservationIds": nonNil(supporting), "counterObservationIds": nonNil(counter), "counterSearched": true}
	case "insight_writeup":
		out = map[string]any{"title": text.insightTitle, "observationSummary": text.observationSummary, "interpretation": text.interpretation,
			"alternativeInterpretation": text.alternative, "productOpportunity": "", "monetizationAngle": ""}
	case "question_exploration":
		// Question-only: no evidence exists, so nothing here is grounded and
		// nothing states a real-world number, quote or source.
		out = map[string]any{
			"candidates": []map[string]any{
				{
					"title": text.exploreTitleA, "explanation": text.exploreExplainA,
					"competingExplanations":   []map[string]string{{"title": text.exploreCompetingTitle, "explanation": text.exploreCompetingExplain}},
					"falsificationConditions": []string{text.exploreFalsifyA},
					"requiredData":            []map[string]string{{"description": text.exploreDataA, "why": text.exploreDataWhy}},
				},
				{
					"title": text.exploreTitleB, "explanation": text.exploreExplainB,
					"competingExplanations":   []map[string]string{{"title": text.exploreCompetingTitle, "explanation": text.exploreCompetingExplain}},
					"falsificationConditions": []string{text.exploreFalsifyB},
					"requiredData":            []map[string]string{{"description": text.exploreDataB, "why": text.exploreDataWhy}},
				},
			},
			"limitations": []string{text.exploreLimitation},
		}
	case "insight_dedupe":
		out = map[string]any{"duplicateGroups": []any{}}
	default:
		return nil, fmt.Errorf("scripted model has no answer for %s", req.Schema.Name)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	if req.Schema.Validate != nil {
		if err := req.Schema.Validate(raw); err != nil {
			return nil, fmt.Errorf("scripted %s answer is invalid: %w", req.Schema.Name, err)
		}
	}
	return &llm.GenerateResponse{Content: raw, Usage: llm.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2}}, nil
}

// scriptedText is every generated string the stand-in writes, per locale.
type scriptedText struct {
	reports, topic                                           string
	traceTitle, traceExpectation, traceActual                string
	hypothesisTitle, latentNeed, expectation, surprisingFact string
	rationale, missingEvidence, falsification                string
	altTrendTitle, altTrend, altTrendMissing                 string
	altMeasureTitle, altMeasure, altMeasureMissing           string
	insightTitle, observationSummary, interpretation         string
	alternative                                              string

	exploreTitleA, exploreExplainA, exploreFalsifyA, exploreDataA  string
	exploreTitleB, exploreExplainB, exploreFalsifyB, exploreDataB  string
	exploreCompetingTitle, exploreCompetingExplain, exploreDataWhy string
	exploreLimitation                                              string
}

var englishText = scriptedText{
	reports: "reports: ", topic: "evidence",
	traceTitle: "Outcome moved against the expectation", traceExpectation: "the outcome stays flat", traceActual: "the outcome changed",
	hypothesisTitle: "Primary explanation", latentNeed: "the observed change has an explanatory mechanism that differs from the baseline",
	expectation: "the observed measure stays near its baseline", surprisingFact: "the observed measure changed",
	rationale:       "if the intervention changed behavior, the change is expected",
	missingEvidence: "untreated comparison group outcomes over the same period", falsification: "the comparison group changed by the same amount",
	altTrendTitle: "Common trend", altTrend: "a shared external trend moved every group", altTrendMissing: "pre-period trend for all groups",
	altMeasureTitle: "Measurement change", altMeasure: "the counting method changed", altMeasureMissing: "measurement definition history",
	insightTitle: "Scripted insight", observationSummary: "grounded observations", interpretation: "a candidate explanation",
	alternative: "a shared trend",

	exploreTitleA: "Candidate A: a structural change", exploreExplainA: "an underlying structural change may explain the situation described in the question",
	exploreFalsifyA: "the situation is unchanged in places without that structural change", exploreDataA: "a time series of the measure across comparable places",
	exploreTitleB: "Candidate B: a change in how it is measured", exploreExplainB: "a change in the counting or definition may explain the apparent pattern",
	exploreFalsifyB: "the pattern persists under one fixed definition", exploreDataB: "the definition history of the measure",
	exploreCompetingTitle: "Chance or a shared external trend", exploreCompetingExplain: "an external trend may move every place at once",
	exploreDataWhy:    "to check the candidate against evidence that has not been supplied yet",
	exploreLimitation: "No evidence was supplied; every candidate is unverified.",
}

var japaneseText = scriptedText{
	reports: "記録: ", topic: "エビデンス",
	traceTitle: "結果が期待に反して動いた", traceExpectation: "結果は横ばいのまま", traceActual: "結果が変化した",
	hypothesisTitle: "主要な説明", latentNeed: "観測された変化には基準とは異なる説明メカニズムがある",
	expectation: "観測値は基準付近にとどまる", surprisingFact: "観測値が変化した",
	rationale:       "介入が行動を変えたのであれば、この変化は予想される",
	missingEvidence: "同じ期間の未介入の比較群の結果", falsification: "比較群も同じだけ変化していた",
	altTrendTitle: "共通のトレンド", altTrend: "外部の共通トレンドがすべての群を動かした", altTrendMissing: "全群の事前期間のトレンド",
	altMeasureTitle: "測定方法の変更", altMeasure: "数え方が変わった", altMeasureMissing: "測定定義の変更履歴",
	insightTitle: "スクリプトによるインサイト", observationSummary: "根拠のある観察", interpretation: "候補となる説明",
	alternative: "共通のトレンド",

	exploreTitleA: "候補A: 構造的な変化", exploreExplainA: "質問にある状況は、背景にある構造的な変化で説明できるかもしれない",
	exploreFalsifyA: "その構造的な変化がない地域でも同じ状況が見られる", exploreDataA: "比較できる地域ごとの指標の時系列",
	exploreTitleB: "候補B: 測り方の変化", exploreExplainB: "数え方や定義の変更が見かけのパターンを生んでいるかもしれない",
	exploreFalsifyB: "定義を固定してもパターンが残る", exploreDataB: "指標の定義の変更履歴",
	exploreCompetingTitle: "偶然、または共通の外部トレンド", exploreCompetingExplain: "外部の共通トレンドがすべての地域を同時に動かしているかもしれない",
	exploreDataWhy:    "まだ用意されていない資料でこの候補を確かめるため",
	exploreLimitation: "資料が与えられていないため、すべての候補は未検証です。",
}

func nonNil[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}
