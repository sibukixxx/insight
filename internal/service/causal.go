package service

import "insight-lab/internal/domain"

// expandCompetingHypotheses promotes alternative explanations from model
// suggestions into first-class candidates. The ordinary pipeline will then
// retrieve evidence, counter-evidence and calculate validation independently
// for every returned item.
func expandCompetingHypotheses(input []hypothesisCandidate) []hypothesisCandidate {
	var out []hypothesisCandidate
	for _, primary := range input {
		setID := newID("hset")
		size := 1 + len(primary.AlternativeExplanations)
		primary.HypothesisSetID = setID
		primary.HypothesisRole = domain.HypothesisPrimary
		primary.HypothesisSetSize = size
		alternatives := primary.AlternativeExplanations
		primary.AlternativeExplanations = nil
		out = append(out, primary)

		for _, alternative := range alternatives {
			candidate := primary
			candidate.Title = alternative.Title
			candidate.LatentNeed = alternative.Explanation
			candidate.Rationale = alternative.Rationale
			if candidate.Rationale == "" {
				candidate.Rationale = alternative.Explanation
			}
			candidate.HypothesisRole = domain.HypothesisCompeting
			candidate.AlternativeExplanations = nil
			candidate.CandidateCausalStructure = domain.CandidateCausalStructure{}
			candidate.MissingEvidence = alternative.MissingEvidence
			candidate.FalsificationCriteria = alternative.FalsificationCriteria
			candidate.RequiredData = alternative.RequiredData
			candidate.RequiredComparisons = alternative.RequiredComparisons
			candidate.CandidateDesigns = alternative.CandidateDesigns
			out = append(out, candidate)
		}
	}
	return out
}

type CausalAssessmentInput struct {
	ExpectationBasis      string
	Alternatives          []domain.CompetingHypothesis
	Structure             domain.CandidateCausalStructure
	MissingEvidence       []string
	FalsificationCriteria []string
	RequiredData          []string
	RequiredComparisons   []string
	CandidateDesigns      []string
	SupportingCount       int
	CounterCount          int
}

// AssessCausalReadiness applies deterministic guardrails after model output.
// The current pipeline has grounded observations but performs no causal study,
// so it cannot emit CAUSALLY_SUPPORTED or IDENTIFIED.
func AssessCausalReadiness(in CausalAssessmentInput) (domain.CausalStatus, domain.ValidationStatus, domain.IdentificationStatus) {
	validation := domain.ValidationUntested
	switch {
	case in.SupportingCount == 0:
		validation = domain.ValidationInsufficientEvidence
	case in.CounterCount >= in.SupportingCount:
		validation = domain.ValidationContradicted
	case in.CounterCount > 0:
		validation = domain.ValidationPartiallySupported
	default:
		validation = domain.ValidationPlausible
	}
	return domain.CausalHypothesis, validation, domain.IdentificationNotIdentified
}

// expectationBasis validates a raw provenance string against the full
// ExpectationBasis vocabulary (issue #22), not just the three legacy values
// the pipeline used to emit. A value the pipeline did not recognize is never
// guessed as MODEL_PROPOSED; it is recorded as UNKNOWN so it cannot later
// support a strong validation claim.
func expectationBasis(value string) domain.ExpectationBasis {
	basis := domain.ExpectationBasis(value)
	if !basis.Valid() {
		return domain.ExpectationUnknown
	}
	return basis
}

// InsightFindingKind labels a result produced while a research iteration is
// in stage using the originating expectation's provenance. Reaching
// VALIDATION never promotes an individual expectation whose own provenance
// could not support a strong validation claim (post-hoc, unknown, or
// legacy): it is still reported as an exploratory finding.
func InsightFindingKind(stage domain.ResearchStage, basis domain.ExpectationBasis) domain.FindingKind {
	kind := stage.FindingKind()
	if kind == domain.FindingValidationResult && !basis.Normalize().PermitsStrongValidationClaim() {
		return domain.FindingExploratory
	}
	return kind
}

// CandidateControlVariables intentionally excludes colliders and mediators;
// proposed causal roles are review inputs, not an automatic adjustment set.
func CandidateControlVariables(structure domain.CandidateCausalStructure) []domain.CausalVariable {
	var out []domain.CausalVariable
	for _, variable := range structure.Variables {
		if variable.Role == domain.RoleConfounder && variable.Status == domain.ProposalSupported {
			out = append(out, variable)
		}
	}
	return out
}
