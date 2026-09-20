package domain

import (
	"fmt"
	"time"
)

// ResearchGapCategory classifies evidence that is still required before a
// research question can be answered responsibly.
type ResearchGapCategory string

const (
	ResearchGapConfounder      ResearchGapCategory = "CONFOUNDER"
	ResearchGapComparison      ResearchGapCategory = "COMPARISON_CONTROL"
	ResearchGapPrePeriod       ResearchGapCategory = "PRE_PERIOD"
	ResearchGapTiming          ResearchGapCategory = "TIMING"
	ResearchGapMeasurement     ResearchGapCategory = "MEASUREMENT"
	ResearchGapExternalContext ResearchGapCategory = "EXTERNAL_CONTEXT"
	ResearchGapSourceQuality   ResearchGapCategory = "SOURCE_QUALITY"
	ResearchGapOther           ResearchGapCategory = "OTHER"
)

// ResearchGap is evidence still required before a research question can be
// answered responsibly. Gaps carry an identity across iterations so that a
// stop decision can still point at what was never resolved.
type ResearchGap struct {
	ID                    string              `json:"id"`
	Category              ResearchGapCategory `json:"category"`
	Need                  string              `json:"need"`
	WhyItMatters          string              `json:"whyItMatters"`
	AffectedHypothesisIDs []string            `json:"affectedHypothesisIds,omitempty"`
	DependsOnGapIDs       []string            `json:"dependsOnGapIds,omitempty"`
	Resolvable            bool                `json:"resolvable"`
	Resolved              bool                `json:"resolved"`
	// FirstSeenIterationID and AddressedInIterationID link the gap to research
	// history. AddressedInIterationID is only set when a later iteration carried
	// added evidence and the gap no longer appears in the re-analysis.
	FirstSeenIterationID   string `json:"firstSeenIterationId,omitempty"`
	AddressedInIterationID string `json:"addressedInIterationId,omitempty"`
}

// DiscriminatingPower is a qualitative statement about how much a piece of
// evidence would separate competing hypotheses. It is not a probability.
type DiscriminatingPower string

const (
	DiscriminatingNone     DiscriminatingPower = "NONE"
	DiscriminatingLow      DiscriminatingPower = "LOW"
	DiscriminatingModerate DiscriminatingPower = "MODERATE"
	DiscriminatingHigh     DiscriminatingPower = "HIGH"
)

type GapUrgency string

const (
	UrgencyLow    GapUrgency = "LOW"
	UrgencyMedium GapUrgency = "MEDIUM"
	UrgencyHigh   GapUrgency = "HIGH"
)

// AcquisitionDifficulty is qualitative. Insight never acquires evidence, so
// this only informs the human who plans the acquisition.
type AcquisitionDifficulty string

const (
	AcquisitionUnknown    AcquisitionDifficulty = "UNKNOWN"
	AcquisitionLow        AcquisitionDifficulty = "LOW"
	AcquisitionModerate   AcquisitionDifficulty = "MODERATE"
	AcquisitionHigh       AcquisitionDifficulty = "HIGH"
	AcquisitionInfeasible AcquisitionDifficulty = "INFEASIBLE"
)

// RequirementPriority explains why one DataRequirement should be pursued
// before another. Rank is an ordering within one iteration, not a score.
type RequirementPriority struct {
	Rank                     int                   `json:"rank"`
	DiscriminatingPower      DiscriminatingPower   `json:"discriminatingPower"`
	CanDistinguishHypotheses bool                  `json:"canDistinguishHypotheses"`
	CanFalsify               bool                  `json:"canFalsify"`
	Urgency                  GapUrgency            `json:"urgency"`
	AcquisitionDifficulty    AcquisitionDifficulty `json:"acquisitionDifficulty"`
	Rationale                []string              `json:"rationale,omitempty"`
}

