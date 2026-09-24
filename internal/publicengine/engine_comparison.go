package publicengine

import (
	"context"
	"net/http"
	"sort"
	"strings"

	"insight-lab/internal/domain"
	"insight-lab/internal/usecase"
)

// ListAnalyses lists every analysis run of the subject, oldest first, with
// fingerprints and provenance (#81/#82).
func (e *Engine) ListAnalyses(ctx context.Context, subjectID string) (AnalysisList, error) {
	subject, err := e.requireSubject(ctx, subjectID)
	if err != nil {
		return AnalysisList{}, err
	}
	runs, err := e.app.ListAnalyses(ctx, subject.ProjectID)
	if err != nil {
		return AnalysisList{}, err
	}
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].CreatedAt.Before(runs[j].CreatedAt) })
	out := AnalysisList{ContractVersion: ContractVersion, SubjectID: subjectID, Analyses: []AnalysisRun{}}
	for _, r := range runs {
		out.Analyses = append(out.Analyses, toAnalysisRun(subjectID, r))
	}
	return out, nil
}

// CompareAnalyses compares two runs of the subject (#83). It attributes
// differences to input or execution without judging which run is better.
func (e *Engine) CompareAnalyses(ctx context.Context, subjectID, fromID, toID string) (RunComparisonResult, error) {
	subject, err := e.requireSubject(ctx, subjectID)
	if err != nil {
		return RunComparisonResult{}, err
	}
	c, err := e.app.CompareAnalyses(ctx, subject.ProjectID, fromID, toID)
	if err != nil {
		return RunComparisonResult{}, err
	}
	return RunComparisonResult{ContractVersion: ContractVersion, SubjectID: subjectID, Comparison: *c}, nil
}

// ListResearchRuns lists the subject's research runs, oldest first.
func (e *Engine) ListResearchRuns(ctx context.Context, subjectID string) (ResearchRunList, error) {
	subject, err := e.requireSubject(ctx, subjectID)
	if err != nil {
		return ResearchRunList{}, err
	}
	runs, err := e.app.ListResearchRuns(ctx, subject.ProjectID)
	if err != nil {
		return ResearchRunList{}, err
	}
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].CreatedAt.Before(runs[j].CreatedAt) })
	out := ResearchRunList{ContractVersion: ContractVersion, SubjectID: subjectID, ResearchRuns: []ResearchRunSummary{}}
	for _, r := range runs {
		s := ResearchRunSummary{ResearchRunID: r.ID, Question: r.Question, IterationCount: len(r.Iterations), CreatedAt: formatTime(r.CreatedAt)}
		if latest, ok := r.LatestIteration(); ok {
			s.LatestIterationID, s.LatestIterationSequence = latest.ID, latest.Sequence
		}
		out.ResearchRuns = append(out.ResearchRuns, s)
	}
	return out, nil
}

// ReEvaluate re-evaluates a research run after new evidence arrived (#74).
// It returns 201 when a new iteration was appended and 200 otherwise.
func (e *Engine) ReEvaluate(ctx context.Context, researchRunID string, req ReEvaluationRequest) (int, []byte, error) {
	return e.idempotent(ctx, req.ContractVersion, req.IdempotencyKey, "reEvaluate:"+researchRunID, req, func() (int, any, error) {
		run, err := e.publicResearchRun(ctx, researchRunID)
		if err != nil {
			return 0, nil, err
		}
		if len(req.CorrelationKey) > maxIdentifierLength || len(req.Note) > maxQuestionLength || len(req.Trigger.Source) > maxIdentifierLength {
			return 0, nil, newError(CodeInvalidRequest, "correlationKey and trigger.source are limited to %d and note to %d characters", maxIdentifierLength, maxQuestionLength)
		}
		for _, list := range [][]string{req.EvidenceChanges.Added, req.EvidenceChanges.Removed, req.EvidenceChanges.Changed, req.AffectedGapIDs} {
			if len(list) > maxDocumentsPerRequest {
				return 0, nil, newError(CodeInvalidRequest, "evidence change lists are limited to %d entries", maxDocumentsPerRequest)
			}
		}
		outcome, err := e.app.ReEvaluate(ctx, usecase.ReEvaluateInput{
			RunID: run.ID, CorrelationKey: strings.TrimSpace(req.CorrelationKey), PreviousIterationID: req.PreviousIterationID,
			AnalysisID: req.AnalysisID, Trigger: domain.ReEvaluationTrigger{Kind: req.Trigger.Kind, Source: req.Trigger.Source},
			EvidenceChanges: req.EvidenceChanges, AffectedGapIDs: req.AffectedGapIDs, Note: req.Note,
		})
		if err != nil {
			return 0, nil, err
		}
		result, err := e.researchResult(ctx, run.ProjectID, outcome.Run)
		if err != nil {
			return 0, nil, err
		}
		status := http.StatusOK
		if outcome.Status == usecase.ReEvaluationNewIteration {
			status = http.StatusCreated
		}
		return status, ReEvaluationResult{
			ContractVersion: ContractVersion, SubjectID: run.ProjectID, ResearchRunID: run.ID,
			Status: string(outcome.Status), ReEvaluation: outcome.Record, Result: result,
		}, nil
	})
}
