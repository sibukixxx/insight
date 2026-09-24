package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Scenario analysis (#66) is prospective reasoning, not prediction. A
// ScenarioSet branches one observed baseline into several possible futures,
// each with explicit assumptions and frozen, falsifiable expectations about
// what later observations would show. Scenario status records how later
// evidence has treated a branch so far; it is never a probability and the
// engine never picks a "most likely future".

// ScenarioStatus is the evidence state of one branch. It is not likelihood.
type ScenarioStatus string

const (
	ScenarioUntested        ScenarioStatus = "UNTESTED"
	ScenarioConsistentSoFar ScenarioStatus = "CONSISTENT_SO_FAR"
	ScenarioWeakened        ScenarioStatus = "WEAKENED"
	ScenarioContradicted    ScenarioStatus = "CONTRADICTED"
	ScenarioInconclusive    ScenarioStatus = "INCONCLUSIVE"
)

// AssumptionKind keeps observed facts apart from assumptions. Only
// OBSERVED_BASELINE may cite observed evidence as fact.
type AssumptionKind string

const (
	AssumptionObservedBaseline AssumptionKind = "OBSERVED_BASELINE"
	AssumptionExogenous        AssumptionKind = "EXOGENOUS_ASSUMPTION"
	AssumptionModel            AssumptionKind = "MODEL_ASSUMPTION"
	AssumptionHuman            AssumptionKind = "HUMAN_ASSUMPTION"
	AssumptionPolicy           AssumptionKind = "POLICY_ASSUMPTION"
)

func (k AssumptionKind) Valid() bool {
	switch k {
	case AssumptionObservedBaseline, AssumptionExogenous, AssumptionModel, AssumptionHuman, AssumptionPolicy:
		return true
	}
	return false
}

// ScenarioOrigin records who drafted a scenario set.
type ScenarioOrigin string

const (
	ScenarioOriginScaffold ScenarioOrigin = "DETERMINISTIC_SCAFFOLD"
	ScenarioOriginHuman    ScenarioOrigin = "HUMAN"
	ScenarioOriginModel    ScenarioOrigin = "MODEL"
	ScenarioOriginImported ScenarioOrigin = "IMPORTED"
)

// ScenarioRelationKind states how two branches relate; scenarios need not be
// mutually exclusive, but the relation must be explicit when declared.
type ScenarioRelationKind string

const (
	RelationMutuallyExclusive ScenarioRelationKind = "MUTUALLY_EXCLUSIVE"
	RelationOverlapping       ScenarioRelationKind = "OVERLAPPING"
	RelationNested            ScenarioRelationKind = "NESTED"
	RelationConflicting       ScenarioRelationKind = "CONFLICTING"
)

type ScenarioBaseline struct {
	AsOf         time.Time `json:"asOf"`
	Description  string    `json:"description"`
	EvidenceRefs []string  `json:"evidenceRefs,omitempty"`
}

type ScenarioHorizon struct {
	Label string    `json:"label"`
	End   time.Time `json:"end"`
}

type TimeWindow struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type Assumption struct {
	ID           string         `json:"id"`
	Statement    string         `json:"statement"`
	Kind         AssumptionKind `json:"kind"`
	SourceRefs   []string       `json:"sourceRefs,omitempty"`
	EvidenceRefs []string       `json:"evidenceRefs,omitempty"`
	AsOf         time.Time      `json:"asOf"`
	ValidUntil   *time.Time     `json:"validUntil,omitempty"`
}

// ScenarioExpectation is a frozen, falsifiable statement about a future
// observation. It reuses Expectation provenance: post-hoc text is never
// treated as prior.
type ScenarioExpectation struct {
	ID                              string           `json:"id"`
	Indicator                       string           `json:"indicator"`
	Statement                       string           `json:"statement"`
	Direction                       string           `json:"direction,omitempty"` // INCREASE | DECREASE | NO_CHANGE | RANGE
	Range                           *ValueRange      `json:"range,omitempty"`
	ObservationWindow               TimeWindow       `json:"observationWindow"`
	FalsificationCondition          string           `json:"falsificationCondition"`
	Provenance                      ExpectationBasis `json:"provenance"`
	ObservedDataAvailableAtCreation bool             `json:"observedDataAvailableAtCreation"`
	SourceRefs                      []string         `json:"sourceRefs,omitempty"`
}

type ValueRange struct {
	Min  *float64 `json:"min,omitempty"`
	Max  *float64 `json:"max,omitempty"`
	Unit string   `json:"unit,omitempty"`
}

// ScenarioProbability may only be carried when a caller supplies it with a
// basis and sources. The engine never generates one.
type ScenarioProbability struct {
	Value      float64  `json:"value"`
	Basis      string   `json:"basis"`
	SourceRefs []string `json:"sourceRefs"`
}

type Scenario struct {
	ID                        string                `json:"id"`
	Title                     string                `json:"title"`
	Horizon                   ScenarioHorizon       `json:"horizon"`
	Assumptions               []Assumption          `json:"assumptions"`
	Mechanisms                []string              `json:"mechanisms,omitempty"`
	Expectations              []ScenarioExpectation `json:"expectations"`
	DisconfirmingObservations []string              `json:"disconfirmingObservations,omitempty"`
	AffectedDimensions        []string              `json:"affectedDimensions,omitempty"`
	EvidenceRefs              []string              `json:"evidenceRefs,omitempty"`
	UnresolvedGapIDs          []string              `json:"unresolvedGapIds,omitempty"`
	DerivedFromHypothesisID   string                `json:"derivedFromHypothesisId,omitempty"`
	Limitations               []string              `json:"limitations,omitempty"`
	Probability               *ScenarioProbability  `json:"probability,omitempty"`
}

type ScenarioRelation struct {
	ScenarioIDs []string             `json:"scenarioIds"`
	Kind        ScenarioRelationKind `json:"kind"`
	Note        string               `json:"note,omitempty"`
}

// ScenarioSet is append-only: curating scenarios creates a new Version and
// earlier versions stay readable.
type ScenarioSet struct {
	ID                 string             `json:"id"`
	ResearchRunID      string             `json:"researchRunId"`
	Version            int                `json:"version"`
	Question           string             `json:"question"`
	Baseline           ScenarioBaseline   `json:"baseline"`
	NonExhaustive      bool               `json:"nonExhaustive"`
	SharedEvidenceRefs []string           `json:"sharedEvidenceRefs,omitempty"`
	Scenarios          []Scenario         `json:"scenarios"`
	Relations          []ScenarioRelation `json:"relations,omitempty"`
	Origin             ScenarioOrigin     `json:"origin"`
	FromIterationID    string             `json:"fromIterationId,omitempty"`
	CreatedAt          time.Time          `json:"createdAt"`
}

var ErrScenarioInvalid = errors.New("scenario: invalid")

func scenarioErr(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrScenarioInvalid, fmt.Sprintf(format, args...))
}

func blankStr(s string) bool { return strings.TrimSpace(s) == "" }
