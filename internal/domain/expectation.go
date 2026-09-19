package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// AuthorType records who wrote an Expectation. It is orthogonal to
// provenance: a human can write a post-hoc expectation and a model can
// restate a literature-backed one.
type AuthorType string

const (
	AuthorHuman    AuthorType = "HUMAN"
	AuthorModel    AuthorType = "MODEL"
	AuthorImported AuthorType = "IMPORTED"
)

func (a AuthorType) Valid() bool {
	switch a {
	case AuthorHuman, AuthorModel, AuthorImported:
		return true
	}
	return false
}

var (
	ErrExpectationStatementRequired     = errors.New("expectation: statement is required")
	ErrExpectationProvenanceInvalid     = errors.New("expectation: provenance is not a known ExpectationBasis")
	ErrExpectationProvenanceUnknown     = errors.New("expectation: provenance is unknown")
	ErrExpectationAuthorInvalid         = errors.New("expectation: author type is invalid")
	ErrExpectationCreatedAtRequired     = errors.New("expectation: createdAt is required")
	ErrExpectationSourceRequired        = errors.New("expectation: LITERATURE/DOMAIN_KNOWLEDGE provenance needs a source reference or justification")
	ErrExpectationPriorAfterObservation = errors.New("expectation: pre-observation provenance but observed data was available at creation")
	// ErrExpectationPostHocWithoutObservation guards the flag in the other
	// direction: a post-hoc origin cannot claim the data was unseen.
	ErrExpectationPostHocWithoutObservation = errors.New("expectation: post-hoc provenance but observed data marked unavailable at creation")
	ErrExpectationCriteriaRequired          = errors.New("expectation: at least one falsification criterion is required to freeze")
	ErrExpectationAlreadyFrozen             = errors.New("expectation: already frozen for validation")
)

// Expectation is a statement about what the data should show, recorded with
// enough provenance that a reader can tell whether it was fixed before the
// data was observed. It is the unit that a research iteration freezes and
// then tests; Insight.Expectation remains the prose summary shown alongside
// the surprising fact.
//
// Values are treated as append-only history: Freeze and DeriveForValidation
// return copies and never mutate the receiver.
type Expectation struct {
	ID                  string           `json:"id"`
	Statement           string           `json:"statement"`
	Provenance          ExpectationBasis `json:"provenance"`
	AuthorType          AuthorType       `json:"authorType"`
	ResearchIterationID string           `json:"researchIterationId"`
	// SourceReferences are citations, URLs, DOIs or document IDs that let a
	// reader follow LITERATURE / DOMAIN_KNOWLEDGE origins.
	SourceReferences []string `json:"sourceReferences,omitempty"`
	// Justification is the free-text reason a DOMAIN_KNOWLEDGE expectation is
	// held, when no citable source exists.
	Justification string `json:"justification,omitempty"`
	// ObservedDataAvailableAtCreation is true when the author (human or
	// model) could see the data of ResearchIterationID while writing the
	// statement. It refers to this iteration's data only.
	ObservedDataAvailableAtCreation bool `json:"observedDataAvailableAtCreation"`
	// FalsificationCriteria state what observation would count against the
	// expectation. They must exist before the expectation can be frozen.
	FalsificationCriteria []string `json:"falsificationCriteria,omitempty"`
	// FrozenForValidation marks that statement and criteria are fixed and
	// may no longer be edited before evidence is collected.
	FrozenForValidation bool       `json:"frozenForValidation"`
	FrozenAt            *time.Time `json:"frozenAt,omitempty"`
	// DerivedFromExpectationID links an expectation carried into a new
	// iteration (DERIVED_FROM_PRIOR_RUN) back to its exploratory source.
	DerivedFromExpectationID string    `json:"derivedFromExpectationId,omitempty"`
	CreatedAt                time.Time `json:"createdAt"`
}

// EffectiveProvenance is Provenance with legacy and missing values
// normalised. Missing provenance is reported as UNKNOWN, never guessed.
func (e Expectation) EffectiveProvenance() ExpectationBasis {
	return e.Provenance.Normalize()
}

