package triage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"insight-lab/internal/llm"
)

const modelSystemPrompt = `You triage dataset variables for a research question.
You see only a bounded column profile, never raw rows. For every column decide a bucket:
INCLUDE (deterministic processing should compute with it), DEFER (keep for later),
NEEDS_REVIEW (definition or relevance unclear). Never exclude data.
Optionally give a role (METRIC, DIMENSION, PERIOD, IDENTIFIER) and candidate classifications
(POSSIBLE_OUTCOME, POSSIBLE_EXPOSURE, POSSIBLE_CONFOUNDER, POSSIBLE_MEDIATOR, POSSIBLE_COLLIDER,
COMPARISON_CANDIDATE, DEFINITION_UNKNOWN). Classifications are hypotheses, not findings:
semantic relevance is not causal evidence and correlation is not causation.
Do not compute or estimate any numbers. Give a short rationale for every column.`

type modelDecision struct {
	Name            string   `json:"name"`
	Bucket          string   `json:"bucket"`
	Role            string   `json:"role"`
	Classifications []string `json:"classifications"`
	Rationale       string   `json:"rationale"`
}

type modelOutput struct {
	Decisions []modelDecision `json:"decisions"`
}

func modelSchema() llm.Schema {
	decision := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"name", "bucket", "role", "classifications", "rationale"},
		"properties": map[string]any{
			"name":            map[string]any{"type": "string"},
			"bucket":          map[string]any{"type": "string", "enum": []string{"INCLUDE", "DEFER", "NEEDS_REVIEW"}},
			"role":            map[string]any{"type": "string", "enum": []string{"", "METRIC", "DIMENSION", "PERIOD", "IDENTIFIER"}},
			"classifications": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": Classifications}},
			"rationale":       map[string]any{"type": "string"},
		},
	}
	return llm.Schema{
		Name: "data_triage",
		Schema: map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"decisions"},
			"properties": map[string]any{"decisions": map[string]any{"type": "array", "items": decision}},
		},
		Validate: func(raw json.RawMessage) error {
			var out modelOutput
			if err := json.Unmarshal(raw, &out); err != nil {
				return err
			}
			for i, d := range out.Decisions {
				if strings.TrimSpace(d.Name) == "" || strings.TrimSpace(d.Rationale) == "" {
					return fmt.Errorf("decisions[%d] needs name and rationale", i)
				}
			}
			return nil
		},
	}
}

// Model asks a language model for a proposal. Its output passes through the
// same Normalize invariant as every other proposer, so a model can neither
// invent columns, drop columns nor exclude data.
func Model(ctx context.Context, client llm.Client, in Input) ([]Decision, error) {
	payload, err := json.Marshal(map[string]any{
		"question": in.Question, "hypothesisIds": in.HypothesisIDs, "researchGaps": in.Gaps,
		"rowCount": in.Profile.RowCount, "columns": in.Profile.Columns,
	})
	if err != nil {
		return nil, err
	}
	resp, err := client.Generate(ctx, llm.GenerateRequest{
		SystemPrompt: modelSystemPrompt,
		Messages:     []llm.Message{{Role: "user", Content: string(payload)}},
		Schema:       modelSchema(),
		Temperature:  0,
	})
	if err != nil {
		return nil, fmt.Errorf("model triage: %w", err)
	}
	var out modelOutput
	if err := json.Unmarshal(resp.Content, &out); err != nil {
		return nil, fmt.Errorf("model triage: decode: %w", err)
	}
	decisions := make([]Decision, 0, len(out.Decisions))
	for _, d := range out.Decisions {
		decisions = append(decisions, Decision{Name: d.Name, Bucket: Bucket(d.Bucket), Role: Role(d.Role), Classifications: d.Classifications, Rationale: d.Rationale})
	}
	return decisions, nil
}
