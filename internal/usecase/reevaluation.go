package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"insight-lab/internal/domain"
)

var (
	// ErrStaleIteration means the caller based its re-evaluation on an
	// iteration that is no longer the latest one.
	ErrStaleIteration = errors.New("previous iteration is not the latest iteration of the research run")
	// ErrReEvaluationConflict means a correlation key was reused for a
	// different analysis run.
	ErrReEvaluationConflict = errors.New("correlation key was already used for a different analysis run")
)

// ReEvaluationStatus is the outcome of a re-evaluation request.
type ReEvaluationStatus string

const (
	ReEvaluationNewIteration     ReEvaluationStatus = "NEW_ITERATION"
	ReEvaluationNoEvidenceChange ReEvaluationStatus = "NO_EVIDENCE_CHANGE"
	ReEvaluationAlreadyEvaluated ReEvaluationStatus = "ALREADY_EVALUATED"
)

const fullScopeReason = "partial re-evaluation is not yet proven safe; the whole iteration was re-evaluated from the new analysis run"

type ReEvaluateInput struct {
	RunID               string
	CorrelationKey      string
	PreviousIterationID string
	AnalysisID          string
	Trigger             domain.ReEvaluationTrigger
	EvidenceChanges     domain.EvidenceChanges
	AffectedGapIDs      []string
	Note                string
}

type ReEvaluationOutcome struct {
	Status ReEvaluationStatus
	Record domain.ReEvaluation
	Run    *domain.ResearchRun
}