// DataRequirement is provider-neutral. It describes evidence to collect but
// never claims that a particular source exists or that collection occurred.
type DataRequirement struct {
	GapID                   string   `json:"gapId"`
	Need                    string   `json:"need"`
	Reason                  string   `json:"reason"`
	AffectedHypothesisIDs   []string `json:"affectedHypothesisIds,omitempty"`
	RequiredDimensions      []string `json:"requiredDimensions,omitempty"`
	RequiredPeriod          string   `json:"requiredPeriod,omitempty"`
	RequiredPopulation      string   `json:"requiredPopulation,omitempty"`
	SuggestedSourceCategory string   `json:"suggestedSourceCategory,omitempty"`
	DependsOnGapIDs         []string `json:"dependsOnGapIds,omitempty"`
	// Priority is empty until the requirement has been prioritized within an
	// iteration. A zero Rank means "not yet prioritized".
	Priority RequirementPriority `json:"priority"`
}

type HypothesisEvolution string

const (
	HypothesisCreated      HypothesisEvolution = "CREATED"
	HypothesisUnchanged    HypothesisEvolution = "UNCHANGED"
	HypothesisStrengthened HypothesisEvolution = "STRENGTHENED"
	HypothesisWeakened     HypothesisEvolution = "WEAKENED"
	HypothesisContradicted HypothesisEvolution = "CONTRADICTED"
)

type HypothesisChange struct {
	HypothesisID string              `json:"hypothesisId"`
	Evolution    HypothesisEvolution `json:"evolution"`
	Reason       string              `json:"reason"`
}

type HypothesisState struct {
	HypothesisID         string               `json:"hypothesisId"`
	ComparisonKey        string               `json:"comparisonKey"`
	ValidationStatus     ValidationStatus     `json:"validationStatus"`
	IdentificationStatus IdentificationStatus `json:"identificationStatus"`
}

// ValidationEvidenceProvenance records why an evidence source was treated as
// independent for a VALIDATION transition. This is audit provenance, not a
// probability or causal-certainty score.
type ValidationEvidenceProvenance struct {
	ExpectationID     string    `json:"expectationId"`
	EvidenceReference string    `json:"evidenceReference"`
	SourceIterationID string    `json:"sourceIterationId,omitempty"`
	DatasetReference  string    `json:"datasetReference,omitempty"`
	IndependenceRationale string `json:"independenceRationale"`
	Actor             string    `json:"actor,omitempty"`
	RecordedAt        time.Time `json:"recordedAt"`
}

func (v ValidationEvidenceProvenance) Validate(currentIterationID string, expectations []Expectation) error {
	if strings.TrimSpace(v.ExpectationID) == "" || strings.TrimSpace(v.EvidenceReference) == "" || strings.TrimSpace(v.IndependenceRationale) == "" {
		return fmt.Errorf("validation evidence provenance requires expectationId, evidenceReference and independenceRationale")
	}
	if v.SourceIterationID != "" && v.SourceIterationID == currentIterationID {
		return fmt.Errorf("same-iteration evidence cannot be recorded as independent validation evidence")
	}
	foundFrozen := false
	for _, e := range expectations {
		if e.ID == v.ExpectationID && e.FrozenForValidation {
			foundFrozen = true
			break
		}
	}
	if !foundFrozen {
		return fmt.Errorf("validation evidence references non-frozen expectation %q", v.ExpectationID)
	}
	return nil
}

// DecisionReadiness states how far the research process has been prepared
// for a responsible human decision. It is not a truth probability, not a
// causal certainty, and not a commercial recommendation. Values are chosen so
// they never collide with ValidationStatus / IdentificationStatus / CausalStatus.
type DecisionReadiness string

const (
	ReadinessEvidenceInsufficient        DecisionReadiness = "EVIDENCE_INSUFFICIENT"
	ReadinessExploratoryOnly             DecisionReadiness = "EXPLORATORY_ONLY"
	ReadinessValidationRequired          DecisionReadiness = "VALIDATION_REQUIRED"
	ReadinessEvidenceConverging          DecisionReadiness = "EVIDENCE_CONVERGING"
	ReadinessDecisionReadyWithLimitation DecisionReadiness = "DECISION_READY_WITH_LIMITATIONS"
	ReadinessInconclusive                DecisionReadiness = "INCONCLUSIVE"
	ReadinessBlocked                     DecisionReadiness = "BLOCKED"
)

func AllDecisionReadiness() []DecisionReadiness {
	return []DecisionReadiness{
		ReadinessEvidenceInsufficient, ReadinessExploratoryOnly, ReadinessValidationRequired,
		ReadinessEvidenceConverging, ReadinessDecisionReadyWithLimitation, ReadinessInconclusive, ReadinessBlocked,
	}
}

