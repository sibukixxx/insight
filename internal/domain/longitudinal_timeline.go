package domain

import (
	"encoding/json"
	"time"
)

// LongitudinalTimeline is a read model of one ResearchRun over time (#71).
// It is derived from immutable iteration payloads and run snapshots; building
// it never mutates history. World/evidence changes and instrument changes are
// kept in separate lanes so one is never mistaken for the other.
type LongitudinalTimeline struct {
	ResearchRunID     string                     `json:"researchRunId"`
	Question          string                     `json:"question"`
	AsOf              *time.Time                 `json:"asOf,omitempty"`
	Iterations        []TimelineIteration        `json:"iterations"`
	EvidenceEvents    []EvidenceEvent            `json:"evidenceEvents"`
	ObservationDeltas []TimelineObservationDelta `json:"observationDeltas"`
	HypothesisEvents  []HypothesisEvent          `json:"hypothesisEvents"`
	InsightVersions   []InsightVersion           `json:"insightVersions"`
	InstrumentChanges []InstrumentChange         `json:"instrumentChanges"`
	Limitations       []string                   `json:"limitations"`
}

// TimelineIteration is the frozen interpretation recorded at one iteration.
type TimelineIteration struct {
	IterationID          string             `json:"iterationId"`
	Sequence             int                `json:"sequence"`
	PreviousIterationID  string             `json:"previousIterationId,omitempty"`
	AnalysisID           string             `json:"analysisId,omitempty"`
	ObservationWindow    *ObservationWindow `json:"observationWindow,omitempty"`
	Question             string             `json:"question"`
	Readiness            DecisionReadiness  `json:"readiness"`
	Stopped              bool               `json:"stopped"`
	UnresolvedGapIDs     []string           `json:"unresolvedGapIds,omitempty"`
	WhatWeCannotConclude []string           `json:"whatWeCannotConclude,omitempty"`
	// CarriedUncertainty lists cannot-conclude statements that survived from
	// the previous iteration; ResolvedUncertainty lists those that did not.
	CarriedUncertainty  []string  `json:"carriedUncertainty,omitempty"`
	ResolvedUncertainty []string  `json:"resolvedUncertainty,omitempty"`
	RecordedAt          time.Time `json:"recordedAt"`
}

// EvidenceEvent is one world/evidence-lane change between iterations.
type EvidenceEvent struct {
	IterationID string             `json:"iterationId"`
	Sequence    int                `json:"sequence"`
	Kind        EvidenceChangeKind `json:"kind"`
	Reference   string             `json:"reference"`
	GapIDs      []string           `json:"gapIds,omitempty"`
	Detail      string             `json:"detail,omitempty"`
}

// TimelineObservationDelta binds a deterministic observation delta (#70) to
// the pair of iterations whose analyses produced the two observations.
type TimelineObservationDelta struct {
	FromIterationID string          `json:"fromIterationId"`
	ToIterationID   string          `json:"toIterationId"`
	SeriesKey       string          `json:"seriesKey"`
	Delta           json.RawMessage `json:"delta"`
}

// HypothesisEvent is one hypothesis lifecycle step, tied to the concrete
// evidence delta of its iteration.
type HypothesisEvent struct {
	IterationID  string              `json:"iterationId"`
	Sequence     int                 `json:"sequence"`
	HypothesisID string              `json:"hypothesisId"`
	Evolution    HypothesisEvolution `json:"evolution"`
	Reason       string              `json:"reason"`
	EvidenceRefs []string            `json:"evidenceRefs,omitempty"`
	Attribution  ChangeAttribution   `json:"attribution"`
	// InvalidatesPriorInterpretation is true for WEAKENED/CONTRADICTED: the
	// earlier interpretation stays frozen but is no longer current.
	InvalidatesPriorInterpretation bool `json:"invalidatesPriorInterpretation"`
}

// InsightVersion is the insight state of one iteration and why it changed.
type InsightVersion struct {
	IterationID string            `json:"iterationId"`
	Sequence    int               `json:"sequence"`
	InsightIDs  []string          `json:"insightIds,omitempty"`
	Explanation []string          `json:"explanation,omitempty"`
	Attribution ChangeAttribution `json:"attribution"`
}

// InstrumentChange is an instrument-lane marker: the engine, model, prompt or
// rules of the analysis behind an iteration differ from the previous one, or
// could not be compared (#82/#83 semantics). Input changes are world/evidence
// events and never appear here.
type InstrumentChange struct {
	FromIterationID string           `json:"fromIterationId"`
	ToIterationID   string           `json:"toIterationId"`
	Execution       FingerprintState `json:"execution"`
	ChangedFields   []string         `json:"changedFields,omitempty"`
}
