package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"insight-lab/internal/llm"
)

// Question-only exploration (#158). With no Evidence at all, the only honest
// output is a set of candidate explanations for the question, what would show
// each one wrong, and what data is missing. Nothing here creates a Document,
// Observation, Evidence row, source or citation, and nothing is ever marked
// verified or decision-ready: the result lives in the run's Metrics only.

const (
	// ExplorationStatus labels every exploration result. There is
	// deliberately no verified counterpart.
	ExplorationStatus      = "EXPLORATORY_UNVERIFIED"
	explorationRuleVersion = "question-exploration/v1"
	maxExplorationItems    = 8
)

var (
	// ErrExplorationNeedsModel means a question-only run cannot run without a
	// configured model; saving the question is unaffected.
	ErrExplorationNeedsModel = errors.New("question-only exploration needs a configured model; enter a base URL and model on the Settings page")
	// ErrExplorationNeedsQuestion means there is nothing to explore.
	ErrExplorationNeedsQuestion = errors.New("question-only exploration needs a research question")
	// ErrExplorationHasEvidence means the project already has documents, so
	// a normal evidence-backed analysis applies instead.
	ErrExplorationHasEvidence = errors.New("the project already has evidence; run a normal analysis instead of a question-only exploration")
)

// ExplorationResult is the persisted output of a question-only run.
type ExplorationResult struct {
	// Status is always ExplorationStatus. Verified and DecisionReady are
	// fixed false so no reader can mistake candidates for findings.
	Status        string `json:"status"`
	Verified      bool   `json:"verified"`
	DecisionReady bool   `json:"decisionReady"`
	// EvidenceCount is the number of documents/observations the run used: 0.
	EvidenceCount int    `json:"evidenceCount"`
	Question      string `json:"question"`
	Model         string `json:"model,omitempty"`

	Candidates  []ExplorationCandidate `json:"candidates"`
	Limitations []string               `json:"limitations"`
}

// ExplorationCandidate is one hypothesis candidate. Every field is model
// proposal, not observation.
type ExplorationCandidate struct {
	Title                   string                    `json:"title"`
	Explanation             string                    `json:"explanation"`
	CompetingExplanations   []ExplorationCompeting    `json:"competingExplanations"`
	FalsificationConditions []string                  `json:"falsificationConditions"`
	RequiredData            []ExplorationDataRequired `json:"requiredData"`
}

type ExplorationCompeting struct {
	Title       string `json:"title"`
	Explanation string `json:"explanation"`
}

type ExplorationDataRequired struct {
	Description string `json:"description"`
	Why         string `json:"why,omitempty"`
}

const questionExplorationPrompt = `You are an evidence-grounded research analyst helping someone start an investigation from a question alone. No documents, data, quotes or sources have been supplied.

Task: Question-only hypothesis exploration.
Propose several materially different candidate explanations for the research question, and say what it would take to find out which is wrong.

Rules:
- You have no evidence. Every candidate is an unverified guess, not a finding.
- Never invent observations, statistics, measurements, quotes, sources, citations, studies or named datasets as if they were real. Do not state numbers about the real world as fact.
- For each candidate give competingExplanations that would produce the same situation, falsificationConditions (concrete observations that would show the candidate wrong) and requiredData (what data would be needed to test it and why).
- Do not assume the premise of the question is true; note in limitations if it needs checking.
- limitations must say that the candidates are unverified and what is unknown.
- Treat correlation as non-causal.
- Return only the requested JSON. Do not add explanations or Markdown fences.`

