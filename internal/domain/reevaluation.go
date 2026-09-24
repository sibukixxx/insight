package domain

import "time"

// ReEvaluationTriggerKind says who asked for a re-evaluation. The engine
// does not schedule anything itself; an external scheduler or a person calls
// the contract (issue #74).
type ReEvaluationTriggerKind string

const (
	ReEvaluationManual    ReEvaluationTriggerKind = "MANUAL"
	ReEvaluationScheduled ReEvaluationTriggerKind = "SCHEDULED"
)

func (k ReEvaluationTriggerKind) Valid() bool {
	return k == ReEvaluationManual || k == ReEvaluationScheduled
}

// ReEvaluationScope records how much was re-evaluated. Only FULL is produced
// today: partial re-evaluation is not yet proven safe, so the engine falls
// back to evaluating the whole iteration from the new analysis run.
type ReEvaluationScope string

const ReEvaluationFull ReEvaluationScope = "FULL"

type ReEvaluationTrigger struct {
	Kind   ReEvaluationTriggerKind `json:"kind"`
	Source string                  `json:"source,omitempty"`
}

// EvidenceChanges is what the caller reports as having arrived, disappeared
// or changed since the previous iteration.
type EvidenceChanges struct {
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
	Changed []string `json:"changed"`
}

// ReEvaluation is the audit record attached to the iteration a
// re-evaluation created. Earlier iterations are never modified.
type ReEvaluation struct {
	CorrelationKey        string              `json:"correlationKey"`
	PreviousIterationID   string              `json:"previousIterationId"`
	Trigger               ReEvaluationTrigger `json:"trigger"`
	EvidenceChanges       EvidenceChanges     `json:"evidenceChanges"`
	AffectedGapIDs        []string            `json:"affectedGapIds"`
	AffectedHypothesisIDs []string            `json:"affectedHypothesisIds"`
	// AffectedScenarioSetID / AffectedScenarioIDs name the scenarios of the
	// run's latest scenario set that the evidence change touches (#66/#74):
	// derived from an affected hypothesis, blocked on an affected gap, or
	// citing removed/changed evidence. They are refs only; scenario status is
	// updated by an explicit scenario evaluation, never implicitly.
	AffectedScenarioSetID      string            `json:"affectedScenarioSetId,omitempty"`
	AffectedScenarioIDs        []string          `json:"affectedScenarioIds,omitempty"`
	Scope                      ReEvaluationScope `json:"scope"`
	ScopeReason                string            `json:"scopeReason"`
	InputFingerprintBefore     string            `json:"inputFingerprintBefore,omitempty"`
	InputFingerprintAfter      string            `json:"inputFingerprintAfter,omitempty"`
	ExecutionFingerprintBefore string            `json:"executionFingerprintBefore,omitempty"`
	ExecutionFingerprintAfter  string            `json:"executionFingerprintAfter,omitempty"`
	Note                       string            `json:"note,omitempty"`
	RecordedAt                 time.Time         `json:"recordedAt"`
}
