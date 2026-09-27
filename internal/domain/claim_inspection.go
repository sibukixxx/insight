package domain

// Claim inspection (#119) checks an external claim against the research
// state an iteration already holds. It never creates evidence from a claim
// and never scores a claim's truth: the status is derived from the existing
// validation, causal and identification states of the hypotheses the claim
// relies on, and it is never stronger than those states allow.

// ClaimStatus is the derived inspection result of one claim.
type ClaimStatus string

const (
	// ClaimSupported: every hypothesis the claim relies on is supported or
	// partially supported, no competing explanation over the same evidence
	// is still open, and the claim's wording does not exceed them.
	ClaimSupported ClaimStatus = "SUPPORTED"
	// ClaimContradicted: every hypothesis the claim relies on is contradicted.
	ClaimContradicted ClaimStatus = "CONTRADICTED"
	// ClaimInsufficient: the claim relies on hypotheses, but the research
	// state cannot settle it (untested, mixed, competing or over-worded).
	ClaimInsufficient ClaimStatus = "INSUFFICIENT"
	// ClaimUnknown: no inspected hypothesis uses the evidence the claim cites.
	ClaimUnknown ClaimStatus = "UNKNOWN"
)

// ClaimFlagCode names a deterministic hint about a claim. Flags are hints for
// a reviewer, like quality flags; keyword-based flags can miss wording.
type ClaimFlagCode string

const (
	ClaimFlagNoSourceCitation         ClaimFlagCode = "NO_SOURCE_CITATION"
	ClaimFlagCitationNotFound         ClaimFlagCode = "CITATION_NOT_FOUND"
	ClaimFlagHypothesisNotFound       ClaimFlagCode = "HYPOTHESIS_NOT_FOUND"
	ClaimFlagCausalWithoutID          ClaimFlagCode = "CAUSAL_LANGUAGE_WITHOUT_IDENTIFICATION"
	ClaimFlagOvergeneralization       ClaimFlagCode = "OVERGENERALIZATION_LANGUAGE"
	ClaimFlagDefinitionMismatch       ClaimFlagCode = "DEFINITION_OR_POPULATION_MISMATCH"
	ClaimFlagConflictingEvidence      ClaimFlagCode = "CONFLICTING_EVIDENCE"
	ClaimFlagCompetingExplanationOpen ClaimFlagCode = "COMPETING_EXPLANATION_OPEN"
)

type ClaimFlag struct {
	Code   ClaimFlagCode `json:"code"`
	Detail string        `json:"detail,omitempty"`
}

// ClaimCitationState says whether a cited reference exists in the subject's
// evidence. A NOT_FOUND citation is never turned into an observation.
type ClaimCitationState string

const (
	ClaimCitationResolved ClaimCitationState = "RESOLVED"
	ClaimCitationNotFound ClaimCitationState = "NOT_FOUND"
)

type ClaimCitation struct {
	Reference     string             `json:"reference"`
	State         ClaimCitationState `json:"state"`
	DocumentID    string             `json:"documentId,omitempty"`
	ObservationID string             `json:"observationId,omitempty"`
}

// ClaimHypothesisLink records the existing states a claim status was derived
// from, so the derivation stays auditable.
type ClaimHypothesisLink struct {
	InsightID            string               `json:"insightId"`
	Title                string               `json:"title,omitempty"`
	Role                 HypothesisRole       `json:"role,omitempty"`
	CausalStatus         CausalStatus         `json:"causalStatus,omitempty"`
	ValidationStatus     ValidationStatus     `json:"validationStatus,omitempty"`
	IdentificationStatus IdentificationStatus `json:"identificationStatus,omitempty"`
}

// ClaimEvidence points at existing grounded evidence; it copies the quote for
// reading, never creates new evidence.
type ClaimEvidence struct {
	EvidenceID    string       `json:"evidenceId"`
	InsightID     string       `json:"insightId"`
	DocumentID    string       `json:"documentId"`
	ObservationID string       `json:"observationId,omitempty"`
	Quote         string       `json:"quote"`
	Type          EvidenceType `json:"type"`
}

// ClaimPreviousInspection links a re-inspection to the earlier iteration.
type ClaimPreviousInspection struct {
	IterationID string      `json:"iterationId"`
	Status      ClaimStatus `json:"status"`
	Changed     bool        `json:"changed"`
}

type ClaimInspection struct {
	ClaimID     string `json:"claimId"`
	ContentHash string `json:"contentHash"`
	RuleVersion string `json:"ruleVersion"`
	// DuplicateOf names an earlier claim with identical content in the same
	// request; the copy is inspected identically and not counted twice.
	DuplicateOf           string                   `json:"duplicateOf,omitempty"`
	Status                ClaimStatus              `json:"status"`
	Basis                 []ClaimHypothesisLink    `json:"basis,omitempty"`
	CompetingHypotheses   []ClaimHypothesisLink    `json:"competingHypotheses,omitempty"`
	Citations             []ClaimCitation          `json:"citations,omitempty"`
	SupportingEvidence    []ClaimEvidence          `json:"supportingEvidence,omitempty"`
	CounterEvidence       []ClaimEvidence          `json:"counterEvidence,omitempty"`
	NeutralEvidence       []ClaimEvidence          `json:"neutralEvidence,omitempty"`
	Flags                 []ClaimFlag              `json:"flags,omitempty"`
	UnverifiedAssumptions []string                 `json:"unverifiedAssumptions,omitempty"`
	RequiredEvidence      []string                 `json:"requiredEvidence,omitempty"`
	ResearchGapIDs        []string                 `json:"researchGapIds,omitempty"`
	CannotConclude        []string                 `json:"cannotConclude,omitempty"`
	Previous              *ClaimPreviousInspection `json:"previous,omitempty"`
}
