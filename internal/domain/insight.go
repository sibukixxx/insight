package domain

import "time"

type EvidenceType string

const (
	EvidenceSupport EvidenceType = "support"
	EvidenceCounter EvidenceType = "counter"
	EvidenceNeutral EvidenceType = "neutral"
)

type Evidence struct {
	ID             string
	InsightID      string
	DocumentID     string
	ObservationID  *string
	Quote          string
	Type           EvidenceType
	RelevanceScore float64
	StartOffset    int
	EndOffset      int
}

// QualityFlagCode names an app-side check that an insight failed. These
// checks encode what makes a "poor-quality insight" (粗悪品): a latent
// need that merely restates the stated need, a latent need expressed
// as a generic abstraction ("承認欲求", "コスパ"), or a hypothesis that is
// not anchored to any deviation-from-expectation trace. They are
// warnings for the human reader, computed deterministically by the app
// (never self-reported by the model), and never cause an insight to be
// silently dropped.
type QualityFlagCode string

const (
	// QualityStatedNeedEcho: latentNeed is (nearly) the same text as
	// statedNeed. A need the customer already voices is, by definition,
	// not an unconscious one.
	QualityStatedNeedEcho QualityFlagCode = "stated_need_echo"
	// QualityGenericTerm: latentNeed leans on an abstract label that
	// explains everything and therefore nothing (承認欲求, 自分らしさ,
	// コスパ, 安心 ...). Detail carries the offending term.
	QualityGenericTerm QualityFlagCode = "generic_term"
	// QualityNoTrace: the hypothesis cites no deviation pattern - it was
	// built from repetition alone, so it may be a well-supported but
	// obvious observation rather than a hidden need.
	QualityNoTrace QualityFlagCode = "no_trace"
	// QualityAbductionIncomplete: the hypothesis is missing the
	// expectation or the surprising fact, so the abductive chain
	// (予想 → ズレ → 仮説) cannot be checked by a reader.
	QualityAbductionIncomplete QualityFlagCode = "abduction_incomplete"
	// QualitySecondhandOnly: every supporting quote comes from a
	// secondhand document (a salesperson's notes, a summary someone
	// wrote). The "observable fact" layer is then someone's
	// interpretation already, so the insight rests on no primary
	// evidence.
	QualitySecondhandOnly QualityFlagCode = "secondhand_only"
)

type QualityFlag struct {
	Code   QualityFlagCode `json:"code"`
	Detail string          `json:"detail,omitempty"`
}

type Insight struct {
	ID         string
	ProjectID  string
	AnalysisID *string
	Title      string
	// Observation is the summary of directly verifiable facts.
	Observation string
	StatedNeed  string
	LatentNeed  string
	JTBD        string
	// Expectation / SurprisingFact / Rationale form the abductive triad
	// behind the hypothesis:
	//   Expectation    - what common sense predicted the person would do
	//   SurprisingFact - what they actually did that breaks the prediction
	//   Rationale      - why, if LatentNeed were true, the surprising fact
	//                    would become a matter of course
	Expectation               string
	SurprisingFact            string
	Rationale                 string
	Interpretation            string
	AlternativeInterpretation string
	ProductOpportunity        string
	MonetizationAngle         string
	Confidence                float64
	QualityFlags              []QualityFlag
	Evidence                  []Evidence
	CreatedAt                 time.Time
}

// HasQualityFlag reports whether the insight carries the given flag.
func (i *Insight) HasQualityFlag(code QualityFlagCode) bool {
	for _, f := range i.QualityFlags {
		if f.Code == code {
			return true
		}
	}
	return false
}