func (d DecisionReadiness) Valid() bool {
	for _, known := range AllDecisionReadiness() {
		if d == known {
			return true
		}
	}
	return false
}

// ReadinessAssessment is the system-computed readiness with its reasons.
// A zero State means the iteration has not been assessed.
type ReadinessAssessment struct {
	State            DecisionReadiness `json:"state"`
	Reasons          []string          `json:"reasons,omitempty"`
	UnresolvedGapIDs []string          `json:"unresolvedGapIds,omitempty"`
	IterationCount   int               `json:"iterationCount"`
	AssessedAt       time.Time         `json:"assessedAt"`
}

type ResearchStopReason string

const (
	StopHypothesesDistinguished    ResearchStopReason = "CRITICAL_HYPOTHESES_DISTINGUISHED"
	StopCounterEvidenceResolved    ResearchStopReason = "MAJOR_COUNTER_EVIDENCE_RESOLVED"
	StopUncertaintyNonMaterial     ResearchStopReason = "REMAINING_UNCERTAINTY_NON_MATERIAL"
	StopNoFeasibleDataSource       ResearchStopReason = "NO_FEASIBLE_DATA_SOURCE"
	StopConflictingEvidenceRemains ResearchStopReason = "CONFLICTING_EVIDENCE_REMAINS"
	StopIdentificationUnresolved   ResearchStopReason = "IDENTIFICATION_UNRESOLVED"
	StopHumanChoice                ResearchStopReason = "HUMAN_STOPPED"
	StopExternalBudgetBoundary     ResearchStopReason = "EXTERNAL_BUDGET_BOUNDARY"
)

// HumanSuppliable reports whether a stop reason may be asserted from outside
// the research process. Whether remaining uncertainty is material to the
// stated question is a human judgement; evidence-based reasons are reserved
// for the system.
func (r ResearchStopReason) HumanSuppliable() bool {
	return r == StopHumanChoice || r == StopExternalBudgetBoundary || r == StopUncertaintyNonMaterial
}

type StopSource string

const (
	StopSourceSystem StopSource = "SYSTEM"
	StopSourceHuman  StopSource = "HUMAN"
)

// StopDecision records why the loop stopped. Unresolved gaps are preserved so
// that stopping never erases what remains unknown.
type StopDecision struct {
	Reason           ResearchStopReason `json:"reason"`
	Source           StopSource         `json:"source"`
	ReadinessAtStop  DecisionReadiness  `json:"readinessAtStop,omitempty"`
	Note             string             `json:"note,omitempty"`
	UnresolvedGapIDs []string           `json:"unresolvedGapIds,omitempty"`
	DecidedAt        time.Time          `json:"decidedAt"`
}

// HumanOverride is an explicit human action on an iteration: overriding the
// computed readiness, stopping the loop, or both. Overrides are appended,
// never replaced, so the history of judgement stays auditable.
type HumanOverride struct {
	Readiness  DecisionReadiness  `json:"readiness,omitempty"`
	StopReason ResearchStopReason `json:"stopReason,omitempty"`
	Note       string             `json:"note,omitempty"`
	RecordedAt time.Time          `json:"recordedAt"`
}

