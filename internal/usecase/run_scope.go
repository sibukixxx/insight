package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"insight-lab/internal/domain"
	"insight-lab/internal/service"
)

var (
	// ErrNoCompletedAnalysis means the project has no completed analysis run
	// to show. It is not ErrNotFound: the project exists, it simply has no
	// results yet.
	ErrNoCompletedAnalysis = errors.New("no completed analysis run is available")
	// ErrMixedAnalysisRuns means an iteration's insights come from more than
	// one analysis run. A report built from it would blend runs, so callers
	// fail closed instead of guessing which run is meant.
	ErrMixedAnalysisRuns = errors.New("research iteration insights span more than one analysis run")
)

// ResolveAnalysis returns the analysis run a result view is bound to. An
// explicit analysisID must belong to projectID; a run from another project is
// reported as ErrNotFound so its existence is not revealed. An empty
// analysisID resolves to the latest completed run, never to a queued, running
// or failed one, which have no results.
func (a *Application) ResolveAnalysis(ctx context.Context, projectID, analysisID string) (*domain.Analysis, error) {
	if err := a.RequireProject(ctx, projectID); err != nil {
		return nil, err
	}
	if analysisID != "" {
		analysis, err := a.repos.Analyses.Get(ctx, analysisID)
		if err != nil {
			return nil, err
		}
		if analysis.ProjectID != projectID {
			return nil, ErrNotFound
		}
		return analysis, nil
	}
	analysis, err := a.repos.Analyses.LatestCompletedByProject(ctx, projectID)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNoCompletedAnalysis
	}
	return analysis, err
}

// iterationAnalysis resolves the single analysis run that produced an
// iteration's insights. ok is false when the iteration carries no insight
// with a recorded analysis (legacy data). Insights from different runs, or
// from another project, yield ErrMixedAnalysisRuns.
func (a *Application) iterationAnalysis(ctx context.Context, projectID string, iteration domain.ResearchIteration) (*domain.Analysis, bool, error) {
	var analysisID string
	for _, id := range iteration.InsightIDs {
		insight, err := a.repos.Insights.Get(ctx, id)
		if err != nil {
			return nil, false, fmt.Errorf("resolve iteration insight %s: %w", id, err)
		}
		if insight.ProjectID != projectID {
			return nil, false, ErrMixedAnalysisRuns
		}
		if insight.AnalysisID == nil || *insight.AnalysisID == "" {
			if analysisID != "" {
				return nil, false, ErrMixedAnalysisRuns
			}
			continue
		}
		if analysisID != "" && analysisID != *insight.AnalysisID {
			return nil, false, ErrMixedAnalysisRuns
		}
		analysisID = *insight.AnalysisID
	}
	if analysisID == "" {
		return nil, false, nil
	}
	analysis, err := a.repos.Analyses.Get(ctx, analysisID)
	if err != nil {
		return nil, false, err
	}
	if analysis.ProjectID != projectID {
		return nil, false, ErrMixedAnalysisRuns
	}
	return analysis, true, nil
}

// iterationMetrics binds provenance to the analysis that produced this
// iteration's insights. A later project analysis must never silently change
// an older report or artifact. Missing or mixed references return false so
// callers leave provenance empty rather than export another run's values.
func (a *Application) iterationMetrics(ctx context.Context, projectID string, iteration domain.ResearchIteration) (service.Metrics, bool) {
	analysis, ok, err := a.iterationAnalysis(ctx, projectID, iteration)
	if err != nil || !ok {
		return service.Metrics{}, false
	}
	return completedMetrics(analysis)
}

// completedMetrics decodes the metrics a completed analysis recorded.
func completedMetrics(analysis *domain.Analysis) (service.Metrics, bool) {
	if analysis == nil || analysis.Status != domain.AnalysisCompleted || analysis.Metrics == "" {
		return service.Metrics{}, false
	}
	var metrics service.Metrics
	if json.Unmarshal([]byte(analysis.Metrics), &metrics) != nil {
		return service.Metrics{}, false
	}
	return metrics, true
}
