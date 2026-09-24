package scripted

import (
	"context"
	"encoding/json"
	"net/http"

	"insight-lab/internal/llm"
)

// Answer returns the scripted answer for one pipeline step.
func Answer(schemaName, input string) (json.RawMessage, error) {
	resp, err := Model{}.Generate(context.Background(), llm.GenerateRequest{
		Messages: []llm.Message{{Role: "user", Content: input}},
		Schema:   llm.Schema{Name: schemaName},
	})
	if err != nil {
		return nil, err
	}
	return resp.Content, nil
}

// Handler serves an OpenAI-compatible POST /chat/completions backed by the
// scripted model, so a real insight-lab can run model-backed with
// -base-url pointing here. It is a test tool, never a production model.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
			ResponseFormat struct {
				JSONSchema *struct {
					Name string `json:"name"`
				} `json:"json_schema"`
			} `json:"response_format"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20)).Decode(&req); err != nil || len(req.Messages) == 0 || req.ResponseFormat.JSONSchema == nil {
			writeError(w, "request must carry messages and a json_schema response_format")
			return
		}
		content, err := Answer(req.ResponseFormat.JSONSchema.Name, req.Messages[len(req.Messages)-1].Content)
		if err != nil {
			writeError(w, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": string(content)}}},
			"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		})
	})
	return mux
}

func writeError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": msg, "type": "invalid_request_error"}})
}
