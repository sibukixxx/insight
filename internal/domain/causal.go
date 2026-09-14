package domain

// CausalStatus describes the strength of a causal claim. It is deliberately
// separate from Confidence, which is only an internal evidence-quality score.
type CausalStatus string

const (
	CausalObservedAssociation CausalStatus = "OBSERVED_ASSOCIATION"
	CausalHypothesis          CausalStatus = "CAUSAL_HYPOTHESIS"
	CausallySupported         CausalStatus = "CAUSALLY_SUPPORTED"
	CausalNotIdentified       CausalStatus = "NOT_IDENTIFIED"
)

type ValidationStatus string

const (
	ValidationUntested             ValidationStatus = "UNTESTED"
	ValidationPlausible            ValidationStatus = "PLAUSIBLE"
	ValidationPartiallySupported   ValidationStatus = "PARTIALLY_SUPPORTED"
	ValidationSupported            ValidationStatus = "SUPPORTED"
	ValidationInsufficientEvidence ValidationStatus = "INSUFFICIENT_EVIDENCE"
	ValidationContradicted         ValidationStatus = "CONTRADICTED"
)

type IdentificationStatus string

const (
	IdentificationIdentified    IdentificationStatus = "IDENTIFIED"
	IdentificationNotIdentified IdentificationStatus = "NOT_IDENTIFIED"
	IdentificationUnknown       IdentificationStatus = "UNKNOWN"
)

type ExpectationBasis string

const (
	ExpectationSourceBacked  ExpectationBasis = "SOURCE_BACKED"
	ExpectationModelProposed ExpectationBasis = "MODEL_PROPOSED"
	ExpectationUnknown       ExpectationBasis = "UNKNOWN"
)

type VariableRole string

const (
	RoleExposure   VariableRole = "EXPOSURE"
	RoleOutcome    VariableRole = "OUTCOME"
	RoleConfounder VariableRole = "CONFOUNDER"
	RoleMediator   VariableRole = "MEDIATOR"
	RoleCollider   VariableRole = "COLLIDER"
	RoleUnknown    VariableRole = "UNKNOWN"
)

type ProposalStatus string

const (
	ProposalProposed  ProposalStatus = "PROPOSED"
	ProposalSupported ProposalStatus = "SUPPORTED"
	ProposalUnknown   ProposalStatus = "UNKNOWN"
)

type CausalVariable struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Role   VariableRole   `json:"role"`
	Status ProposalStatus `json:"status"`
}

type CausalRelation struct {
	From   string         `json:"from"`
	To     string         `json:"to"`
	Status ProposalStatus `json:"status"`
}

// CandidateCausalStructure is a proposal to inspect, never a discovered or
// proven DAG. In particular, collider variables must not automatically become
// controls.
type CandidateCausalStructure struct {
	Variables []CausalVariable `json:"variables"`
	Relations []CausalRelation `json:"relations"`
}

type CompetingHypothesis struct {
	Title                 string   `json:"title"`
	Explanation           string   `json:"explanation"`
	Rationale             string   `json:"rationale,omitempty"`
	MissingEvidence       []string `json:"missingEvidence,omitempty"`
	FalsificationCriteria []string `json:"falsificationCriteria,omitempty"`
	RequiredData          []string `json:"requiredData,omitempty"`
	RequiredComparisons   []string `json:"requiredComparisons,omitempty"`
	CandidateDesigns      []string `json:"candidateDesigns,omitempty"`
}

type HypothesisRole string

const (
	HypothesisPrimary   HypothesisRole = "PRIMARY"
	HypothesisCompeting HypothesisRole = "COMPETING"
)

type ValidationNeed struct {
	Data       []string `json:"data"`
	Comparison []string `json:"comparison"`
	Design     []string `json:"design"`
}
