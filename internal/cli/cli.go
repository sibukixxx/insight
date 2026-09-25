// Package cli is the headless operator surface of Insight (#96). It is a thin
// layer over the same wired engine the HTTP server uses (app.Open): every
// command calls the Public Engine Contract operations of publicengine, so the
// CLI adds no research semantics of its own. Output is JSON on stdout;
// failures are a JSON error on stderr with a meaningful exit code.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"insight-lab/internal/app"
	"insight-lab/internal/publicengine"
)

// Exit codes. They are part of the CLI contract for automation.
const (
	ExitOK                    = 0
	ExitInternal              = 1
	ExitUsage                 = 2
	ExitNotFound              = 3
	ExitRejected              = 4 // invalid request / unsupported version / verification failed
	ExitConflict              = 5 // idempotency/identity/state conflicts, research refused
	ExitCapabilityUnavailable = 6 // profile / model binding / input source not configured
	ExitAnalysisFailed        = 7
)

// errUsage marks command-line misuse.
var errUsage = errors.New("usage")

// errAnalysisFailed marks an analysis that finished with status failed.
var errAnalysisFailed = errors.New("analysis failed")

// Serve runs the HTTP server (API and optional Reference Web).
type Serve func(ctx context.Context, cfg *app.Config) error

// Main dispatches args and returns the process exit code. Arguments that do
// not start with a known command keep the historical `insight-lab [flags]`
// behavior: they start the server.
func Main(ctx context.Context, args []string, stdout, stderr io.Writer, serve Serve) int {
	name, rest := "serve", args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, rest = args[0], args[1:]
	}
	cmd, ok := commands[name]
	if !ok {
		fmt.Fprintf(stderr, "insight-lab: unknown command %q\n\n%s", name, usage)
		return ExitUsage
	}
	if name == "help" {
		fmt.Fprint(stdout, usage)
		return ExitOK
	}
	if name == "serve" {
		fs := flag.NewFlagSet("insight-lab serve", flag.ContinueOnError)
		fs.SetOutput(stderr)
		build := app.BindFlags(fs)
		if err := fs.Parse(rest); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return ExitOK
			}
			return ExitUsage
		}
		cfg, err := build()
		if err == nil {
			err = serve(ctx, cfg)
		}
		if err != nil {
			fmt.Fprintln(stderr, "insight-lab:", err)
			return ExitInternal
		}
		return ExitOK
	}
	out, err := cmd(ctx, rest, stderr)
	if err != nil {
		return fail(stderr, err)
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if raw, ok := out.(json.RawMessage); ok {
		var v any
		if json.Unmarshal(raw, &v) == nil {
			out = v
		}
	}
	if err := enc.Encode(out); err != nil {
		return fail(stderr, err)
	}
	return ExitOK
}

type errorOutput struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	ExitCode int `json:"exitCode"`
}

func fail(stderr io.Writer, err error) int {
	var out errorOutput
	switch {
	case errors.Is(err, flag.ErrHelp):
		return ExitOK
	case errors.Is(err, errUsage):
		out.Error.Code, out.Error.Message, out.ExitCode = "USAGE", err.Error(), ExitUsage
	case errors.Is(err, errAnalysisFailed):
		out.Error.Code, out.Error.Message, out.ExitCode = "ANALYSIS_FAILED", err.Error(), ExitAnalysisFailed
	default:
		ce := publicengine.AsError(err)
		out.Error.Code, out.Error.Message, out.ExitCode = string(ce.Code), ce.Message, exitCodeFor(ce.Code)
	}
	b, _ := json.Marshal(out)
	fmt.Fprintln(stderr, string(b))
	return out.ExitCode
}

func exitCodeFor(code publicengine.Code) int {
	switch code {
	case publicengine.CodeNotFound:
		return ExitNotFound
	case publicengine.CodeInvalidRequest, publicengine.CodeUnsupportedContractVersion, publicengine.CodeInputVerificationFailed:
		return ExitRejected
	case publicengine.CodeIdempotencyConflict, publicengine.CodeIdentityConflict, publicengine.CodeStaleIteration,
		publicengine.CodeMixedAnalysisRuns, publicengine.CodeAnalysisNotCompleted, publicengine.CodeAnalysisHasNoHypotheses:
		return ExitConflict
	case publicengine.CodeExecutionProfileUnavailable, publicengine.CodeModelBindingUnavailable, publicengine.CodeInputSourceUnavailable:
		return ExitCapabilityUnavailable
	}
	return ExitInternal
}

const usage = `Insight Lab — evidence-first research engine

Usage:
  insight-lab [serve] [engine flags]          start the API server (+ Reference Web unless -no-web)
  insight-lab engine   [engine flags]         engine capabilities (JSON)
  insight-lab subject create  -namespace N -id ID [-title T]
  insight-lab evidence add    -subject S (-document FILE [-source document] | -artifact FILE | -request FILE)
  insight-lab analysis start  -subject S [-research-question Q] [-reasoning-profile GENERAL_RESEARCH|CUSTOMER_INSIGHT] [-execution-profile AUTO|LIGHT|STANDARD|HEAVY] [-label L]
  insight-lab analysis get     -subject S -analysis A
  insight-lab analysis results -subject S -analysis A
  insight-lab research start    -subject S -analysis A -question Q
  insight-lab research continue -run R -analysis A [-question Q]
  insight-lab research get      -run R
  insight-lab research export   -run R [-out FILE]   (Research Artifact JSON)
  insight-lab status  -subject S

Engine flags (all commands): -db, -api-key, -model, -base-url, -input-root, -heavy-dir, -allowed-models
Server flags: -host, -port, -no-browser, -no-web, -demo, -client

Output is JSON on stdout. Errors are JSON on stderr with exit codes:
  0 ok · 1 internal · 2 usage · 3 not found · 4 rejected request · 5 conflict/refused
  6 capability unavailable · 7 analysis failed
Headless commands run no HTTP server and open no browser. 'analysis start' waits
for the analysis to finish in-process.
`
