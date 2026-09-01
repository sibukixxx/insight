package service

import (
	"encoding/json"
	"fmt"

	"insight-lab/internal/llm"
)

// --- Observation Extraction ---

type observationCandidate struct {
	Quote    string `json:"quote"`
	Behavior string `json:"behavior"`
	Topic    string `json:"topic"`
}

type observationExtractionOutput struct {
	Observations []observationCandidate `json:"observations"`
}

func observationExtractionSchema() llm.Schema {
	return llm.Schema{
		Name: "observation_extraction",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"observations": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"quote":    map[string]any{"type": "string"},
							"behavior": map[string]any{"type": "string"},
							"topic":    map[string]any{"type": "string"},
						},
						"required": []string{"quote", "behavior"},
					},
				},
			},
			"required": []string{"observations"},
		},
		Validate: func(raw json.RawMessage) error {
			var out observationExtractionOutput
			if err := json.Unmarshal(raw, &out); err != nil {
				return fmt.Errorf("invalid json: %w", err)
			}
			if out.Observations == nil {
				return fmt.Errorf(`"observations" array is required (use [] if none)`)
			}
			return nil
		},
	}
}

// --- Pattern Detection ---

type observationRef struct {
	ID       string `json:"id"`
	Quote    string `json:"quote"`
	Behavior string `json:"behavior"`
	Topic    string `json:"topic,omitempty"`
}

type patternCandidate struct {
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	ObservationIDs []string `json:"observationIds"`
}

// patternRef is how an already-persisted Pattern (real ID, observations
// already filtered down to ones that exist) is described back to the
// Hypothesis Generation step, so a hypothesis can cite the specific
// pattern(s) it was built from.
type patternRef struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Description      string `json:"description,omitempty"`
	ObservationCount int    `json:"observationCount"`
}

type patternDetectionOutput struct {
	Patterns []patternCandidate `json:"patterns"`
}

func patternDetectionSchema() llm.Schema {
	return llm.Schema{
		Name: "pattern_detection",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"patterns": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"title":          map[string]any{"type": "string"},
							"description":    map[string]any{"type": "string"},
							"observationIds": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						},
						"required": []string{"title", "observationIds"},
					},
				},
			},
			"required": []string{"patterns"},
		},
		Validate: func(raw json.RawMessage) error {
			var out patternDetectionOutput
			if err := json.Unmarshal(raw, &out); err != nil {
				return fmt.Errorf("invalid json: %w", err)
			}
			if out.Patterns == nil {
				return fmt.Errorf(`"patterns" array is required (use [] if none found)`)
			}
			return nil
		},
	}
}

// --- Need Hypothesis Generation ---

type hypothesisCandidate struct {
	Title                    string   `json:"title"`
	StatedNeed               string   `json:"statedNeed"`
	LatentNeed               string   `json:"latentNeed"`
	JTBD                     string   `json:"jtbd"`
	Rationale                string   `json:"rationale"`
	SupportingObservationIDs []string `json:"supportingObservationIds"`
	BasedOnPatternIDs        []string `json:"basedOnPatternIds"`
}

type hypothesisOutput struct {
	Hypotheses []hypothesisCandidate `json:"hypotheses"`
}

func hypothesisSchema() llm.Schema {
	return llm.Schema{
		Name: "need_hypothesis",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"hypotheses": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"title":                    map[string]any{"type": "string"},
							"statedNeed":               map[string]any{"type": "string"},
							"latentNeed":               map[string]any{"type": "string"},
							"jtbd":                     map[string]any{"type": "string"},
							"rationale":                map[string]any{"type": "string"},
							"supportingObservationIds": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
							"basedOnPatternIds":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						},
						"required": []string{"title", "latentNeed", "supportingObservationIds"},
					},
				},
			},
			"required": []string{"hypotheses"},
		},
		Validate: func(raw json.RawMessage) error {
			var out hypothesisOutput
			if err := json.Unmarshal(raw, &out); err != nil {
				return fmt.Errorf("invalid json: %w", err)
			}
			if len(out.Hypotheses) == 0 {
				return fmt.Errorf(`"hypotheses" must contain at least one item`)
			}
			for i, h := range out.Hypotheses {
				if h.Title == "" || h.LatentNeed == "" {
					return fmt.Errorf("hypotheses[%d]: title and latentNeed are required", i)
				}
			}
			return nil
		},
	}
}

// --- Evidence Retrieval ---

type evidenceRetrievalOutput struct {
	SupportingObservationIDs []string `json:"supportingObservationIds"`
	CounterObservationIDs    []string `json:"counterObservationIds"`
	CounterSearched          bool     `json:"counterSearched"`
}

func evidenceRetrievalSchema() llm.Schema {
	return llm.Schema{
		Name: "evidence_retrieval",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"supportingObservationIds": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"counterObservationIds":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"counterSearched":          map[string]any{"type": "boolean"},
			},
			"required": []string{"supportingObservationIds", "counterObservationIds", "counterSearched"},
		},
		Validate: func(raw json.RawMessage) error {
			var out evidenceRetrievalOutput
			if err := json.Unmarshal(raw, &out); err != nil {
				return fmt.Errorf("invalid json: %w", err)
			}
			if out.SupportingObservationIDs == nil || out.CounterObservationIDs == nil {
				return fmt.Errorf(`"supportingObservationIds" and "counterObservationIds" arrays are required (use [] if none)`)
			}
			return nil
		},
	}
}

// --- Insight Generation (write-up only; never introduces new quotes) ---

type insightWriteup struct {
	Title                     string `json:"title"`
	ObservationSummary        string `json:"observationSummary"`
	Interpretation            string `json:"interpretation"`
	AlternativeInterpretation string `json:"alternativeInterpretation"`
	ProductOpportunity        string `json:"productOpportunity"`
	MonetizationAngle         string `json:"monetizationAngle"`
}

func insightWriteupSchema() llm.Schema {
	return llm.Schema{
		Name: "insight_writeup",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":                     map[string]any{"type": "string"},
				"observationSummary":        map[string]any{"type": "string"},
				"interpretation":            map[string]any{"type": "string"},
				"alternativeInterpretation": map[string]any{"type": "string"},
				"productOpportunity":        map[string]any{"type": "string"},
				"monetizationAngle":         map[string]any{"type": "string"},
			},
			"required": []string{"title", "interpretation", "alternativeInterpretation"},
		},
		Validate: func(raw json.RawMessage) error {
			var out insightWriteup
			if err := json.Unmarshal(raw, &out); err != nil {
				return fmt.Errorf("invalid json: %w", err)
			}
			if out.Title == "" || out.Interpretation == "" || out.AlternativeInterpretation == "" {
				return fmt.Errorf("title, interpretation and alternativeInterpretation are required")
			}
			return nil
		},
	}
}

// --- Insight Dedupe ---

type dedupeOutput struct {
	DuplicateGroups [][]int `json:"duplicateGroups"`
}

func dedupeSchema() llm.Schema {
	return llm.Schema{
		Name: "insight_dedupe",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"duplicateGroups": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
				},
			},
			"required": []string{"duplicateGroups"},
		},
		Validate: func(raw json.RawMessage) error {
			var out dedupeOutput
			if err := json.Unmarshal(raw, &out); err != nil {
				return fmt.Errorf("invalid json: %w", err)
			}
			if out.DuplicateGroups == nil {
				return fmt.Errorf(`"duplicateGroups" array is required (use [] if every insight is unique)`)
			}
			return nil
		},
	}
}
