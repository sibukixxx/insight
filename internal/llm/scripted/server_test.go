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

// The stand-in must follow an explicit output locale so ja-JP conformance
// exercises generated text, while quotes stay verbatim evidence (#125).
func TestScriptedServerWritesJapaneseTextAndKeepsQuotesWhenOutputLocaleIsJaJP(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()
	client := llm.NewOpenAIClient(srv.URL, "scripted", "scripted-model")
	generate := func(schema, input string) json.RawMessage {
		t.Helper()
		resp, err := client.Generate(context.Background(), llm.GenerateRequest{
			SystemPrompt: "task\n\nOutput locale: ja-JP.\n- Write every generated natural-language field in Japanese",
			Messages:     []llm.Message{{Role: "user", Content: input}},
			Schema:       llm.Schema{Name: schema, Schema: map[string]any{"type": "object"}, Validate: func(json.RawMessage) error { return nil }},
		})
		if err != nil {
			t.Fatal(err)
		}
		return resp.Content
	}
	var obs struct {
		Observations []struct{ Quote, Behavior string } `json:"observations"`
	}
	if err := json.Unmarshal(generate("observation_extraction", "Sales rose 12% in March."), &obs); err != nil || len(obs.Observations) != 1 {
		t.Fatalf("observations = %+v (%v)", obs, err)
	}
	if obs.Observations[0].Quote != "Sales rose 12% in March." || obs.Observations[0].Behavior != "記録: Sales rose 12% in March." {
		t.Fatalf("quote must stay verbatim and generated text must be Japanese: %+v", obs.Observations[0])
	}
	var hyp struct {
		Hypotheses []struct {
			LatentNeed string `json:"latentNeed"`
		} `json:"hypotheses"`
	}
	if err := json.Unmarshal(generate("need_hypothesis", `{"observations":[]}`), &hyp); err != nil || len(hyp.Hypotheses) != 1 || hyp.Hypotheses[0].LatentNeed != "観測された変化には基準とは異なる説明メカニズムがある" {
		t.Fatalf("hypothesis = %+v (%v)", hyp, err)
	}
}
