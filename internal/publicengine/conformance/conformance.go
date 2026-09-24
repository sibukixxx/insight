// Package conformance runs the shared Public Engine Contract fixtures
// (contracts/public-engine/v1/fixtures) against a live engine over plain
// HTTP. It deliberately does not use any SDK: the engine repository must not
// depend on SDK implementation repositories (#59). Standalone SDKs run the
// same fixture files with the same assertion rules.
package conformance

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Fixture is one conformance scenario.
type Fixture struct {
	Name            string `json:"fixture"`
	ContractVersion string `json:"contractVersion"`
	Covers          string `json:"covers"`
	// Engine is "deterministic" or "model_backed": which engine
	// configuration the scenario must run against.
	Engine string `json:"engine"`
	Steps  []Step `json:"steps"`
}

type Step struct {
	Op      string            `json:"op"`
	Params  map[string]string `json:"params"`
	Request json.RawMessage   `json:"request"`
	Expect  Expect            `json:"expect"`
	Save    map[string]string `json:"save"`
	Assert  []Assertion       `json:"assert"`
}

// Expect is either {"ok": true} or {"errorCode": ..., "httpStatus": ...}.
type Expect struct {
	OK         bool   `json:"ok"`
	ErrorCode  string `json:"errorCode"`
	HTTPStatus int    `json:"httpStatus"`
}

// Assertion checks one JSON path of the step result. Exactly one of Equals,
// Exists or MinLength applies.
type Assertion struct {
	Path      string          `json:"path"`
	Equals    json.RawMessage `json:"equals,omitempty"`
	Exists    *bool           `json:"exists,omitempty"`
	MinLength *int            `json:"minLength,omitempty"`
}

// Load reads every fixture in dir, sorted by file name.
func Load(dir string) ([]Fixture, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []Fixture
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var f Fixture
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		out = append(out, f)
	}
	return out, nil
}

// Run executes the fixture's steps in order and returns the first failure.
func Run(ctx context.Context, client *Client, f Fixture) error {
	vars := map[string]string{"uniq": uniq()}
	for i, step := range f.Steps {
		if err := runStep(ctx, client, step, vars); err != nil {
			return fmt.Errorf("%s step %d (%s): %w", f.Name, i+1, step.Op, err)
		}
	}
	return nil
}

func runStep(ctx context.Context, client *Client, step Step, vars map[string]string) error {
	params := map[string]string{}
	for k, v := range step.Params {
		params[k] = substitute(v, vars)
	}
	request := []byte(substitute(string(step.Request), vars))
	result, err := client.Call(ctx, step.Op, params, request)
	if step.Expect.ErrorCode != "" {
		var apiErr *Error
		if !errors.As(err, &apiErr) {
			return fmt.Errorf("expected error %s, got result (err=%v)", step.Expect.ErrorCode, err)
		}
		if apiErr.Code != step.Expect.ErrorCode || (step.Expect.HTTPStatus != 0 && apiErr.HTTPStatus != step.Expect.HTTPStatus) {
			return fmt.Errorf("expected %s/%d, got %s/%d: %s", step.Expect.ErrorCode, step.Expect.HTTPStatus, apiErr.Code, apiErr.HTTPStatus, apiErr.Message)
		}
		return nil
	}
	if err != nil {
		return err
	}
	for _, a := range step.Assert {
		if err := check(result, a, vars); err != nil {
			if reason, ok := lookup(result, "error"); ok {
				return fmt.Errorf("%w (result error: %v)", err, reason)
			}
			return err
		}
	}
	for name, path := range step.Save {
		value, ok := lookup(result, path)
		if !ok {
			return fmt.Errorf("save %s: path %q not found", name, path)
		}
		vars[name] = fmt.Sprint(value)
	}
	return nil
}

// Error is a contract error response.
type Error struct {
	HTTPStatus int
	Code       string
	Message    string
}

func (e *Error) Error() string { return fmt.Sprintf("%s (%d): %s", e.Code, e.HTTPStatus, e.Message) }

// Client is a minimal HTTP caller for fixture operations.
type Client struct {
	BaseURL      string
	HTTP         *http.Client
	PollInterval time.Duration
}

type route struct{ method, path string }