// ReEvaluate re-evaluates a research run because new evidence arrived
// (issue #74). It never mutates history: it either appends one new
// iteration carrying an audit record, or reports that nothing changed.
// The same correlation key with the same analysis is idempotent.
func (a *Application) ReEvaluate(ctx context.Context, in ReEvaluateInput) (*ReEvaluationOutcome, error) {
	if strings.TrimSpace(in.CorrelationKey) == "" || in.PreviousIterationID == "" || in.AnalysisID == "" {
		return nil, fmt.Errorf("%w: correlationKey, previousIterationId and analysisId are required", ErrInvalidInput)
	}
	if !in.Trigger.Kind.Valid() {
		return nil, fmt.Errorf("%w: trigger.kind must be MANUAL or SCHEDULED", ErrInvalidInput)
	}
	run, err := a.repos.Research.GetResearchRun(ctx, in.RunID)
	if err != nil {
		return nil, err
	}
	for _, it := range run.Iterations {
		if it.ReEvaluation == nil || it.ReEvaluation.CorrelationKey != in.CorrelationKey {
			continue
		}
		if it.AnalysisID != in.AnalysisID {
			return nil, ErrReEvaluationConflict
		}
		return &ReEvaluationOutcome{Status: ReEvaluationAlreadyEvaluated, Record: *it.ReEvaluation, Run: run}, nil
	}
	latest, ok := run.LatestIteration()
	if !ok || latest.ID != in.PreviousIterationID {
		return nil, ErrStaleIteration
	}
	before, err := a.optionalAnalysis(ctx, latest.AnalysisID)
	if err != nil {
		return nil, err
	}
	after, err := a.ResolveAnalysis(ctx, run.ProjectID, in.AnalysisID)
	if err != nil {
		return nil, err
	}
	record := domain.ReEvaluation{
		CorrelationKey: in.CorrelationKey, PreviousIterationID: latest.ID, Trigger: in.Trigger,
		EvidenceChanges: normalizedChanges(in.EvidenceChanges), AffectedGapIDs: sortedUnique(in.AffectedGapIDs),
		AffectedHypothesisIDs: []string{}, Scope: domain.ReEvaluationFull, ScopeReason: fullScopeReason,
		InputFingerprintAfter: after.InputFingerprint, ExecutionFingerprintAfter: after.ExecutionFingerprint,
		Note: strings.TrimSpace(in.Note), RecordedAt: a.now(),
	}
	if before != nil {
		record.InputFingerprintBefore, record.ExecutionFingerprintBefore = before.InputFingerprint, before.ExecutionFingerprint
		// A known, unchanged input fingerprint proves the evidence did not
		// change, whatever the caller reports; no iteration is created.
		if before.InputFingerprint != "" && before.InputFingerprint == after.InputFingerprint {
			return &ReEvaluationOutcome{Status: ReEvaluationNoEvidenceChange, Record: record, Run: run}, nil
		}
	}
	added := append(append([]string{}, record.EvidenceChanges.Added...), record.EvidenceChanges.Changed...)
	var links []domain.AddedEvidenceLink
	for _, ref := range added {
		links = append(links, domain.AddedEvidenceLink{Reference: ref, GapIDs: record.AffectedGapIDs, Note: record.Note})
	}
	iteration, err := a.buildAppendedIteration(ctx, run, AppendResearchIterationInput{RunID: run.ID, AnalysisID: after.ID, AddedEvidenceLinks: links})
	if err != nil {
		return nil, err
	}
	record.AffectedHypothesisIDs = affectedHypotheses(latest, iteration, record.AffectedGapIDs)
	if err := a.recordAffectedScenarios(ctx, run.ID, &record); err != nil {
		return nil, err
	}
	iteration.ReEvaluation = &record
	if err := a.repos.Research.AppendResearchIteration(ctx, run.ID, iteration); err != nil {
		return nil, fmt.Errorf("append re-evaluation iteration: %w", err)
	}
	updated, err := a.repos.Research.GetResearchRun(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	return &ReEvaluationOutcome{Status: ReEvaluationNewIteration, Record: record, Run: updated}, nil
}

func (a *Application) optionalAnalysis(ctx context.Context, id string) (*domain.Analysis, error) {
	if id == "" {
		return nil, nil
	}
	analysis, err := a.repos.Analyses.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return analysis, err
}

// affectedHypotheses lists hypotheses named by the affected gaps of the
// previous iteration plus those whose validation changed in the new one.
func affectedHypotheses(previous, current domain.ResearchIteration, gapIDs []string) []string {
	var ids []string
	for _, g := range previous.ResearchGaps {
		if slices.Contains(gapIDs, g.ID) {
			ids = append(ids, g.AffectedHypothesisIDs...)
		}
	}
	for _, c := range current.HypothesisChanges {
		if c.Evolution != domain.HypothesisUnchanged {
			ids = append(ids, c.HypothesisID)
		}
	}
	return sortedUnique(ids)
}

func normalizedChanges(c domain.EvidenceChanges) domain.EvidenceChanges {
	return domain.EvidenceChanges{Added: sortedUnique(c.Added), Removed: sortedUnique(c.Removed), Changed: sortedUnique(c.Changed)}
}

func sortedUnique(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range in {
		if v = strings.TrimSpace(v); v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

// recordAffectedScenarios fills the scenario refs of record from the run's
// latest scenario set. A run without scenarios (or without scenario storage)
// records none.
func (a *Application) recordAffectedScenarios(ctx context.Context, runID string, record *domain.ReEvaluation) error {
	if a.repos.Scenarios == nil {
		return nil
	}
	sets, err := a.repos.Scenarios.ListScenarioSets(ctx, runID)
	if err != nil {
		return fmt.Errorf("list scenario sets: %w", err)
	}
	var latest *domain.ScenarioSet
	for _, s := range sets {
		if latest == nil || s.Version > latest.Version {
			latest = s
		}
	}
	if latest == nil {
		return nil
	}
	evidence := append(append([]string{}, record.EvidenceChanges.Removed...), record.EvidenceChanges.Changed...)
	touches := func(sc domain.Scenario) bool {
		if sc.DerivedFromHypothesisID != "" && slices.Contains(record.AffectedHypothesisIDs, sc.DerivedFromHypothesisID) {
			return true
		}
		for _, g := range sc.UnresolvedGapIDs {
			if slices.Contains(record.AffectedGapIDs, g) {
				return true
			}
		}
		refs := append([]string{}, sc.EvidenceRefs...)
		for _, as := range sc.Assumptions {
			refs = append(refs, as.EvidenceRefs...)
		}
		for _, r := range refs {
			if slices.Contains(evidence, r) {
				return true
			}
		}
		return false
	}
	var ids []string
	for _, sc := range latest.Scenarios {
		if touches(sc) {
			ids = append(ids, sc.ID)
		}
	}
	if len(ids) > 0 {
		record.AffectedScenarioSetID, record.AffectedScenarioIDs = latest.ID, sortedUnique(ids)
	}
	return nil
}
