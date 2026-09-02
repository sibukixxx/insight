package domain

import "time"

// PatternKind distinguishes the two ways a marketer "notices something"
// while reading many voices at once.
//
//   - PatternRepetition: the same behavior, complaint or workaround shows up
//     across multiple people. This is what the original Pattern Detection
//     step found.
//   - PatternDeviation: a single behavior that contradicts what common sense
//     would predict - paying more than planned, taking time despite being in
//     a hurry, keeping a product while complaining about it, or an expected
//     behavior that conspicuously did not happen ("the dog that didn't
//     bark"). These are the "traces of desire" (欲望の痕跡) that a hidden,
//     unconscious need leaves behind; an insight built on one of these is
//     far less likely to be a restatement of a stated need.
type PatternKind string

const (
	PatternRepetition PatternKind = "repetition"
	PatternDeviation  PatternKind = "deviation"
)

func (k PatternKind) Valid() bool {
	return k == PatternRepetition || k == PatternDeviation
}

// DeviationType classifies how the observed behavior diverged from the
// expectation. It is a fixed vocabulary so the UI can label traces
// consistently and so the app can count them without parsing prose.
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

// Pattern is a "noticing" step a marketer does by hand when skimming
// interviews. It sits between Observation (a single grounded quote) and
// Insight (the final hypothesis, tested against evidence): visualizing
// this layer is what turns insight generation from a black box into an
// inspectable trail.
//
// For Kind == PatternDeviation, Expectation holds the common-sense
// prediction ("忙しいなら出来合いの総菜を選ぶはず") and Description holds what
// actually happened; the gap between the two is the trace.
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

// IsTrace reports whether this pattern is a deviation-from-expectation
// (a trace of an unconscious desire) rather than a mere repetition.
func (p *Pattern) IsTrace() bool {
	return p.Kind == PatternDeviation
}
