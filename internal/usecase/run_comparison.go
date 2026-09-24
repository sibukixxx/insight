package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/service"
)

// ErrCrossProjectComparison means two runs from different projects were
// asked to be compared. Comparing them would mix unrelated evidence.
var ErrCrossProjectComparison = errors.New("analysis runs belong to different projects")

// ErrInvalidInput means a request is missing required identifiers.
var ErrInvalidInput = errors.New("invalid input")

// InstrumentChangeNote is recorded on a research iteration's Insight Delta
// when its analysis run used a different (or unrecorded) execution than the
// previous iteration's run.
const InstrumentChangeNote = "execution changed between iterations; result delta is confounded with instrument change"

// InstrumentUnknownNote is recorded when either iteration's execution was
// not recorded, so an instrument change cannot be ruled out.
const InstrumentUnknownNote = "execution was not recorded for one of the iterations; an instrument change cannot be ruled out"

// CompareAnalyses compares two analysis runs of projectID (issue #83).
func (a *Application) CompareAnalyses(ctx context.Context, projectID, fromID, toID string) (*service.RunComparison, error) {
	if fromID == "" || toID == "" {
		return nil, fmt.Errorf("%w: both analysis ids are required", ErrInvalidInput)
	}
	from, err := a.ResolveAnalysis(ctx, projectID, fromID)
	if err != nil {
		return nil, err
	}
	to, err := a.ResolveAnalysis(ctx, projectID, toID)
	if err != nil {
		return nil, err
	}
	history, err := a.repos.Analyses.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	fromInput, err := a.comparisonInput(ctx, from)
	if err != nil {
		return nil, err
	}
	toInput, err := a.comparisonInput(ctx, to)
	if err != nil {
		return nil, err
	}
	c := service.CompareAnalysisRuns(fromInput, toInput, history)
	return &c, nil
}

// CompareAnalysesByID compares two runs addressed only by ID; both must
// belong to the same project.
func (a *Application) CompareAnalysesByID(ctx context.Context, fromID, toID string) (*service.RunComparison, error) {
	from, err := a.repos.Analyses.Get(ctx, fromID)
	if err != nil {
		return nil, err
	}
	to, err := a.repos.Analyses.Get(ctx, toID)
	if err != nil {
		return nil, err
	}
	if from.ProjectID != to.ProjectID {
		return nil, ErrCrossProjectComparison
	}
	return a.CompareAnalyses(ctx, from.ProjectID, fromID, toID)
}

func (a *Application) comparisonInput(ctx context.Context, analysis *domain.Analysis) (service.RunComparisonInput, error) {
	in := service.RunComparisonInput{Analysis: analysis}
	if analysis.Status != domain.AnalysisCompleted {
		return in, nil
	}
	insights, err := a.repos.Insights.ListByAnalysis(ctx, analysis.ID)
	if err != nil {
		return in, err
	}
	for _, insight := range insights {
		evidence, err := a.repos.Evidence.ListByInsight(ctx, insight.ID)
		if err != nil {
			return in, fmt.Errorf("list evidence of %s: %w", insight.ID, err)
		}
		copied := *insight
		copied.Evidence = copied.Evidence[:0:0]
		for _, e := range evidence {
			copied.Evidence = append(copied.Evidence, *e)
		}
		in.Insights = append(in.Insights, &copied)
	}
	return in, nil
}

// MetricsHistoryEntry is one run in a project's metrics history.
type MetricsHistoryEntry struct {
	AnalysisID           string          `json:"analysisId"`
	Status               string          `json:"status"`
	Label                string          `json:"label,omitempty"`
	CreatedAt            string          `json:"createdAt"`
	InputFingerprint     string          `json:"inputFingerprint,omitempty"`
	ExecutionFingerprint string          `json:"executionFingerprint,omitempty"`
	Metrics              json.RawMessage `json:"metrics,omitempty"`
}

// MetricsHistory lists a project's runs oldest first with their recorded
// metrics and fingerprints. It ranks nothing.
func (a *Application) MetricsHistory(ctx context.Context, projectID string) ([]MetricsHistoryEntry, error) {
	runs, err := a.ListAnalyses(ctx, projectID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].CreatedAt.Before(runs[j].CreatedAt) })
	out := make([]MetricsHistoryEntry, 0, len(runs))
	for _, r := range runs {
		e := MetricsHistoryEntry{AnalysisID: r.ID, Status: string(r.Status), Label: r.Label,
			CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339Nano), InputFingerprint: r.InputFingerprint, ExecutionFingerprint: r.ExecutionFingerprint}
		if r.Status == domain.AnalysisCompleted && json.Valid([]byte(r.Metrics)) {
			e.Metrics = json.RawMessage(r.Metrics)
		}
		out = append(out, e)
	}
	return out, nil
}

// instrumentNote compares the execution of two iterations' analysis runs and
// returns the note to record, or "" when the execution is the same.
func (a *Application) instrumentNote(ctx context.Context, previousAnalysisID, currentAnalysisID string) string {
	if previousAnalysisID == "" || currentAnalysisID == "" || previousAnalysisID == currentAnalysisID {
		return ""
	}
	prev, err := a.repos.Analyses.Get(ctx, previousAnalysisID)
	if err != nil {
		return InstrumentUnknownNote
	}
	cur, err := a.repos.Analyses.Get(ctx, currentAnalysisID)
	if err != nil {
		return InstrumentUnknownNote
	}
	switch {
	case prev.ExecutionFingerprint == "" || cur.ExecutionFingerprint == "":
		return InstrumentUnknownNote
	case prev.ExecutionFingerprint != cur.ExecutionFingerprint:
		return InstrumentChangeNote
	}
	return ""
}