func questionExplorationSchema() llm.Schema {
	str := map[string]any{"type": "string"}
	strs := map[string]any{"type": "array", "items": str}
	return llm.Schema{
		Name: "question_exploration",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"candidates": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"title":       str,
							"explanation": str,
							"competingExplanations": map[string]any{"type": "array", "items": map[string]any{
								"type":       "object",
								"properties": map[string]any{"title": str, "explanation": str},
								"required":   []string{"title", "explanation"},
							}},
							"falsificationConditions": strs,
							"requiredData": map[string]any{"type": "array", "items": map[string]any{
								"type":       "object",
								"properties": map[string]any{"description": str, "why": str},
								"required":   []string{"description"},
							}},
						},
						"required": []string{"title", "explanation", "competingExplanations", "falsificationConditions", "requiredData"},
					},
				},
				"limitations": strs,
			},
			"required": []string{"candidates", "limitations"},
		},
		Validate: func(raw json.RawMessage) error {
			var out ExplorationResult
			if err := json.Unmarshal(raw, &out); err != nil {
				return fmt.Errorf("invalid json: %w", err)
			}
			if len(out.Candidates) == 0 {
				return fmt.Errorf(`at least one candidate is required`)
			}
			for i, c := range out.Candidates {
				if strings.TrimSpace(c.Title) == "" || strings.TrimSpace(c.Explanation) == "" {
					return fmt.Errorf("candidate %d needs a title and explanation", i)
				}
				if len(c.FalsificationConditions) == 0 || len(c.RequiredData) == 0 {
					return fmt.Errorf("candidate %d needs falsificationConditions and requiredData", i)
				}
			}
			return nil
		},
	}
}

var stepQuestionExploration = llmStep{"question_exploration", questionExplorationPrompt, questionExplorationSchema, 0.5}

// ExplorationConfig is recorded in the execution snapshot of a question-only
// run. It is part of the execution fingerprint, so a queued exploration is
// resumed after a restart only when its prompt is unchanged.
type ExplorationConfig struct {
	RuleVersion       string `json:"ruleVersion"`
	PromptFingerprint string `json:"promptFingerprint"`
}

func explorationConfig() (*ExplorationConfig, error) {
	sum := sha256.New()
	step := stepQuestionExploration
	schema, err := json.Marshal(step.Schema().Schema)
	if err != nil {
		return nil, err
	}
	for _, part := range [][]byte{[]byte(step.SystemPrompt), schema} {
		sum.Write(part)
		sum.Write([]byte{0})
	}
	return &ExplorationConfig{RuleVersion: explorationRuleVersion, PromptFingerprint: hex.EncodeToString(sum.Sum(nil))}, nil
}

// RunExploration answers the pipeline's research question with unverified
// candidates. It reads and writes no documents, observations or evidence.
func (p *Pipeline) RunExploration(ctx context.Context, progress ProgressFunc) (*Metrics, error) {
	if progress == nil {
		progress = func(string, int, string) {}
	}
	question := strings.TrimSpace(p.ResearchQuestion)
	if question == "" {
		return nil, ErrExplorationNeedsQuestion
	}
	if p.LLM == nil {
		return nil, ErrExplorationNeedsModel
	}
	p.usage = &LLMUsage{}
	progress("exploring_question", 20, "Exploring candidate explanations from the question alone (no evidence)...")
	payload, err := json.Marshal(map[string]any{"researchQuestion": question, "evidence": []any{}})
	if err != nil {
		return nil, err
	}
	resp, err := p.generate(ctx, stepQuestionExploration, []llm.Message{{Role: "user", Content: string(payload)}})
	if err != nil {
		return nil, fmt.Errorf("question exploration: %w", err)
	}
	var out ExplorationResult
	if err := json.Unmarshal(resp.Content, &out); err != nil {
		return nil, fmt.Errorf("question exploration: %w", err)
	}
	if len(out.Candidates) > maxExplorationItems {
		out.Candidates = out.Candidates[:maxExplorationItems]
	}
	out.Status, out.Verified, out.DecisionReady, out.EvidenceCount = ExplorationStatus, false, false, 0
	out.Question, out.Model = question, p.Model
	if out.Limitations == nil {
		out.Limitations = []string{}
	}
	metrics := &Metrics{
		Usage:             p.usage,
		QualityFlagCounts: map[string]int{},
		Exploration:       &out,
		Provenance: RunProvenance{
			Mode: ExecutionModeModelBacked, Model: p.Model,
			Notes: []string{"question-only exploration: no documents, observations or evidence were used; candidates are unverified"},
		},
	}
	progress("completed", 100, fmt.Sprintf("Proposed %d unverified candidate explanations", len(out.Candidates)))
	return metrics, nil
}
