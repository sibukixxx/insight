package scripted

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"insight-lab/internal/llm"
)

// The engine's own OpenAI client must be able to talk to the scripted server;
// that is what lets SDK repositories run model-backed fixtures end to end.
func TestOpenAIClientReceivesGroundedAnswerFromScriptedServer(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()
	client := llm.NewOpenAIClient(srv.URL, "scripted", "scripted-model")
	resp, err := client.Generate(context.Background(), llm.GenerateRequest{
		SystemPrompt: "extract",
		Messages:     []llm.Message{{Role: "user", Content: "Sales rose in March.\nThis customer contradicts that."}},
		Schema:       llm.Schema{Name: "observation_extraction", Schema: map[string]any{"type": "object"}, Validate: func(json.RawMessage) error { return nil }},
	})
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Observations []struct {
			Quote string `json:"quote"`
		} `json:"observations"`
	}
	if err := json.Unmarshal(resp.Content, &out); err != nil || len(out.Observations) != 2 || out.Observations[1].Quote != "This customer contradicts that." {
		t.Fatalf("answer = %s (%v)", resp.Content, err)
	}
}

func TestScriptedServerRejectsUnknownStep(t *testing.T) {
	if _, err := Answer("unknown_step", "x"); err == nil {
		t.Fatal("unknown step must fail instead of inventing an answer")
	}
}
