package domain

import (
	"errors"
	"fmt"
	"time"
)

// ContributionType names the kind of value a research artifact offers a
// reader before it may be promoted toward a public report. It exists so
// promotion never depends on manufacturing a surprising result: a
// correction or replication is an accepted reason to publish on its own.
type ContributionType string

const (
	ContributionNovelMismatch                   ContributionType = "NOVEL_MISMATCH"
	ContributionCorrection                      ContributionType = "CORRECTION"
	ContributionRefinement                      ContributionType = "REFINEMENT"
	ContributionReplication                     ContributionType = "REPLICATION"
	ContributionDefinitionAudit                 ContributionType = "DEFINITION_AUDIT"
	ContributionCounterEvidence                 ContributionType = "COUNTER_EVIDENCE"
	ContributionInconclusiveButDecisionRelevant ContributionType = "INCONCLUSIVE_BUT_DECISION_RELEVANT"
	ContributionOther                           ContributionType = "OTHER"
)

func (c ContributionType) Valid() bool {
	switch c {
	case ContributionNovelMismatch, ContributionCorrection, ContributionRefinement,
		ContributionReplication, ContributionDefinitionAudit, ContributionCounterEvidence,
		ContributionInconclusiveButDecisionRelevant, ContributionOther:
		return true
	}
	return false
}

// PermitsPublicationValue reports whether the contribution type alone
// establishes that the artifact is worth publishing, without requiring a
// NOVEL_MISMATCH (issue #24, "Mismatch rule"). OTHER does not qualify on its
// own: an uncategorized contribution needs a human to say why it matters
// rather than being waved through.
func (c ContributionType) PermitsPublicationValue() bool {
	switch c {
	case ContributionNovelMismatch, ContributionCorrection, ContributionRefinement,
		ContributionReplication, ContributionDefinitionAudit, ContributionCounterEvidence,
		ContributionInconclusiveButDecisionRelevant:
		return true
	}
	return false
}

// PromotionState is where a research artifact stands on the path from raw
// research toward a public report. It is kept separate from
// DecisionReadiness: a run can be DECISION_READY_WITH_LIMITATIONS for
// internal use while remaining far from PUBLICATION_READY.
type PromotionState string

const (
	PromotionDraft                  PromotionState = "DRAFT"
	PromotionResearchComplete       PromotionState = "RESEARCH_COMPLETE"
	PromotionHumanReviewRequired    PromotionState = "HUMAN_REVIEW_REQUIRED"
	PromotionPublicationReady       PromotionState = "PUBLICATION_READY"
	PromotionPublished              PromotionState = "PUBLISHED"
	PromotionRejectedForPublication PromotionState = "REJECTED_FOR_PUBLICATION"
)

func (s PromotionState) Valid() bool {
	switch s {
	case PromotionDraft, PromotionResearchComplete, PromotionHumanReviewRequired,
		PromotionPublicationReady, PromotionPublished, PromotionRejectedForPublication:
		return true
	}
	return false
}

var promotionOrder = map[PromotionState]int{
	PromotionDraft:               0,
	PromotionResearchComplete:    1,
	PromotionHumanReviewRequired: 2,
	PromotionPublicationReady:    3,
	PromotionPublished:           4,
}

// PublicationChecklist is the machine-readable form of the minimum
// publication checks in issue #24. Every field defaults to false: nothing is
// assumed satisfied. Human review is tracked separately on
// PromotionGateInput because it is a first-class approval, not one box among
// many that could be set true by accident.
type PublicationChecklist struct {
	SourceProvenanceComplete              bool `json:"sourceProvenanceComplete"`
	DeterministicCalculationsReproducible bool `json:"deterministicCalculationsReproducible"`
	ObservationGrounded                   bool `json:"observationGrounded"`
	ExpectationProvenanceVisible          bool `json:"expectationProvenanceVisible"`
	ResearchStageVisible                  bool `json:"researchStageVisible"`
	ClaimEvidenceMappingComplete          bool `json:"claimEvidenceMappingComplete"`
	CompetingHypothesisConsidered         bool `json:"competingHypothesisConsidered"`
	CounterEvidenceSearched               bool `json:"counterEvidenceSearched"`
	LimitationsPresent                    bool `json:"limitationsPresent"`
	ResearchGapsDisclosed                 bool `json:"researchGapsDisclosed"`
	UnresolvableConclusionsDisclosed      bool `json:"unresolvableConclusionsDisclosed"`
	NoHiddenPopulationUnitPeriodMismatch  bool `json:"noHiddenPopulationUnitPeriodMismatch"`
	IndependentValidationStatusAccurate   bool `json:"independentValidationStatusAccurate"`
	DecisionReadinessHonestlyStated       bool `json:"decisionReadinessHonestlyStated"`
}

