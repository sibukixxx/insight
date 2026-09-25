package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"insight-lab/internal/app"
	"insight-lab/internal/publicengine"
)

type command func(ctx context.Context, args []string, stderr io.Writer) (any, error)

var commands = map[string]command{
	"serve": nil, "help": nil,
	"engine":   runEngine,
	"subject":  group(map[string]command{"create": subjectCreate}),
	"evidence": group(map[string]command{"add": evidenceAdd}),
	"analysis": group(map[string]command{"start": analysisStart, "get": analysisGet, "results": analysisResults}),
	"research": group(map[string]command{"start": researchStart, "continue": researchContinue, "get": researchGet, "export": researchExport}),
	"status":   runStatus,
}

func group(subs map[string]command) command {
	return func(ctx context.Context, args []string, stderr io.Writer) (any, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("%w: missing subcommand", errUsage)
		}
		sub, ok := subs[args[0]]
		if !ok {
			return nil, fmt.Errorf("%w: unknown subcommand %q", errUsage, args[0])
		}
		return sub(ctx, args[1:], stderr)
	}
}

// engineCall parses engine flags plus command flags, opens the shared engine
// (app.Open, the same wiring the server uses) and runs fn against its Public
// Engine. Workers stop and the DB closes before returning.
func engineCall(ctx context.Context, name string, args []string, stderr io.Writer, flags func(*flag.FlagSet), fn func(context.Context, *publicengine.Engine) (any, error)) (any, error) {
	fs := flag.NewFlagSet("insight-lab "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	build := app.BindFlags(fs)
	if flags != nil {
		flags(fs)
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", errUsage, err)
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("%w: unexpected argument %q", errUsage, fs.Arg(0))
	}
	cfg, err := build()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	eng, err := app.Open(ctx, cfg)
	if err != nil {
		cancel()
		return nil, err
	}
	defer func() { cancel(); _ = eng.Close() }()
	return fn(ctx, eng.Public)
}

func required(pairs ...string) error {
	for i := 0; i < len(pairs); i += 2 {
		if pairs[i+1] == "" {
			return fmt.Errorf("%w: -%s is required", errUsage, pairs[i])
		}
	}
	return nil
}

func digest(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:32]
}

func randomKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func raw(_ int, body []byte, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return json.RawMessage(body), nil
}

func runEngine(ctx context.Context, args []string, stderr io.Writer) (any, error) {
	return engineCall(ctx, "engine", args, stderr, nil, func(_ context.Context, e *publicengine.Engine) (any, error) {
		return e.Engine(), nil
	})
}

func subjectCreate(ctx context.Context, args []string, stderr io.Writer) (any, error) {
	var ns, id, title string
	return engineCall(ctx, "subject create", args, stderr, func(fs *flag.FlagSet) {
		fs.StringVar(&ns, "namespace", "", "opaque subject namespace")
		fs.StringVar(&id, "id", "", "opaque subject id")
		fs.StringVar(&title, "title", "", "optional title")
	}, func(ctx context.Context, e *publicengine.Engine) (any, error) {
		if err := required("namespace", ns, "id", id); err != nil {
			return nil, err
		}
		return raw(e.CreateSubject(ctx, publicengine.CreateSubjectRequest{ContractVersion: publicengine.ContractVersion,
			IdempotencyKey: "cli-subject:" + digest(ns, id), Subject: publicengine.SubjectRef{Namespace: ns, ID: id}, Title: title}))
	})
}

func evidenceAdd(ctx context.Context, args []string, stderr io.Writer) (any, error) {
	var subject, document, source, artifact, request string
	return engineCall(ctx, "evidence add", args, stderr, func(fs *flag.FlagSet) {
		fs.StringVar(&subject, "subject", "", "subject id")
		fs.StringVar(&document, "document", "", "text evidence file")
		fs.StringVar(&source, "source", "document", "evidence source category of -document")
		fs.StringVar(&artifact, "artifact", "", "Analytical Artifact v1 JSON file")
		fs.StringVar(&request, "request", "", "full AddEvidenceRequest JSON file (documents, analyticalArtifacts, inputSources)")
	}, func(ctx context.Context, e *publicengine.Engine) (any, error) {
		if err := required("subject", subject); err != nil {
			return nil, err
		}
		req, err := evidenceRequest(document, source, artifact, request)
		if err != nil {
			return nil, err
		}
		return raw(e.AddEvidence(ctx, subject, req))
	})
}

func evidenceRequest(document, source, artifact, request string) (publicengine.AddEvidenceRequest, error) {
	var req publicengine.AddEvidenceRequest
	set := 0
	for _, v := range []string{document, artifact, request} {
		if v != "" {
			set++
		}
	}
	if set != 1 {
		return req, fmt.Errorf("%w: give exactly one of -document, -artifact, -request", errUsage)
	}
	path := document + artifact + request
	data, err := os.ReadFile(path)
	if err != nil {
		return req, fmt.Errorf("%w: %v", errUsage, err)
	}
	switch {
	case request != "":
		if err := json.Unmarshal(data, &req); err != nil {
			return req, fmt.Errorf("%w: -request is not an AddEvidenceRequest: %v", errUsage, err)
		}
	case artifact != "":
		req.AnalyticalArtifacts = []json.RawMessage{data}
	default:
		req.Documents = []publicengine.EvidenceDocument{{ExternalRef: filepath.Base(path), Source: source, Title: filepath.Base(path), Content: string(data)}}
	}
	if req.ContractVersion == "" {
		req.ContractVersion = publicengine.ContractVersion
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = "cli-evidence:" + digest(string(data))
	}
	return req, nil
}

var pollInterval = 100 * time.Millisecond
