package domain

import "time"

// ResearchGapCategory classifies evidence that is still required before a
// research question can be answered responsibly.
type ResearchGapCategory string

const (
	ResearchGapConfounder          ResearchGapCategory = "CONFOUNDER"
	ResearchGapComparison          ResearchGapCategory = "COMPARISON_CONTROL"
	ResearchGapPrePeriod           ResearchGapCategory = "PRE_PERIOD"
	ResearchGapTiming              ResearchGapCategory = "TIMING"
	ResearchGapMeasurement         ResearchGapCategory = "MEASUREMENT"
	ResearchGapExternalContext     ResearchGapCategory = "EXTERNAL_CONTEXT"
	ResearchGapSourceQuality       ResearchGapCategory = "SOURCE_QUALITY"
	ResearchGapOther               ResearchGapCategory = "OTHER"
)

type ResearchGap struct {
	ID                    string              `json:"id"`
	Category              ResearchGapCategory `json:"category"`
	Need                  string              `json:"need"`
	WhyItMatters          string              `json:"whyItMatters"`
	AffectedHypothesisIDs []string            `json:"affectedHypothesisIds,omitempty"`
	Resolvable            bool                `json:"resolvable"`
	Resolved              bool                `json:"resolved"`
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
	SuggestedSourceCategory string   `json:"suggestedSourceCategory,omitempty"`
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

// ResearchIteration is append-only research history. Callers create a new
// value for re-analysis instead of mutating an earlier iteration.
type ResearchIteration struct {
	ID                    string             `json:"id"`
	Sequence              int                `json:"sequence"`
	Question              string             `json:"question"`
	InputReferences       []string           `json:"inputReferences,omitempty"`
	ObservationIDs        []string           `json:"observationIds,omitempty"`
	SurpriseIDs           []string           `json:"surpriseIds,omitempty"`
	HypothesisSetIDs      []string           `json:"hypothesisSetIds,omitempty"`
	ResearchGaps          []ResearchGap      `json:"researchGaps,omitempty"`
	DataRequirements      []DataRequirement  `json:"dataRequirements,omitempty"`
	AddedEvidence         []string           `json:"addedEvidence,omitempty"`
	HypothesisChanges     []HypothesisChange `json:"hypothesisChanges,omitempty"`
	WhatWeCannotConclude  []string           `json:"whatWeCannotConclude,omitempty"`
	CreatedAt             time.Time          `json:"createdAt"`
}

type ResearchRun struct {
	ID         string              `json:"id"`
	ProjectID  string              `json:"projectId"`
	Question   string              `json:"question"`
	Iterations []ResearchIteration `json:"iterations"`
	CreatedAt  time.Time           `json:"createdAt"`
}

// AppendIteration returns a copy so prior run values remain untouched.
func (r ResearchRun) AppendIteration(iteration ResearchIteration) ResearchRun {
	out := r
	out.Iterations = append(append([]ResearchIteration(nil), r.Iterations...), iteration)
	return out
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