// Validate checks structural completeness and the provenance/timing
// invariants. Missing provenance is allowed (it stays UNKNOWN); an invalid
// value is not.
func (e Expectation) Validate() error {
	if strings.TrimSpace(e.Statement) == "" {
		return ErrExpectationStatementRequired
	}
	if e.Provenance != "" && !e.Provenance.Valid() {
		return fmt.Errorf("%w: %q", ErrExpectationProvenanceInvalid, e.Provenance)
	}
	if !e.AuthorType.Valid() {
		return fmt.Errorf("%w: %q", ErrExpectationAuthorInvalid, e.AuthorType)
	}
	if e.CreatedAt.IsZero() {
		return ErrExpectationCreatedAtRequired
	}
	provenance := e.EffectiveProvenance()
	if provenance.RequiresSourceReference() && len(e.SourceReferences) == 0 && strings.TrimSpace(e.Justification) == "" {
		return fmt.Errorf("%w: %q", ErrExpectationSourceRequired, provenance)
	}
	switch provenance.ObservationTiming() {
	case TimingPreObservation:
		if e.ObservedDataAvailableAtCreation {
			return fmt.Errorf("%w: %q", ErrExpectationPriorAfterObservation, provenance)
		}
	case TimingPostObservation:
		if !e.ObservedDataAvailableAtCreation {
			return fmt.Errorf("%w: %q", ErrExpectationPostHocWithoutObservation, provenance)
		}
	}
	return nil
}

// CreatedBeforeObservation reports whether the expectation may be presented
// as a pre-observation prediction for this iteration's data. Post-hoc and
// unknown origins, and anything written with the data in view, are excluded.
func (e Expectation) CreatedBeforeObservation() bool {
	return !e.ObservedDataAvailableAtCreation && e.EffectiveProvenance().ObservationTiming() == TimingPreObservation
}

// Freeze fixes the statement and criteria for validation and returns the
// frozen copy. Post-hoc expectations may be frozen so that a later
// iteration can test them; freezing never changes provenance.
func (e Expectation) Freeze(at time.Time) (Expectation, error) {
	if e.FrozenForValidation {
		return Expectation{}, ErrExpectationAlreadyFrozen
	}
	if err := e.Validate(); err != nil {
		return Expectation{}, err
	}
	if e.EffectiveProvenance() == ExpectationUnknown {
		return Expectation{}, ErrExpectationProvenanceUnknown
	}
	if len(e.FalsificationCriteria) == 0 {
		return Expectation{}, ErrExpectationCriteriaRequired
	}
	out := e.clone()
	out.FrozenForValidation = true
	frozenAt := at
	out.FrozenAt = &frozenAt
	return out, nil
}

// DeriveForValidation carries an exploratory expectation into a new research
// iteration. The copy is unfrozen, marked DERIVED_FROM_PRIOR_RUN, records its
// lineage, and states that the new iteration's data was not yet observed.
// The source expectation keeps its own post-hoc provenance.
func (e Expectation) DeriveForValidation(newID, newIterationID string, at time.Time) Expectation {
	out := e.clone()
	out.ID = newID
	out.ResearchIterationID = newIterationID
	out.Provenance = ExpectationDerivedFromPriorRun
	out.DerivedFromExpectationID = e.ID
	out.ObservedDataAvailableAtCreation = false
	out.FrozenForValidation = false
	out.FrozenAt = nil
	out.CreatedAt = at
	return out
}

// IsIndependentEvidence reports whether evidence collected in the given
// research iteration is independent of the process that produced this
// expectation. Evidence from the same iteration only counts when the
// expectation was fixed before that iteration's data was observed.
func (e Expectation) IsIndependentEvidence(evidenceIterationID string) bool {
	if evidenceIterationID == "" {
		return false
	}
	if evidenceIterationID != e.ResearchIterationID {
		return true
	}
	return !e.ObservedDataAvailableAtCreation
}

// PermitsStrongValidationClaim reports whether a result against this
// expectation may be reported as a validation result rather than an
// exploratory finding: the expectation must be frozen, created before the
// data was observed, and of a provenance that allows it.
func (e Expectation) PermitsStrongValidationClaim() bool {
	return e.FrozenForValidation && e.CreatedBeforeObservation() && e.EffectiveProvenance().PermitsStrongValidationClaim()
}

func (e Expectation) clone() Expectation {
	out := e
	out.SourceReferences = append([]string(nil), e.SourceReferences...)
	out.FalsificationCriteria = append([]string(nil), e.FalsificationCriteria...)
	if e.FrozenAt != nil {
		frozenAt := *e.FrozenAt
		out.FrozenAt = &frozenAt
	}
	return out
}
