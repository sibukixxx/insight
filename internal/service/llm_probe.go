package service

import (
	"context"
	"encoding/json"
	"fmt"

	"insight-lab/internal/llm"
)

// ProbeSchema is used by the settings connection test: the smallest
// possible structured-output round trip, just enough to tell whether the
// configured endpoint accepts the request at all and which fallback stage
// (see internal/llm/openai.go) it needed.
func probeSchema() llm.Schema {
	return llm.Schema{
		Name: "connection_probe",
		Schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"ok": map[string]any{"type": "boolean"}},
			"required":   []string{"ok"},
		},
		Validate: func(raw json.RawMessage) error {
			var out struct {
				OK bool `json:"ok"`
			}
			if err := json.Unmarshal(raw, &out); err != nil {
				return fmt.Errorf("invalid json: %w", err)
			}
			if !out.OK {
				return fmt.Errorf(`"ok" must be true`)
			}
			return nil
		},
	}
}

type ProbeResult struct {
	Mode llm.Mode
}

func ProbeConnection(ctx context.Context, client llm.Client) (*ProbeResult, error) {
	resp, err := client.Generate(ctx, llm.GenerateRequest{
		SystemPrompt: "接続確認です。指定されたJSON形式のみを出力してください。",
		Messages:     []llm.Message{{Role: "user", Content: `{"ok":true} という形式で応答してください。`}},
		Schema:       probeSchema(),
		Temperature:  0,
	})
	if err != nil {
		return nil, err
	}
	return &ProbeResult{Mode: resp.Mode}, nil
}
