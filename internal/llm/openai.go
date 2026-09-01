package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	stepTimeout        = 120 * time.Second
	maxValidationRetry = 2
	maxTransientRetry  = 3
	maxTotalIterations = 10 // hard safety belt against any looping bug above
)

type OpenAIClient struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

func NewOpenAIClient(baseURL, apiKey, model string) *OpenAIClient {
	return &OpenAIClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		HTTP:    &http.Client{Timeout: stepTimeout},
	}
}

type apiError struct {
	StatusCode        int
	Message           string
	SchemaUnsupported bool
	Retryable         bool
}

func (e *apiError) Error() string {
	return fmt.Sprintf("llm api error (status %d): %s", e.StatusCode, e.Message)
}

func (c *OpenAIClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	if req.Schema.Validate == nil {
		return nil, errors.New("llm: GenerateRequest.Schema.Validate must not be nil")
	}

	ctx, cancel := context.WithTimeout(ctx, stepTimeout)
	defer cancel()

	mode := ModeJSONSchema
	messages := buildMessages(req, mode)

	validationAttempts := 0
	transientAttempts := 0

	for iteration := 0; iteration < maxTotalIterations; iteration++ {
		raw, usage, err := c.callOnce(ctx, messages, mode, req.Schema, req.Temperature)
		if err != nil {
			var aerr *apiError
			if errors.As(err, &aerr) {
				if aerr.SchemaUnsupported && mode == ModeJSONSchema {
					mode = ModeJSONObject
					messages = buildMessages(req, mode)
					continue
				}
				if aerr.Retryable && transientAttempts < maxTransientRetry {
					transientAttempts++
					if slept := sleepBackoff(ctx, transientAttempts); !slept {
						return nil, ctx.Err()
					}
					continue
				}
			}
			return nil, err
		}

		if verr := req.Schema.Validate(raw); verr != nil {
			validationAttempts++
			if validationAttempts > maxValidationRetry {
				return nil, fmt.Errorf("response failed schema validation after %d attempts: %w", validationAttempts, verr)
			}
			messages = append(messages,
				Message{Role: "assistant", Content: string(raw)},
				Message{Role: "user", Content: fmt.Sprintf(
					"前回の出力はスキーマに適合しませんでした: %v\n%sで指定した形式のJSONのみを、他のテキストを含めずに出力してください。",
					verr, req.Schema.Name)},
			)
			continue
		}

		return &GenerateResponse{Content: raw, Mode: mode, Usage: usage}, nil
	}

	return nil, fmt.Errorf("llm: exceeded %d iterations without a valid response", maxTotalIterations)
}

func sleepBackoff(ctx context.Context, attempt int) bool {
	delay := time.Duration(attempt) * 500 * time.Millisecond
	select {
	case <-time.After(delay):
		return true
	case <-ctx.Done():
		return false
	}
}

func buildMessages(req GenerateRequest, mode Mode) []Message {
	system := req.SystemPrompt
	if mode == ModeJSONObject {
		schemaJSON, _ := json.MarshalIndent(req.Schema.Schema, "", "  ")
		system = system + "\n\n次のJSON Schemaに厳密に従うJSONオブジェクトのみを出力してください。説明文やコードブロックのマークダウンは含めないでください。\n" + string(schemaJSON)
	}
	out := make([]Message, 0, len(req.Messages)+1)
	out = append(out, Message{Role: "system", Content: system})
	out = append(out, req.Messages...)
	return out
}

type chatRequest struct {
	Model          string         `json:"model"`
	Messages       []chatMessage  `json:"messages"`
	Temperature    float64        `json:"temperature"`
	ResponseFormat responseFormat `json:"response_format"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type       string          `json:"type"`
	JSONSchema *jsonSchemaSpec `json:"json_schema,omitempty"`
}

type jsonSchemaSpec struct {
	Name   string         `json:"name"`
	Schema map[string]any `json:"schema"`
	Strict bool           `json:"strict"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type chatErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func (c *OpenAIClient) callOnce(ctx context.Context, messages []Message, mode Mode, schema Schema, temperature float64) (json.RawMessage, Usage, error) {
	body := chatRequest{
		Model:       c.Model,
		Temperature: temperature,
	}
	for _, m := range messages {
		body.Messages = append(body.Messages, chatMessage{Role: m.Role, Content: m.Content})
	}
	if mode == ModeJSONSchema {
		body.ResponseFormat = responseFormat{
			Type: "json_schema",
			JSONSchema: &jsonSchemaSpec{
				Name:   schema.Name,
				Schema: schema.Schema,
				Strict: true,
			},
		}
	} else {
		body.ResponseFormat = responseFormat{Type: "json_object"}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, Usage{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, Usage{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, Usage{}, &apiError{Message: err.Error(), Retryable: true}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, Usage{}, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp chatErrorResponse
		_ = json.Unmarshal(respBody, &errResp)
		msg := errResp.Error.Message
		if msg == "" {
			msg = string(respBody)
		}
		return nil, Usage{}, &apiError{
			StatusCode:        resp.StatusCode,
			Message:           msg,
			SchemaUnsupported: resp.StatusCode == http.StatusBadRequest && looksLikeSchemaUnsupported(msg),
			Retryable:         resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500,
		}
	}

	var parsed chatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, Usage{}, fmt.Errorf("parse response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, Usage{}, errors.New("llm: response contained no choices")
	}

	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	content = stripCodeFence(content)

	usage := Usage{
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
		TotalTokens:      parsed.Usage.TotalTokens,
	}
	return json.RawMessage(content), usage, nil
}

func looksLikeSchemaUnsupported(message string) bool {
	m := strings.ToLower(message)
	for _, needle := range []string{"response_format", "json_schema", "unsupported", "unknown parameter", "not supported"} {
		if strings.Contains(m, needle) {
			return true
		}
	}
	return false
}

// stripCodeFence strips a ```json ... ``` or ``` ... ``` wrapper some
// models add around structured output even when asked not to.
func stripCodeFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
