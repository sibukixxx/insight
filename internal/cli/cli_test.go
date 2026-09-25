package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"insight-lab/internal/app"
	"insight-lab/internal/llm/scripted"
)

type result struct {
	code   int
	stdout map[string]any
	stderr string
}

func run(t *testing.T, args ...string) result {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Main(context.Background(), args, &out, &errOut, func(context.Context, *app.Config) error {
		t.Fatal("headless commands must not start the server")
		return nil
	})
	r := result{code: code, stderr: errOut.String()}
	if out.Len() > 0 {
		if err := json.Unmarshal(out.Bytes(), &r.stdout); err != nil {
			t.Fatalf("stdout is not JSON: %v\n%s", err, out.String())
		}
	}
	return r
}

func errorCode(t *testing.T, r result) string {
	t.Helper()
	var e errorOutput
	if err := json.Unmarshal([]byte(r.stderr), &e); err != nil {
		t.Fatalf("stderr is not a JSON error: %q", r.stderr)
	}
	if e.ExitCode != r.code {
		t.Fatalf("stderr exitCode %d != process exit %d", e.ExitCode, r.code)
	}
	return e.Error.Code
}

func TestMainReturnsUsageExitForUnknownCommandAndMissingFlags(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Main(context.Background(), []string{"frobnicate"}, &out, &errOut, nil); code != ExitUsage {
		t.Fatalf("unknown command exit = %d", code)
	}
	db := filepath.Join(t.TempDir(), "x.db")
	r := run(t, "analysis", "get", "-db", db, "-subject", "s")
	if r.code != ExitUsage || errorCode(t, r) != "USAGE" {
		t.Fatalf("missing -analysis: %+v", r)
	}
}

func TestMainKeepsLegacyFlagsStartingTheServerWithOptionalWeb(t *testing.T) {
	var got *app.Config
	var out, errOut bytes.Buffer
	code := Main(context.Background(), []string{"-port", "9999", "-no-web", "-db", filepath.Join(t.TempDir(), "s.db")}, &out, &errOut,
		func(_ context.Context, cfg *app.Config) error { got = cfg; return nil })
	if code != ExitOK || got == nil || got.Port != 9999 || !got.NoWeb {
		t.Fatalf("legacy invocation: code=%d cfg=%+v stderr=%s", code, got, errOut.String())
	}
	got = nil
	if code := Main(context.Background(), []string{"serve", "-port", "9998"}, &out, &errOut,
		func(_ context.Context, cfg *app.Config) error { got = cfg; return nil }); code != ExitOK || got.Port != 9998 || got.NoWeb {
		t.Fatalf("serve invocation: code=%d cfg=%+v", code, got)
	}
}

func TestMainMapsContractErrorsToExitCodes(t *testing.T) {
	db := filepath.Join(t.TempDir(), "e.db")
	r := run(t, "analysis", "get", "-db", db, "-subject", "missing", "-analysis", "missing")
	if r.code != ExitNotFound || errorCode(t, r) != "NOT_FOUND" {
		t.Fatalf("not found: %+v", r)
	}
	s := run(t, "subject", "create", "-db", db, "-namespace", "cli", "-id", "subject-1")
	doc := filepath.Join(t.TempDir(), "note.txt")
	_ = os.WriteFile(doc, []byte("Sales rose after the change."), 0o600)
	run(t, "evidence", "add", "-db", db, "-subject", s.stdout["subjectId"].(string), "-document", doc)
	// No model is configured and the evidence is plain text: the deterministic
	// pipeline cannot analyze it, so the analysis fails.
	r = run(t, "analysis", "start", "-db", db, "-subject", s.stdout["subjectId"].(string))
	if r.code != ExitAnalysisFailed || errorCode(t, r) != "ANALYSIS_FAILED" {
		t.Fatalf("failed analysis: %+v", r)
	}
}

// A full research flow runs headlessly: subject, evidence, question-focused
// model-backed analysis (scripted, deterministic stand-in model), research
// run, machine-readable artifact export and status. No server, no browser.
func TestHeadlessResearchFlowEndToEnd(t *testing.T) {
	llm := httptest.NewServer(scripted.Handler())
	defer llm.Close()
	dir := t.TempDir()
	db := filepath.Join(dir, "flow.db")
	model := []string{"-db", db, "-base-url", llm.URL, "-model", "scripted-model", "-api-key", "scripted"}
	with := func(args ...string) []string { return append(args, model...) }

	info := run(t, with("engine")...)
	if info.code != ExitOK || info.stdout["modelBacked"] != true {
		t.Fatalf("engine: %+v", info)
	}
	subj := run(t, with("subject", "create", "-namespace", "cli", "-id", "flow")...)
	sid, _ := subj.stdout["subjectId"].(string)
	doc := filepath.Join(dir, "evidence.txt")
	_ = os.WriteFile(doc, []byte("Exports rose after the tariff cut.\nThis customer contradicts that trend."), 0o600)
	if r := run(t, with("evidence", "add", "-subject", sid, "-document", doc)...); r.code != ExitOK {
		t.Fatalf("evidence: %+v", r)
	}
	an := run(t, with("analysis", "start", "-subject", sid, "-research-question", "Why did exports rise?", "-execution-profile", "LIGHT", "-timeout", (30*time.Second).String())...)
	if an.code != ExitOK || an.stdout["status"] != "completed" {
		t.Fatalf("analysis: %+v", an)
	}
	aid := an.stdout["analysisId"].(string)
	rr := run(t, with("research", "start", "-subject", sid, "-analysis", aid, "-question", "Why did exports rise?")...)
	if rr.code != ExitOK {
		t.Fatalf("research start: %+v", rr)
	}
	runID := rr.stdout["researchRunId"].(string)
	out := filepath.Join(dir, "artifact.json")
	if r := run(t, with("research", "export", "-run", runID, "-out", out)...); r.code != ExitOK {
		t.Fatalf("export: %+v", r)
	}
	var artifact map[string]any
	data, _ := os.ReadFile(out)
	if err := json.Unmarshal(data, &artifact); err != nil || artifact["artifactSchema"] != "insight-lab.research-artifact" {
		t.Fatalf("exported artifact: %v %s", err, data)
	}
	st := run(t, with("status", "-subject", sid)...)
	if st.code != ExitOK || len(st.stdout["analyses"].([]any)) != 1 || len(st.stdout["researchRuns"].([]any)) != 1 {
		t.Fatalf("status: %+v", st)
	}
}
