// Package llm abstracts calls to an OpenAI-compatible chat completion API.
// Every caller must pass a JSON schema and a Validate function: the client
// never hands back free-form text that the rest of the system trusts
// blindly, because an unvalidated LLM response is exactly how quotes get
// invented (see internal/service/grounding.go, which is the second line of
// defense once a response passes here).
package llm

import (
	"context"
	"encoding/json"
)

type Message struct {
	Role    string // "system" | "user" | "assistant"
	Content string
}

// Schema describes the JSON shape a step expects back. Name/Schema are sent
// to providers that support response_format=json_schema; Validate is run
// against every response regardless of which fallback stage produced it.
type Schema struct {
	Name     string
	Schema   map[string]any
	Validate func(raw json.RawMessage) error
}

type GenerateRequest struct {
	SystemPrompt string
	Messages     []Message
	Schema       Schema
	Temperature  float64
}

// Mode records which structured-output strategy actually produced the
// response, surfaced by the settings connection test so a demo doesn't
// discover a fallback silently mid-pipeline.
type Mode string

const (
	ModeJSONSchema Mode = "json_schema"
	ModeJSONObject Mode = "json_object"
)

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type GenerateResponse struct {
	Content json.RawMessage
	Mode    Mode
	Usage   Usage
}

type Client interface {
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
}