// ResearchIteration is append-only research history. Callers create a new
// value for re-analysis instead of mutating an earlier iteration.
type ResearchIteration struct {
	ID               string            `json:"id"`
	Sequence         int               `json:"sequence"`
	Stage            ResearchStage     `json:"stage,omitempty"`
	Question         string            `json:"question"`
	InputReferences  []string          `json:"inputReferences,omitempty"`
	ObservationIDs   []string          `json:"observationIds,omitempty"`
	SurpriseIDs      []string          `json:"surpriseIds,omitempty"`
	HypothesisSetIDs []string          `json:"hypothesisSetIds,omitempty"`
	InsightIDs       []string          `json:"insightIds,omitempty"`
	HypothesisStates []HypothesisState `json:"hypothesisStates,omitempty"`
	// Expectations are the first-class validation targets carried on this
	// iteration: those built from this iteration's own insights plus any
	// frozen expectation derived from a prior iteration. They are never
	// mutated in place; freezing or deriving always produces a new entry.
	Expectations         []Expectation       `json:"expectations,omitempty"`
	ResearchGaps         []ResearchGap       `json:"researchGaps,omitempty"`
	DataRequirements     []DataRequirement   `json:"dataRequirements,omitempty"`
	AddedEvidence        []string            `json:"addedEvidence,omitempty"`
	HypothesisChanges    []HypothesisChange  `json:"hypothesisChanges,omitempty"`
	ValidationEvidence   []ValidationEvidenceProvenance `json:"validationEvidence,omitempty"`
	WhatWeCannotConclude []string            `json:"whatWeCannotConclude,omitempty"`
	Readiness            ReadinessAssessment `json:"readiness"`
	Stop                 *StopDecision       `json:"stop,omitempty"`
	HumanOverrides       []HumanOverride     `json:"humanOverrides,omitempty"`
	// PromotionGateInput is the evidence a promotion transition was last
	// checked against (issue #24). It is kept alongside Promotion so a later
	// explicit transition (e.g. to PUBLISHED) can be re-checked without the
	// caller having to resupply every fact.
	PromotionGateInput PromotionGateInput  `json:"promotionGateInput,omitempty"`
	Promotion          PromotionAssessment `json:"promotion"`
	CreatedAt          time.Time           `json:"createdAt"`
}

// UnresolvedGapIDs lists gaps that are still open in this iteration.
func (it ResearchIteration) UnresolvedGapIDs() []string {
	var ids []string
	for _, gap := range it.ResearchGaps {
		if !gap.Resolved {
			ids = append(ids, gap.ID)
		}
	}
	return ids
}

// EffectiveReadiness returns the latest human readiness override when one
// exists, otherwise the system assessment. The computed assessment is never
// overwritten.
func (it ResearchIteration) EffectiveReadiness() DecisionReadiness {
	for i := len(it.HumanOverrides) - 1; i >= 0; i-- {
		if it.HumanOverrides[i].Readiness != "" {
			return it.HumanOverrides[i].Readiness
		}
	}
	return it.Readiness.State
}

// ApplyHumanOverride returns a copy with the override appended. A stop
// requested by a human keeps every gap and records what was unresolved.
func (it ResearchIteration) ApplyHumanOverride(override HumanOverride) (ResearchIteration, error) {
	if override.Readiness == "" && override.StopReason == "" {
		return ResearchIteration{}, fmt.Errorf("human override must set readiness or a stop reason")
	}
	if override.Readiness != "" && !override.Readiness.Valid() {
		return ResearchIteration{}, fmt.Errorf("invalid readiness override %q", override.Readiness)
	}
	if override.StopReason != "" && !override.StopReason.HumanSuppliable() {
		return ResearchIteration{}, fmt.Errorf("stop reason %q is decided by the research process, not supplied by a human", override.StopReason)
	}
	out := it
	out.HumanOverrides = append(append([]HumanOverride(nil), it.HumanOverrides...), override)
	if override.StopReason != "" {
		out.Stop = &StopDecision{
			Reason: override.StopReason, Source: StopSourceHuman, ReadinessAtStop: out.EffectiveReadiness(),
			Note: override.Note, UnresolvedGapIDs: it.UnresolvedGapIDs(), DecidedAt: override.RecordedAt,
		}
	}
	return out, nil
}

type ResearchRun struct {
	ID         string              `json:"id"`
	ProjectID  string              `json:"projectId"`
	Question   string              `json:"question"`
	Iterations []ResearchIteration `json:"iterations"`
	CreatedAt  time.Time           `json:"createdAt"`
}

// LatestIteration returns the most recent iteration, if any.
func (r ResearchRun) LatestIteration() (ResearchIteration, bool) {
	if len(r.Iterations) == 0 {
		return ResearchIteration{}, false
	}
	return r.Iterations[len(r.Iterations)-1], true
}

// CurrentStage is the research stage of the latest iteration, or "" for a
// run with no iterations yet. It is never inferred from anything other than
// the latest iteration's own recorded stage.
func (r ResearchRun) CurrentStage() ResearchStage {
	latest, ok := r.LatestIteration()
	if !ok {
		return ""
	}
	return latest.Stage
}