// checklistItems lists the checklist fields in the fixed order used by
// UnmetItems, paired with the machine-readable name reported for each.
func (c PublicationChecklist) checklistItems() []struct {
	name string
	ok   bool
} {
	return []struct {
		name string
		ok   bool
	}{
		{"source_provenance_complete", c.SourceProvenanceComplete},
		{"deterministic_calculations_reproducible", c.DeterministicCalculationsReproducible},
		{"observation_grounded", c.ObservationGrounded},
		{"expectation_provenance_visible", c.ExpectationProvenanceVisible},
		{"research_stage_visible", c.ResearchStageVisible},
		{"claim_evidence_mapping_complete", c.ClaimEvidenceMappingComplete},
		{"competing_hypothesis_considered", c.CompetingHypothesisConsidered},
		{"counter_evidence_searched", c.CounterEvidenceSearched},
		{"limitations_present", c.LimitationsPresent},
		{"research_gaps_disclosed", c.ResearchGapsDisclosed},
		{"unresolvable_conclusions_disclosed", c.UnresolvableConclusionsDisclosed},
		{"no_hidden_population_unit_period_mismatch", c.NoHiddenPopulationUnitPeriodMismatch},
		{"independent_validation_status_accurate", c.IndependentValidationStatusAccurate},
		{"decision_readiness_honestly_stated", c.DecisionReadinessHonestlyStated},
	}
}

// UnmetItems returns the machine-readable names of checklist items that are
// not yet satisfied, in a stable order.
func (c PublicationChecklist) UnmetItems() []string {
	var unmet []string
	for _, item := range c.checklistItems() {
		if !item.ok {
			unmet = append(unmet, item.name)
		}
	}
	return unmet
}

// Satisfied reports whether every checklist item is met.
func (c PublicationChecklist) Satisfied() bool {
	return len(c.UnmetItems()) == 0
}

var (
	ErrPromotionTransitionNotAllowed  = errors.New("promotion: transition not allowed")
	ErrPromotionContributionInvalid   = errors.New("promotion: contribution type is not a known ContributionType")
	ErrPromotionContributionRequired  = errors.New("promotion: a contribution type establishing publication value is required")
	ErrPromotionChecklistIncomplete   = errors.New("promotion: publication checklist is incomplete")
	ErrPromotionHumanReviewRequired   = errors.New("promotion: human review has not been completed for this promoted output")
	ErrPromotionUnresolvedCriticalGap = errors.New("promotion: an unresolved critical gap blocks a strong claim from being published")
)

// PromotionGateInput carries the facts a promotion transition is checked
// against. The application supplies them from the research artifact; the
// gate never infers them from prose and never approves itself.
type PromotionGateInput struct {
	// Contribution states why the artifact is worth reading. Required to
	// leave RESEARCH_COMPLETE.
	Contribution ContributionType
	// Checklist is the minimum publication check state. Required to reach
	// PUBLICATION_READY.
	Checklist PublicationChecklist
	// HumanReviewCompleted records an explicit human approval of this
	// promoted output. It is never set by the system itself.
	HumanReviewCompleted bool
	// MakesStrongClaim is true when the artifact asserts a validated,
	// decision-driving conclusion rather than an exploratory, inconclusive,
	// or explicitly limited one.
	MakesStrongClaim bool
	// HasUnresolvedCriticalGap is true when a known research gap that could
	// change the conclusion has not been closed or disclosed.
	HasUnresolvedCriticalGap bool
}

// PromotionAssessment is the system-computed furthest PromotionState
// reachable from a starting state given a PromotionGateInput, with the
// reason further promotion is blocked, if any. Reaching RESEARCH_COMPLETE
// says nothing about whether the work is a good headline; the two are kept
// separate throughout.
type PromotionAssessment struct {
	State      PromotionState `json:"state"`
	Reasons    []string       `json:"reasons,omitempty"`
	AssessedAt time.Time      `json:"assessedAt"`
}

// Transition checks whether moving from s to the target PromotionState is
// permitted. Moving to REJECTED_FOR_PUBLICATION is always allowed from any
// non-terminal state, since rejection never promotes a claim. All other
// moves are forward, one step at a time, and require the stated evidence.
func (s PromotionState) Transition(to PromotionState, in PromotionGateInput) error {
	if !s.Valid() || !to.Valid() {
		return fmt.Errorf("%w: %q -> %q", ErrPromotionTransitionNotAllowed, s, to)
	}
	if in.Contribution != "" && !in.Contribution.Valid() {
		return fmt.Errorf("%w: %q", ErrPromotionContributionInvalid, in.Contribution)
	}
	if to == PromotionRejectedForPublication {
		if s == PromotionPublished || s == PromotionRejectedForPublication {
			return fmt.Errorf("%w: %q -> %q", ErrPromotionTransitionNotAllowed, s, to)
		}
		return nil
	}
	if s == PromotionRejectedForPublication || s == PromotionPublished {
		return fmt.Errorf("%w: %q -> %q", ErrPromotionTransitionNotAllowed, s, to)
	}
	if promotionOrder[to] != promotionOrder[s]+1 {
		return fmt.Errorf("%w: %q -> %q", ErrPromotionTransitionNotAllowed, s, to)
	}
	switch to {
	case PromotionHumanReviewRequired:
		if !in.Contribution.Valid() || !in.Contribution.PermitsPublicationValue() {
			return ErrPromotionContributionRequired
		}
	case PromotionPublicationReady, PromotionPublished:
		if !in.Contribution.PermitsPublicationValue() {
			return ErrPromotionContributionRequired
		}
		if unmet := in.Checklist.UnmetItems(); len(unmet) > 0 {
			return fmt.Errorf("%w: %v", ErrPromotionChecklistIncomplete, unmet)
		}
		if !in.HumanReviewCompleted {
			return ErrPromotionHumanReviewRequired
		}
		if in.MakesStrongClaim && in.HasUnresolvedCriticalGap {
			return ErrPromotionUnresolvedCriticalGap
		}
	}
	return nil
}
