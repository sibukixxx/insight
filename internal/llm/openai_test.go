package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type testOutput struct {
	Value string `json:"value"`
}

func validateTestOutput(raw json.RawMessage) error {
	var out testOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	if out.Value == "" {
		return errors.New(`"value" is required`)
	}
	return nil
}

func testSchema() Schema {
	return Schema{
		Name:     "test_output",
		Schema:   map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"type": "string"}}},
		Validate: validateTestOutput,
	}
}

func chatResponseBody(content string) string {
	b, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{
			{"message": map[string]any{"content": content}},
		},
		"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
	})
	return string(b)
}

func TestGenerateHappyPathJSONSchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		rf, _ := body["response_format"].(map[string]any)
		if rf["type"] != "json_schema" {
			t.Errorf("expected json_schema response_format, got %v", rf["type"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(chatResponseBody(`{"value":"ok"}`)))
	}))
	defer srv.Close()

	c := NewOpenAIClient(srv.URL, "test-key", "gpt-test")
	resp, err := c.Generate(context.Background(), GenerateRequest{
		SystemPrompt: "sys",
		Messages:     []Message{{Role: "user", Content: "hi"}},
		Schema:       testSchema(),
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Mode != ModeJSONSchema {
		t.Errorf("Mode = %v, want json_schema", resp.Mode)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("Usage.TotalTokens = %d, want 15", resp.Usage.TotalTokens)
	}
	var out testOutput
	if err := json.Unmarshal(resp.Content, &out); err != nil || out.Value != "ok" {
		t.Errorf("Content = %s, err = %v", resp.Content, err)
	}
}

func TestGenerateFallsBackToJSONObjectWhenSchemaUnsupported(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		rf, _ := body["response_format"].(map[string]any)

		if n == 1 {
			if rf["type"] != "json_schema" {
				t.Errorf("first call should try json_schema, got %v", rf["type"])
			}
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Unknown parameter: 'response_format.json_schema'."}}`))
			return
		}

		if rf["type"] != "json_object" {
			t.Errorf("second call should fall back to json_object, got %v", rf["type"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(chatResponseBody(`{"value":"fallback-ok"}`)))
	}))
	defer srv.Close()

	c := NewOpenAIClient(srv.URL, "test-key", "gpt-test")
	resp, err := c.Generate(context.Background(), GenerateRequest{
		SystemPrompt: "sys",
		Messages:     []Message{{Role: "user", Content: "hi"}},
		Schema:       testSchema(),
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Mode != ModeJSONObject {
		t.Errorf("Mode = %v, want json_object", resp.Mode)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestGenerateRetriesOnValidationFailure(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
		if n < 2 {
			_, _ = w.Write([]byte(chatResponseBody(`{"value":""}`))) // fails validation (empty value)
			return
		}
		_, _ = w.Write([]byte(chatResponseBody(`{"value":"finally-ok"}`)))
	}))
	defer srv.Close()

	c := NewOpenAIClient(srv.URL, "", "gpt-test")
	resp, err := c.Generate(context.Background(), GenerateRequest{
		SystemPrompt: "sys",
		Messages:     []Message{{Role: "user", Content: "hi"}},
		Schema:       testSchema(),
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var out testOutput
	_ = json.Unmarshal(resp.Content, &out)
	if out.Value != "finally-ok" {
		t.Errorf("Content = %s", resp.Content)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls (1 retry), got %d", calls)
	}
}

func TestGenerateFailsAfterExhaustingValidationRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(chatResponseBody(`{"value":""}`)))
	}))
	defer srv.Close()

	c := NewOpenAIClient(srv.URL, "", "gpt-test")
	_, err := c.Generate(context.Background(), GenerateRequest{
		SystemPrompt: "sys",
		Messages:     []Message{{Role: "user", Content: "hi"}},
		Schema:       testSchema(),
	})
	if err == nil {
		t.Fatal("expected an error after exhausting validation retries")
	}
	if !strings.Contains(err.Error(), "schema validation") {
		t.Errorf("error = %v, want mention of schema validation", err)
	}
}

func TestGenerateRetriesTransientErrors(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"overloaded"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(chatResponseBody(`{"value":"ok-after-retry"}`)))
	}))
	defer srv.Close()

	c := NewOpenAIClient(srv.URL, "", "gpt-test")
	resp, err := c.Generate(context.Background(), GenerateRequest{
		SystemPrompt: "sys",
		Messages:     []Message{{Role: "user", Content: "hi"}},
		Schema:       testSchema(),
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var out testOutput
	_ = json.Unmarshal(resp.Content, &out)
	if out.Value != "ok-after-retry" {
		t.Errorf("Content = %s", resp.Content)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestGenerateStripsCodeFence(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(chatResponseBody("```json\n{\"value\":\"fenced\"}\n```")))
	}))
	defer srv.Close()

	c := NewOpenAIClient(srv.URL, "", "gpt-test")
	resp, err := c.Generate(context.Background(), GenerateRequest{
		SystemPrompt: "sys",
		Messages:     []Message{{Role: "user", Content: "hi"}},
		Schema:       testSchema(),
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var out testOutput
	if err := json.Unmarshal(resp.Content, &out); err != nil || out.Value != "fenced" {
		t.Errorf("Content = %s, err = %v", resp.Content, err)
	}
}

func TestGenerateNonRetryableErrorFailsFast(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()

	c := NewOpenAIClient(srv.URL, "bad-key", "gpt-test")
	_, err := c.Generate(context.Background(), GenerateRequest{
		SystemPrompt: "sys",
		Messages:     []Message{{Role: "user", Content: "hi"}},
		Schema:       testSchema(),
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 call (no retry on 401), got %d", calls)
	}
}