// CurrentPromotionState is the promotion state of the latest iteration, or
// DRAFT for a run with no iterations yet or whose latest iteration has never
// been assessed. It is never inferred from anything other than the latest
// iteration's own recorded promotion state.
func (r ResearchRun) CurrentPromotionState() PromotionState {
	latest, ok := r.LatestIteration()
	if !ok || latest.Promotion.State == "" {
		return PromotionDraft
	}
	return latest.Promotion.State
}

// Stopped reports whether the latest iteration carries a stop decision. A new
// iteration after a stop means a human chose to continue.
func (r ResearchRun) Stopped() bool {
	latest, ok := r.LatestIteration()
	return ok && latest.Stop != nil
}

// AppendIteration returns a copy so prior run values remain untouched.
func (r ResearchRun) AppendIteration(iteration ResearchIteration) ResearchRun {
	out := r
	out.Iterations = append(append([]ResearchIteration(nil), r.Iterations...), iteration)
	return out
}

// HumanHandoff is the final shape handed to a human. It must never reduce to
// "A/B/C are possible, you decide": it states what survived, what was
// contradicted, what remains unknown, why the loop stopped, and what to
// research next if the human continues.
type HumanHandoff struct {
	ResearchRunID                 string             `json:"researchRunId"`
	Question                      string             `json:"question"`
	IterationCount                int                `json:"iterationCount"`
	Readiness                     DecisionReadiness  `json:"readiness"`
	ReadinessReasons              []string           `json:"readinessReasons,omitempty"`
	StrongestSurvivingHypotheses  []HypothesisState  `json:"strongestSurvivingHypotheses,omitempty"`
	ContradictedHypotheses        []HypothesisState  `json:"contradictedHypotheses,omitempty"`
	WeakenedHypotheses            []HypothesisChange `json:"weakenedHypotheses,omitempty"`
	UnresolvedAlternatives        []HypothesisState  `json:"unresolvedAlternatives,omitempty"`
	RemainingUncertainty          []ResearchGap      `json:"remainingUncertainty,omitempty"`
	WhatWeCannotConclude          []string           `json:"whatWeCannotConclude,omitempty"`
	EvidenceAddedAcrossIterations []string           `json:"evidenceAddedAcrossIterations,omitempty"`
	Stop                          *StopDecision      `json:"stop,omitempty"`
	SuggestedNextResearch         []DataRequirement  `json:"suggestedNextResearch,omitempty"`
	HumanOverrides                []HumanOverride    `json:"humanOverrides,omitempty"`
}

type HumanNovelty string

const (
	NoveltyKnownAlready HumanNovelty = "KNOWN_ALREADY"
	NoveltyPartiallyNew HumanNovelty = "PARTIALLY_NEW"
	NoveltyNew          HumanNovelty = "NEW"
	NoveltySurprising   HumanNovelty = "SURPRISING"
	NoveltyNotUseful    HumanNovelty = "NOT_USEFUL"
)

// HumanEvaluation is deliberately supplied by a human-facing boundary. LLM
// output must never be treated as authoritative novelty assessment.
type HumanEvaluation struct {
	ResearchRunID          string       `json:"researchRunId"`
	IterationID            string       `json:"iterationId"`
	ObservationGrounding   int          `json:"observationGrounding"`
	SurpriseUsefulness     int          `json:"surpriseUsefulness"`
	HypothesisDiversity    int          `json:"hypothesisDiversity"`
	CounterEvidenceQuality int          `json:"counterEvidenceQuality"`
	MissingEvidenceQuality int          `json:"missingEvidenceQuality"`
	IdentificationHonesty  int          `json:"identificationHonesty"`
	NextDataUsefulness     int          `json:"nextDataUsefulness"`
	Novelty                HumanNovelty `json:"novelty"`
	OverallUsefulness      int          `json:"overallUsefulness"`
	Notes                  string       `json:"notes,omitempty"`
	EvaluatedAt            time.Time    `json:"evaluatedAt"`
}

func (n HumanNovelty) Valid() bool {
	switch n {
	case NoveltyKnownAlready, NoveltyPartiallyNew, NoveltyNew, NoveltySurprising, NoveltyNotUseful:
		return true
	}
	return false
}
