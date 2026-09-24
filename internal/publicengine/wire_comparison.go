package publicengine

import (
	"insight-lab/internal/domain"
	"insight-lab/internal/service"
)

// Run comparison (#83) wire types. The comparison value object is defined
// once in the service layer and exposed verbatim; drift_test.go keeps it in
// line with schema.json.
type (
	RunComparison     = service.RunComparison
	RunRef            = service.RunRef
	FieldChange       = service.FieldChange
	ExecutionAxisDiff = service.ExecutionAxisDiff
	InputAxisDiff     = service.InputAxisDiff
	MetricDelta       = service.MetricDelta
	InsightMatch      = service.InsightMatch
	InsightResultDiff = service.InsightResultDiff
	MetricRange       = service.MetricRange
	RepeatGroup       = service.RepeatGroup
)

// Re-evaluation (#74) audit types, shared with the domain.
type (
	ReEvaluationTrigger = domain.ReEvaluationTrigger
	EvidenceChanges     = domain.EvidenceChanges
	ReEvaluationRecord  = domain.ReEvaluation
)

type AnalysisList struct {
	ContractVersion string        `json:"contractVersion"`
	SubjectID       string        `json:"subjectId"`
	Analyses        []AnalysisRun `json:"analyses"`
}

type RunComparisonResult struct {
	ContractVersion string        `json:"contractVersion"`
	SubjectID       string        `json:"subjectId"`
	Comparison      RunComparison `json:"comparison"`
}

type ResearchRunSummary struct {
	ResearchRunID           string `json:"researchRunId"`
	Question                string `json:"question"`
	IterationCount          int    `json:"iterationCount"`
	LatestIterationID       string `json:"latestIterationId"`
	LatestIterationSequence int    `json:"latestIterationSequence"`
	CreatedAt               string `json:"createdAt"`
}

type ResearchRunList struct {
	ContractVersion string               `json:"contractVersion"`
	SubjectID       string               `json:"subjectId"`
	ResearchRuns    []ResearchRunSummary `json:"researchRuns"`
}

type ReEvaluationRequest struct {
	ContractVersion     string              `json:"contractVersion"`
	IdempotencyKey      string              `json:"idempotencyKey"`
	CorrelationKey      string              `json:"correlationKey"`
	PreviousIterationID string              `json:"previousIterationId"`
	AnalysisID          string              `json:"analysisId"`
	Trigger             ReEvaluationTrigger `json:"trigger"`
	EvidenceChanges     EvidenceChanges     `json:"evidenceChanges"`
	AffectedGapIDs      []string            `json:"affectedGapIds,omitempty"`
	Note                string              `json:"note,omitempty"`
}

type ReEvaluationResult struct {
	ContractVersion string             `json:"contractVersion"`
	SubjectID       string             `json:"subjectId"`
	ResearchRunID   string             `json:"researchRunId"`
	Status          string             `json:"status"`
	ReEvaluation    ReEvaluationRecord `json:"reEvaluation"`
	Result          ResearchResult     `json:"result"`
}
