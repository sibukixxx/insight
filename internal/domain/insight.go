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
	// QualityInsufficientCompetition means fewer than three explanations
	// were independently evaluated for the surprising fact.
	QualityInsufficientCompetition QualityFlagCode = "insufficient_competing_hypotheses"
)

type QualityFlag struct {
	Code   QualityFlagCode `json:"code"`
	Detail string          `json:"detail,omitempty"`
}

// ConnectionReferenceKind identifies the kind of already-observed or externally
// supplied node an InsightConnection connects. These references are pointers
// into the evidence/research model; their existence does not make the
// connection true.
type ConnectionReferenceKind string

const (
	ConnectionRefObservation ConnectionReferenceKind = "OBSERVATION"
	ConnectionRefFinding     ConnectionReferenceKind = "FINDING"
	ConnectionRefVariable    ConnectionReferenceKind = "VARIABLE"
	ConnectionRefClaim       ConnectionReferenceKind = "CLAIM"
	ConnectionRefContext     ConnectionReferenceKind = "CONTEXT"
	ConnectionRefDataset     ConnectionReferenceKind = "DATASET"
	ConnectionRefOther       ConnectionReferenceKind = "OTHER"
)

type ConnectionReference struct {
	Kind  ConnectionReferenceKind `json:"kind"`
	ID    string                  `json:"id,omitempty"`
	Label string                  `json:"label,omitempty"`
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

// InsightConnection represents the non-obvious line between facts/variables/
// contexts. It is deliberately separate from CausalStatus: a coherent line
// or sequence is not evidence that causality has been identified.
type InsightConnection struct {
	ID           string                `json:"id,omitempty"`
	Kind         ConnectionKind        `json:"kind"`
	Statement    string                `json:"statement"`
	From         []ConnectionReference `json:"from,omitempty"`
	To           []ConnectionReference `json:"to,omitempty"`
	WhyItMatters string                `json:"whyItMatters,omitempty"`
}

// MechanismCandidate keeps the explanatory bridge inspectable. A model may
// propose it, but status/evidence are not a truth probability and never
// override Insight.CausalStatus or IdentificationStatus.
type MechanismCandidate struct {
	Statement             string   `json:"statement"`
	BridgeAssumptions     []string `json:"bridgeAssumptions,omitempty"`
	SupportingEvidenceIDs []string `json:"supportingEvidenceIds,omitempty"`
	CounterEvidenceIDs    []string `json:"counterEvidenceIds,omitempty"`
	AlternativeMechanisms []string `json:"alternativeMechanisms,omitempty"`
	UnresolvedGaps        []string `json:"unresolvedGaps,omitempty"`
	DistinguishingEvidence []string `json:"distinguishingEvidence,omitempty"`
}

type GeneralizationStatus string

const (
	GeneralizationCandidate GeneralizationStatus = "CANDIDATE"
	GeneralizationSupported GeneralizationStatus = "SUPPORTED"
	GeneralizationRejected  GeneralizationStatus = "REJECTED"
)

// GeneralizationCandidate describes a possible transferable principle. It can
// only be presented as supported after an explicit human review; the LLM may
// propose the candidate and its boundaries but cannot self-certify transfer.
type GeneralizationCandidate struct {
	Principle         string               `json:"principle"`
	SourceContext     string               `json:"sourceContext,omitempty"`
	TargetContexts    []string             `json:"targetContexts,omitempty"`
	BoundaryConditions []string            `json:"boundaryConditions,omitempty"`
	KnownFailures     []string             `json:"knownFailures,omitempty"`
	Status            GeneralizationStatus `json:"status,omitempty"`
	HumanReviewed     bool                 `json:"humanReviewed"`
}

func (g GeneralizationCandidate) PermitsTransferClaim() bool {
	return g.Status == GeneralizationSupported && g.HumanReviewed
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
	ProductOpportunity        string
	MonetizationAngle         string
	Confidence                float64
	HypothesisSetID           string
	HypothesisRole            HypothesisRole
	CausalStatus              CausalStatus
	ValidationStatus          ValidationStatus
	IdentificationStatus      IdentificationStatus
	CompetingHypotheses       []CompetingHypothesis
	Connections               []InsightConnection
	Mechanisms                []MechanismCandidate
	Generalizations           []GeneralizationCandidate
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
