// Package insight is the Go SDK for the Insight Lab Public Engine Contract v1
// (contracts/public-engine/v1). It is a thin HTTP/JSON client: it holds no
// research logic, scoring or domain policy, and it never imports Insight's
// internal packages.
package insight

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultPollInterval = 500 * time.Millisecond

// Client calls one Insight Lab engine. It is safe for concurrent use.
type Client struct {
	baseURL      string
	httpClient   *http.Client
	pollInterval time.Duration
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient sets the HTTP client, e.g. to configure timeouts or TLS.
func WithHTTPClient(c *http.Client) Option { return func(cl *Client) { cl.httpClient = c } }

// WithPollInterval sets how often WaitForAnalysis polls.
func WithPollInterval(d time.Duration) Option { return func(cl *Client) { cl.pollInterval = d } }

// NewClient returns a client for the engine at baseURL, e.g.
// "http://127.0.0.1:8787". The contract path is appended by the client.
func NewClient(baseURL string, opts ...Option) *Client {
	c := &Client{baseURL: strings.TrimRight(baseURL, "/"), httpClient: http.DefaultClient, pollInterval: defaultPollInterval}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// NewIdempotencyKey returns a random key. Keep and reuse the key when you
// retry the same request so the engine can replay the first result.
func NewIdempotencyKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// GetEngine returns the engine's contract and build identity.
func (c *Client) GetEngine(ctx context.Context) (*EngineInfo, error) {
	var out EngineInfo
	return &out, c.do(ctx, http.MethodGet, "/engine", nil, &out)
}

// CreateSubject creates a subject, or returns the existing one created with
// the same opaque reference and content.
func (c *Client) CreateSubject(ctx context.Context, req CreateSubjectRequest) (*Subject, error) {
	fillEnvelope(&req.ContractVersion, &req.IdempotencyKey)
	var out Subject
	return &out, c.do(ctx, http.MethodPost, "/subjects", req, &out)
}

// AddEvidence adds documents and Analytical Artifacts to a subject.
func (c *Client) AddEvidence(ctx context.Context, subjectID string, req AddEvidenceRequest) (*EvidenceReceipt, error) {
	fillEnvelope(&req.ContractVersion, &req.IdempotencyKey)
	var out EvidenceReceipt
	return &out, c.do(ctx, http.MethodPost, "/subjects/"+url.PathEscape(subjectID)+"/evidence", req, &out)
}

// StartAnalysis starts an analysis run over the subject's current evidence.
// The run executes asynchronously; see WaitForAnalysis.
func (c *Client) StartAnalysis(ctx context.Context, subjectID string, req StartAnalysisRequest) (*AnalysisRun, error) {
	fillEnvelope(&req.ContractVersion, &req.IdempotencyKey)
	var out AnalysisRun
	return &out, c.do(ctx, http.MethodPost, "/subjects/"+url.PathEscape(subjectID)+"/analyses", req, &out)
}

// GetAnalysis returns the current state of an analysis run.
func (c *Client) GetAnalysis(ctx context.Context, subjectID, analysisID string) (*AnalysisRun, error) {
	var out AnalysisRun
	return &out, c.do(ctx, http.MethodGet, "/subjects/"+url.PathEscape(subjectID)+"/analyses/"+url.PathEscape(analysisID), nil, &out)
}

// WaitForAnalysis polls until the run completes or fails, or ctx ends. A
// failed run is returned without an error; inspect Status and Error.
func (c *Client) WaitForAnalysis(ctx context.Context, subjectID, analysisID string) (*AnalysisRun, error) {
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()
	for {
		run, err := c.GetAnalysis(ctx, subjectID, analysisID)
		if err != nil {
			return nil, err
		}
		if run.Terminal() {
			return run, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// GetAnalysisResults returns the observations and findings of one completed
// run.
func (c *Client) GetAnalysisResults(ctx context.Context, subjectID, analysisID string) (*AnalysisResults, error) {
	var out AnalysisResults
	return &out, c.do(ctx, http.MethodGet, "/subjects/"+url.PathEscape(subjectID)+"/analyses/"+url.PathEscape(analysisID)+"/results", nil, &out)
}

// CreateResearchRun starts research on one completed analysis run.
func (c *Client) CreateResearchRun(ctx context.Context, subjectID string, req CreateResearchRunRequest) (*ResearchResult, error) {
	fillEnvelope(&req.ContractVersion, &req.IdempotencyKey)
	var out ResearchResult
	return &out, c.do(ctx, http.MethodPost, "/subjects/"+url.PathEscape(subjectID)+"/research-runs", req, &out)
}

// AppendIteration evaluates a research run again on a newer completed
// analysis run. Earlier iterations are never modified.
func (c *Client) AppendIteration(ctx context.Context, researchRunID string, req AppendIterationRequest) (*ResearchResult, error) {
	fillEnvelope(&req.ContractVersion, &req.IdempotencyKey)
	var out ResearchResult
	return &out, c.do(ctx, http.MethodPost, "/research-runs/"+url.PathEscape(researchRunID)+"/iterations", req, &out)
}

// GetResearchRun returns the latest iteration of a research run.
func (c *Client) GetResearchRun(ctx context.Context, researchRunID string) (*ResearchResult, error) {
	var out ResearchResult
	return &out, c.do(ctx, http.MethodGet, "/research-runs/"+url.PathEscape(researchRunID), nil, &out)
}

func fillEnvelope(version, key *string) {
	if *version == "" {
		*version = ContractVersion
	}
	if *key == "" {
		*key = NewIdempotencyKey()
	}
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return &Error{Code: CodeInvalidRequest, Message: "encode request: " + err.Error(), cause: err}
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/api/public/v1"+path, reader)
	if err != nil {
		return &Error{Code: CodeInvalidRequest, Message: err.Error(), cause: err}
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return &Error{Code: CodeUnavailable, Message: err.Error(), cause: err}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return &Error{Code: CodeUnavailable, Message: "read response: " + err.Error(), HTTPStatus: resp.StatusCode, cause: err}
	}
	if resp.StatusCode >= 300 {
		// Only an error status carries an error body; a successful
		// AnalysisRun has an unrelated "error" string for failed runs.
		var failure ErrorResponse
		if json.Unmarshal(data, &failure) != nil || failure.Error.Code == "" {
			return &Error{Code: CodeUnavailable, Message: fmt.Sprintf("HTTP %d without a contract error", resp.StatusCode), HTTPStatus: resp.StatusCode}
		}
		return &Error{Code: failure.Error.Code, Message: failure.Error.Message, HTTPStatus: resp.StatusCode}
	}
	var envelope struct {
		ContractVersion string `json:"contractVersion"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return &Error{Code: CodeUnavailable, Message: fmt.Sprintf("response is not a contract message (HTTP %d)", resp.StatusCode), HTTPStatus: resp.StatusCode, cause: err}
	}
	if envelope.ContractVersion != ContractVersion {
		return &Error{Code: CodeUnsupportedContractVersion, Message: fmt.Sprintf("engine answered with contract version %q; this SDK supports %q", envelope.ContractVersion, ContractVersion)}
	}
	if err := json.Unmarshal(data, out); err != nil {
		return &Error{Code: CodeUnavailable, Message: "decode response: " + err.Error(), HTTPStatus: resp.StatusCode, cause: err}
	}
	return nil
}
