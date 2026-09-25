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
				observations = append(observations, map[string]string{"quote": line, "behavior": "reports: " + line, "topic": "evidence"})
			}
		}
		out = map[string]any{"observations": nonNil(observations)}
	case "trace_detection":
		out = map[string]any{"traces": []map[string]any{{
			"title": "Outcome moved against the expectation", "expectation": "the outcome stays flat",
			"actualBehavior": "the outcome changed", "deviationType": "other", "observationIds": nonNil(ids),
		}}}
	case "pattern_detection":
		out = map[string]any{"patterns": []any{}}
	case "need_hypothesis":
		out = map[string]any{"hypotheses": []map[string]any{{
			"title": "Primary explanation", "statedNeed": "", "latentNeed": "the observed change has an explanatory mechanism that differs from the baseline",
			"jtbd": "", "expectation": "the observed measure stays near its baseline", "surprisingFact": "the observed measure changed",
			"rationale":                "if the intervention changed behavior, the change is expected",
			"supportingObservationIds": nonNil(ids), "basedOnPatternIds": nonNil(patternIDs),
			"expectationBasis":      "MODEL_PROPOSED_POST_HOC",
			"missingEvidence":       []string{"untreated comparison group outcomes over the same period"},
			"falsificationCriteria": []string{"the comparison group changed by the same amount"},
			"alternativeExplanations": []map[string]any{
				{"title": "Common trend", "explanation": "a shared external trend moved every group", "missingEvidence": []string{"pre-period trend for all groups"}},
				{"title": "Measurement change", "explanation": "the counting method changed", "missingEvidence": []string{"measurement definition history"}},
			},
		}}}
	case "evidence_retrieval":
		out = map[string]any{"supportingObservationIds": nonNil(supporting), "counterObservationIds": nonNil(counter), "counterSearched": true}
	case "insight_writeup":
		out = map[string]any{"title": "Scripted insight", "observationSummary": "grounded observations", "interpretation": "a candidate explanation",
			"alternativeInterpretation": "a shared trend", "productOpportunity": "", "monetizationAngle": ""}
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

func nonNil[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}
