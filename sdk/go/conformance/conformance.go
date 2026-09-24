// Package conformance runs the shared Public Engine Contract fixtures
// (contracts/public-engine/v1/fixtures) through the Go SDK against a live
// engine. The Node SDK runs the same files with the same rules, so both SDKs
// are held to identical behavior.
package conformance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	insight "github.com/sibukixxx/insight/sdk/go"
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
func Run(ctx context.Context, client *insight.Client, f Fixture) error {
	vars := map[string]string{"uniq": uniq()}
	for i, step := range f.Steps {
		if err := runStep(ctx, client, step, vars); err != nil {
			return fmt.Errorf("%s step %d (%s): %w", f.Name, i+1, step.Op, err)
		}
	}
	return nil
}

func runStep(ctx context.Context, client *insight.Client, step Step, vars map[string]string) error {
	params := map[string]string{}
	for k, v := range step.Params {
		params[k] = substitute(v, vars)
	}
	request := []byte(substitute(string(step.Request), vars))
	result, err := call(ctx, client, step.Op, params, request)
	if step.Expect.ErrorCode != "" {
		var apiErr *insight.Error
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
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	var doc any
	if err := json.Unmarshal(encoded, &doc); err != nil {
		return err
	}
	for _, a := range step.Assert {
		if err := check(doc, a, vars); err != nil {
			if reason, ok := lookup(doc, "error"); ok {
				return fmt.Errorf("%w (result error: %v)", err, reason)
			}
			return err
		}
	}
	for name, path := range step.Save {
		value, ok := lookup(doc, path)
		if !ok {
			return fmt.Errorf("save %s: path %q not found", name, path)
		}
		vars[name] = fmt.Sprint(value)
	}
	return nil
}

func call(ctx context.Context, c *insight.Client, op string, p map[string]string, request []byte) (any, error) {
	decode := func(dst any) error {
		if len(request) == 0 {
			return nil
		}
		return json.Unmarshal(request, dst)
	}
	switch op {
	case "getEngine":
		return c.GetEngine(ctx)
	case "createSubject":
		var req insight.CreateSubjectRequest
		if err := decode(&req); err != nil {
			return nil, err
		}
		return c.CreateSubject(ctx, req)
	case "addEvidence":
		var req insight.AddEvidenceRequest
		if err := decode(&req); err != nil {
			return nil, err
		}
		return c.AddEvidence(ctx, p["subjectId"], req)
	case "startAnalysis":
		var req insight.StartAnalysisRequest
		if err := decode(&req); err != nil {
			return nil, err
		}
		return c.StartAnalysis(ctx, p["subjectId"], req)
	case "getAnalysis":
		return c.GetAnalysis(ctx, p["subjectId"], p["analysisId"])
	case "waitForAnalysis":
		return c.WaitForAnalysis(ctx, p["subjectId"], p["analysisId"])
	case "getAnalysisResults":
		return c.GetAnalysisResults(ctx, p["subjectId"], p["analysisId"])
	case "createResearchRun":
		var req insight.CreateResearchRunRequest
		if err := decode(&req); err != nil {
			return nil, err
		}
		return c.CreateResearchRun(ctx, p["subjectId"], req)
	case "appendIteration":
		var req insight.AppendIterationRequest
		if err := decode(&req); err != nil {
			return nil, err
		}
		return c.AppendIteration(ctx, p["researchRunId"], req)
	case "getResearchRun":
		return c.GetResearchRun(ctx, p["researchRunId"])
	}
	return nil, fmt.Errorf("unknown op %q", op)
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
