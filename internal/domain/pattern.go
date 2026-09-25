package domain

import "time"

// PatternKind distinguishes repeated regularities from expectation mismatches.
//
// PatternRepetition represents a regularity supported by multiple grounded
// observations. PatternDeviation represents an observation that differs from
// a baseline, expectation, comparison or expected sequence. The vocabulary
// originated in customer research but the semantics are now domain-neutral.
type PatternKind string

const (
	PatternRepetition PatternKind = "repetition"
	PatternDeviation  PatternKind = "deviation"
)

func (k PatternKind) Valid() bool {
	return k == PatternRepetition || k == PatternDeviation
}

// DeviationType classifies how an observation diverged from an expectation.
// Some legacy values are behavior-oriented; generic research uses OTHER when
// a mismatch does not fit them.
type DeviationType string

const (
	DeviationContradiction DeviationType = "contradiction"  // 言っていることとやっていることが違う
	DeviationExcessEffort  DeviationType = "excess_effort"  // 急いでいる・面倒と言いながら手間をかける
	DeviationExcessPayment DeviationType = "excess_payment" // 予定より多く払う・高い方を選ぶ
	DeviationPersistence   DeviationType = "persistence"    // 不満を持ちながら使い続ける・やめない
	DeviationAbsence       DeviationType = "absence"        // 起きるはずの行動が起きていない
	DeviationOther         DeviationType = "other"
)

func (d DeviationType) Valid() bool {
	switch d {
	case DeviationContradiction, DeviationExcessEffort, DeviationExcessPayment,
		DeviationPersistence, DeviationAbsence, DeviationOther:
		return true
	}
	return false
}

// Pattern is an inspectable finding between grounded Observation and a
// hypothesis. It makes the reasoning trail visible instead of hiding the
// model's intermediate noticing step.
//
// For Kind == PatternDeviation, Expectation is the baseline/prediction and
// Description is the observed fact that differs from it.
type Pattern struct {
	ID             string
	ProjectID      string
	AnalysisID     string
	Kind           PatternKind
	Title          string
	Description    string
	Expectation    string        // deviation only: what common sense predicted
	DeviationType  DeviationType // deviation only
	ObservationIDs []string      // populated on read via the pattern_observations join
	CreatedAt      time.Time
}

// IsTrace reports whether this pattern is a deviation-from-expectation rather
// than a repetition. The method name is retained for backward compatibility.
func (p *Pattern) IsTrace() bool {
	return p.Kind == PatternDeviation
}
