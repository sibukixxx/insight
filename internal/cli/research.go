package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"insight-lab/internal/publicengine"
)

func analysisStart(ctx context.Context, args []string, stderr io.Writer) (any, error) {
	var subject, question, profile, reasoning, label, key string
	var timeout time.Duration
	return engineCall(ctx, "analysis start", args, stderr, func(fs *flag.FlagSet) {
		fs.StringVar(&subject, "subject", "", "subject id")
		fs.StringVar(&question, "research-question", "", "focus the analysis on this research question")
		fs.StringVar(&reasoning, "reasoning-profile", "", "GENERAL_RESEARCH (default) or CUSTOMER_INSIGHT")
		fs.StringVar(&profile, "execution-profile", "", "AUTO (default), LIGHT, STANDARD or HEAVY")
		fs.StringVar(&label, "label", "", "optional run label")
		fs.StringVar(&key, "idempotency-key", "", "reuse to replay the same start (default: new analysis)")
		fs.DurationVar(&timeout, "timeout", 30*time.Minute, "maximum time to wait for the analysis")
	}, func(ctx context.Context, e *publicengine.Engine) (any, error) {
		if err := required("subject", subject); err != nil {
			return nil, err
		}
		if key == "" {
			key = randomKey()
		}
		_, body, err := e.StartAnalysis(ctx, subject, publicengine.StartAnalysisRequest{ContractVersion: publicengine.ContractVersion,
			IdempotencyKey: key, Label: label, ResearchQuestion: question, ReasoningProfile: reasoning, ExecutionProfile: profile})
		if err != nil {
			return nil, err
		}
		var run publicengine.AnalysisRun
		if err := json.Unmarshal(body, &run); err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		for run.Status == "queued" || run.Status == "running" {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("waiting for analysis %s: %w", run.AnalysisID, ctx.Err())
			case <-time.After(pollInterval):
			}
			if run, err = e.GetAnalysis(ctx, subject, run.AnalysisID); err != nil {
				return nil, err
			}
		}
		if run.Status == "failed" {
			return nil, fmt.Errorf("%w: %s: %s", errAnalysisFailed, run.AnalysisID, run.Error)
		}
		return run, nil
	})
}

func analysisGet(ctx context.Context, args []string, stderr io.Writer) (any, error) {
	var subject, analysis string
	return engineCall(ctx, "analysis get", args, stderr, subjectAnalysisFlags(&subject, &analysis), func(ctx context.Context, e *publicengine.Engine) (any, error) {
		if err := required("subject", subject, "analysis", analysis); err != nil {
			return nil, err
		}
		return e.GetAnalysis(ctx, subject, analysis)
	})
}

func analysisResults(ctx context.Context, args []string, stderr io.Writer) (any, error) {
	var subject, analysis string
	return engineCall(ctx, "analysis results", args, stderr, subjectAnalysisFlags(&subject, &analysis), func(ctx context.Context, e *publicengine.Engine) (any, error) {
		if err := required("subject", subject, "analysis", analysis); err != nil {
			return nil, err
		}
		return e.GetAnalysisResults(ctx, subject, analysis)
	})
}

func subjectAnalysisFlags(subject, analysis *string) func(*flag.FlagSet) {
	return func(fs *flag.FlagSet) {
		fs.StringVar(subject, "subject", "", "subject id")
		fs.StringVar(analysis, "analysis", "", "analysis id")
	}
}

func researchStart(ctx context.Context, args []string, stderr io.Writer) (any, error) {
	var subject, analysis, question string
	return engineCall(ctx, "research start", args, stderr, func(fs *flag.FlagSet) {
		subjectAnalysisFlags(&subject, &analysis)(fs)
		fs.StringVar(&question, "question", "", "research question")
	}, func(ctx context.Context, e *publicengine.Engine) (any, error) {
		if err := required("subject", subject, "analysis", analysis, "question", question); err != nil {
			return nil, err
		}
		return raw(e.CreateResearchRun(ctx, subject, publicengine.CreateResearchRunRequest{ContractVersion: publicengine.ContractVersion,
			IdempotencyKey: "cli-research:" + digest(subject, analysis, question), Question: question, AnalysisID: analysis}))
	})
}

func researchContinue(ctx context.Context, args []string, stderr io.Writer) (any, error) {
	var run, analysis, question string
	return engineCall(ctx, "research continue", args, stderr, func(fs *flag.FlagSet) {
		fs.StringVar(&run, "run", "", "research run id")
		fs.StringVar(&analysis, "analysis", "", "analysis id for the new iteration")
		fs.StringVar(&question, "question", "", "optional refined question")
	}, func(ctx context.Context, e *publicengine.Engine) (any, error) {
		if err := required("run", run, "analysis", analysis); err != nil {
			return nil, err
		}
		return raw(e.AppendIteration(ctx, run, publicengine.AppendIterationRequest{ContractVersion: publicengine.ContractVersion,
			IdempotencyKey: "cli-iteration:" + digest(run, analysis, question), AnalysisID: analysis, Question: question}))
	})
}

func researchGet(ctx context.Context, args []string, stderr io.Writer) (any, error) {
	var run string
	return engineCall(ctx, "research get", args, stderr, func(fs *flag.FlagSet) {
		fs.StringVar(&run, "run", "", "research run id")
	}, func(ctx context.Context, e *publicengine.Engine) (any, error) {
		if err := required("run", run); err != nil {
			return nil, err
		}
		return e.GetResearchRun(ctx, run)
	})
}

// researchExport writes the versioned Research Artifact (the machine-readable
// research result) of the latest iteration.
func researchExport(ctx context.Context, args []string, stderr io.Writer) (any, error) {
	var run, out string
	return engineCall(ctx, "research export", args, stderr, func(fs *flag.FlagSet) {
		fs.StringVar(&run, "run", "", "research run id")
		fs.StringVar(&out, "out", "", "write the Research Artifact to this file instead of stdout")
	}, func(ctx context.Context, e *publicengine.Engine) (any, error) {
		if err := required("run", run); err != nil {
			return nil, err
		}
		res, err := e.GetResearchRun(ctx, run)
		if err != nil {
			return nil, err
		}
		if out == "" {
			return res.Artifact, nil
		}
		if err := os.WriteFile(out, append(append([]byte{}, res.Artifact...), '\n'), 0o644); err != nil {
			return nil, err
		}
		return map[string]string{"researchRunId": run, "iterationId": res.IterationID, "written": out}, nil
	})
}

func runStatus(ctx context.Context, args []string, stderr io.Writer) (any, error) {
	var subject string
	return engineCall(ctx, "status", args, stderr, func(fs *flag.FlagSet) {
		fs.StringVar(&subject, "subject", "", "subject id")
	}, func(ctx context.Context, e *publicengine.Engine) (any, error) {
		if err := required("subject", subject); err != nil {
			return nil, err
		}
		analyses, err := e.ListAnalyses(ctx, subject)
		if err != nil {
			return nil, err
		}
		runs, err := e.ListResearchRuns(ctx, subject)
		if err != nil {
			return nil, err
		}
		return map[string]any{"subjectId": subject, "analyses": analyses.Analyses, "researchRuns": runs.ResearchRuns}, nil
	})
}
