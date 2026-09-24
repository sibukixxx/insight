package domain

import (
 "time"
 "insight-lab/internal/analytical/model"
)

type EvidenceType string

const (
	EvidenceSupport EvidenceType = "support"
	EvidenceCounter EvidenceType = "counter"
	EvidenceNeutral EvidenceType = "neutral"
)

type Evidence struct {
 Temporal *model.TemporalEvidence `json:"temporal,omitempty"`
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
	// QualityInsufficientCompetition means fewer than three explanations
	// were independently evaluated for the surprising fact.
	QualityInsufficientCompetition QualityFlagCode = "insufficient_competing_hypotheses"
)

type QualityFlag struct {
	Code   QualityFlagCode `json:"code"`
	Detail string          `json:"detail,omitempty"`
}


// InsightReferenceKind identifies what kind of grounded object a connection
// endpoint points at. References stay provider-neutral so the same semantic
// model works for interviews, structured datasets and research reviews.
type InsightReferenceKind string

const (
	InsightRefObservation InsightReferenceKind = "OBSERVATION"
	InsightRefFinding     InsightReferenceKind = "FINDING"
	InsightRefVariable    InsightReferenceKind = "VARIABLE"
	InsightRefClaim       InsightReferenceKind = "CLAIM"
	InsightRefContext     InsightReferenceKind = "CONTEXT"
	InsightRefOther       InsightReferenceKind = "OTHER"
)

func (k InsightReferenceKind) Valid() bool {
	switch k {
	case InsightRefObservation, InsightRefFinding, InsightRefVariable, InsightRefClaim, InsightRefContext, InsightRefOther:
		return true
	}
	return false
}

type InsightReference struct {
	Kind  InsightReferenceKind `json:"kind"`
	ID    string               `json:"id,omitempty"`
	Label string               `json:"label,omitempty"`
}

type ConnectionKind string

const (
	ConnectionAssociation        ConnectionKind = "ASSOCIATION"
	ConnectionContrast           ConnectionKind = "CONTRAST"
	ConnectionSequence           ConnectionKind = "SEQUENCE"
	ConnectionInteraction        ConnectionKind = "INTERACTION"
	ConnectionConstraint         ConnectionKind = "CONSTRAINT"
	ConnectionMechanismCandidate ConnectionKind = "MECHANISM_CANDIDATE"
	ConnectionOther              ConnectionKind = "OTHER"
)

func (k ConnectionKind) Valid() bool {
	switch k {
	case ConnectionAssociation, ConnectionContrast, ConnectionSequence, ConnectionInteraction,
		ConnectionConstraint, ConnectionMechanismCandidate, ConnectionOther:
		return true
	}
	return false
}

// InsightConnection is a proposed non-obvious relation between grounded
// objects. Kind is descriptive only; it never promotes causal status.
type InsightConnection struct {
	Sources   []InsightReference `json:"sources,omitempty"`
	Targets   []InsightReference `json:"targets,omitempty"`
	Kind      ConnectionKind     `json:"kind,omitempty"`
	Statement string             `json:"statement,omitempty"`
	WhyItMatters string          `json:"whyItMatters,omitempty"`
}

// MechanismStep is one bridge in a candidate explanatory chain. EvidenceRefs
// point to existing evidence/artifact identifiers; an unsupported step stays
// explicit instead of being hidden inside persuasive prose.
type MechanismStep struct {
	Statement      string   `json:"statement"`
	EvidenceRefs   []string `json:"evidenceRefs,omitempty"`
	Assumptions    []string `json:"assumptions,omitempty"`
	MissingEvidence []string `json:"missingEvidence,omitempty"`
}

type MechanismCandidate struct {
	Statement            string          `json:"statement,omitempty"`
	Steps                []MechanismStep `json:"steps,omitempty"`
	AlternativeMechanisms []string       `json:"alternativeMechanisms,omitempty"`
	CounterEvidenceRefs  []string        `json:"counterEvidenceRefs,omitempty"`
	FalsificationCriteria []string       `json:"falsificationCriteria,omitempty"`
}

type GeneralizationStatus string

const (
	GeneralizationCandidate GeneralizationStatus = "CANDIDATE"
	GeneralizationSupported GeneralizationStatus = "SUPPORTED"
	GeneralizationRejected  GeneralizationStatus = "REJECTED"
)

func (s GeneralizationStatus) Valid() bool {
	return s == "" || s == GeneralizationCandidate || s == GeneralizationSupported || s == GeneralizationRejected
}

// InsightGeneralization is deliberately a candidate transfer statement, not a
// model-certified universal law. Human review/additional evidence owns any
// promotion beyond CANDIDATE.
type InsightGeneralization struct {
	SourceContext        string               `json:"sourceContext,omitempty"`
	Principle            string               `json:"principle,omitempty"`
	TargetContext        string               `json:"targetContext,omitempty"`
	ApplicabilityConditions []string          `json:"applicabilityConditions,omitempty"`
	BoundaryConditions   []string             `json:"boundaryConditions,omitempty"`
	KnownFailureConditions []string           `json:"knownFailureConditions,omitempty"`
	Status               GeneralizationStatus `json:"status,omitempty"`
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
	ExpectationBasis          ExpectationBasis
	SurprisingFact            string
	Rationale                 string
	Interpretation            string
	AlternativeInterpretation string
	Connection                InsightConnection
	Mechanism                 MechanismCandidate
	Generalization            InsightGeneralization
	ProductOpportunity        string
	MonetizationAngle         string
	Confidence                float64
	HypothesisSetID           string
	HypothesisRole            HypothesisRole
	CausalStatus              CausalStatus
	ValidationStatus          ValidationStatus
	IdentificationStatus      IdentificationStatus
	CompetingHypotheses       []CompetingHypothesis
	CausalStructure           CandidateCausalStructure
	MissingEvidence           []string
	FalsificationCriteria     []string
	NextValidation            ValidationNeed
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