var routes = map[string]route{
	"getEngine":           {http.MethodGet, "/api/public/v1/engine"},
	"createSubject":       {http.MethodPost, "/api/public/v1/subjects"},
	"addEvidence":         {http.MethodPost, "/api/public/v1/subjects/{subjectId}/evidence"},
	"startAnalysis":       {http.MethodPost, "/api/public/v1/subjects/{subjectId}/analyses"},
	"getAnalysis":         {http.MethodGet, "/api/public/v1/subjects/{subjectId}/analyses/{analysisId}"},
	"getAnalysisResults":  {http.MethodGet, "/api/public/v1/subjects/{subjectId}/analyses/{analysisId}/results"},
	"createResearchRun":   {http.MethodPost, "/api/public/v1/subjects/{subjectId}/research-runs"},
	"appendIteration":     {http.MethodPost, "/api/public/v1/research-runs/{researchRunId}/iterations"},
	"getResearchRun":      {http.MethodGet, "/api/public/v1/research-runs/{researchRunId}"},
	"listAnalyses":        {http.MethodGet, "/api/public/v1/subjects/{subjectId}/analyses"},
	"compareAnalyses":     {http.MethodGet, "/api/public/v1/subjects/{subjectId}/analyses/{analysisId}/compare/{otherAnalysisId}"},
	"listResearchRuns":    {http.MethodGet, "/api/public/v1/subjects/{subjectId}/research-runs"},
	"reEvaluate":          {http.MethodPost, "/api/public/v1/research-runs/{researchRunId}/re-evaluations"},
	"getScenarios":        {http.MethodGet, "/api/public/v1/research-runs/{researchRunId}/scenarios"},
	"createScenarioSet":   {http.MethodPost, "/api/public/v1/research-runs/{researchRunId}/scenario-sets"},
	"scaffoldScenarioSet": {http.MethodPost, "/api/public/v1/research-runs/{researchRunId}/scenario-sets/scaffold"},
	"evaluateScenarios":   {http.MethodPost, "/api/public/v1/research-runs/{researchRunId}/scenario-sets/{scenarioSetId}/evaluations"},
}

// RegisterRoute lets later contract operations extend the runner.
func RegisterRoute(op, method, path string) { routes[op] = route{method, path} }

// Call executes one operation and returns decoded JSON.
func (c *Client) Call(ctx context.Context, op string, p map[string]string, request []byte) (any, error) {
	if op == "waitForAnalysis" {
		return c.waitForAnalysis(ctx, p)
	}
	r, ok := routes[op]
	if !ok {
		return nil, fmt.Errorf("unknown op %q", op)
	}
	path := r.path
	for k, v := range p {
		path = strings.ReplaceAll(path, "{"+k+"}", v)
	}
	var body io.Reader
	if r.method != http.MethodGet {
		if len(request) == 0 {
			request = []byte("{}")
		}
		body = bytes.NewReader(request)
	}
	req, err := http.NewRequestWithContext(ctx, r.method, strings.TrimRight(c.BaseURL, "/")+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		var e struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		return nil, &Error{HTTPStatus: resp.StatusCode, Code: e.Error.Code, Message: e.Error.Message}
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", op, err)
	}
	return doc, nil
}

func (c *Client) waitForAnalysis(ctx context.Context, p map[string]string) (any, error) {
	interval := c.PollInterval
	if interval <= 0 {
		interval = 50 * time.Millisecond
	}
	for {
		doc, err := c.Call(ctx, "getAnalysis", p, nil)
		if err != nil {
			return nil, err
		}
		if status, _ := lookup(doc, "status"); status != "queued" && status != "running" {
			return doc, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

var varPattern = regexp.MustCompile(`\$\{([A-Za-z0-9_]+)\}`)

func substitute(s string, vars map[string]string) string {
	return varPattern.ReplaceAllStringFunc(s, func(m string) string {
		if v, ok := vars[m[2:len(m)-1]]; ok {
			return v
		}
		return m
	})
}

var segmentPattern = regexp.MustCompile(`^([^\[]+)((?:\[\d+\])*)$`)
var indexPattern = regexp.MustCompile(`\[(\d+)\]`)

// lookup resolves "a.b[0].c" against decoded JSON.
func lookup(doc any, path string) (any, bool) {
	current := doc
	for _, part := range strings.Split(path, ".") {
		m := segmentPattern.FindStringSubmatch(part)
		if m == nil {
			return nil, false
		}
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		if current, ok = obj[m[1]]; !ok {
			return nil, false
		}
		for _, idx := range indexPattern.FindAllStringSubmatch(m[2], -1) {
			list, ok := current.([]any)
			n, _ := strconv.Atoi(idx[1])
			if !ok || n >= len(list) {
				return nil, false
			}
			current = list[n]
		}
	}
	return current, current != nil
}

func check(doc any, a Assertion, vars map[string]string) error {
	value, found := lookup(doc, a.Path)
	switch {
	case a.Exists != nil:
		if found != *a.Exists {
			return fmt.Errorf("%s: exists = %v, want %v", a.Path, found, *a.Exists)
		}
	case a.MinLength != nil:
		list, ok := value.([]any)
		if !ok || len(list) < *a.MinLength {
			return fmt.Errorf("%s: want at least %d items, got %v", a.Path, *a.MinLength, value)
		}
	case a.Equals != nil:
		var want any
		if err := json.Unmarshal([]byte(substitute(string(a.Equals), vars)), &want); err != nil {
			return err
		}
		if !found || !reflect.DeepEqual(value, want) {
			return fmt.Errorf("%s = %v, want %v", a.Path, value, want)
		}
	default:
		return fmt.Errorf("%s: assertion has no condition", a.Path)
	}
	return nil
}

func uniq() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
