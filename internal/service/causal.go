package service

import "insight-lab/internal/domain"

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

func expectationBasis(value string) domain.ExpectationBasis {
	switch domain.ExpectationBasis(value) {
	case domain.ExpectationSourceBacked, domain.ExpectationModelProposed, domain.ExpectationUnknown:
		return domain.ExpectationBasis(value)
	default:
		return domain.ExpectationModelProposed
	}
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
