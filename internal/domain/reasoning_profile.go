package domain

// ReasoningProfile selects what kind of synthesis the caller wants from the
// same evidence reasoning core (#109). It is independent of AnalysisMode (how
// input is interpreted), ResearchStage, ExecutionMode (deterministic /
// model-backed) and ExecutionProfile (resource strategy). A profile changes
// synthesis emphasis, never epistemic standards: grounding, counter-evidence,
// causal guardrails, ResearchGap and iteration semantics are shared.
//
// The profile is always explicit. It is never inferred from evidence source,
// subject namespace or document wording.
type ReasoningProfile string

const (
	// ReasoningProfileGeneralResearch is the domain-neutral, question-
	// conditioned research behavior (#107/#108) and the default.
	ReasoningProfileGeneralResearch ReasoningProfile = "GENERAL_RESEARCH"
	// ReasoningProfileCustomerInsight is the explicit customer-research
	// specialization: stated vs latent needs, behavioral traces and JTBD.
	ReasoningProfileCustomerInsight ReasoningProfile = "CUSTOMER_INSIGHT"
)

// ReasoningProfiles lists the supported profiles, default first.
func ReasoningProfiles() []ReasoningProfile {
	return []ReasoningProfile{ReasoningProfileGeneralResearch, ReasoningProfileCustomerInsight}
}

func (p ReasoningProfile) Valid() bool {
	switch p {
	case ReasoningProfileGeneralResearch, ReasoningProfileCustomerInsight:
		return true
	}
	return false
}

// Normalize maps an omitted profile to GENERAL_RESEARCH so requests that
// predate profiles keep the current domain-neutral behavior.
func (p ReasoningProfile) Normalize() ReasoningProfile {
	if p == "" {
		return ReasoningProfileGeneralResearch
	}
	return p
}

// PromptMarker is the line that opens a non-default profile's prompt
// extension. It makes the selected profile visible in recorded prompts and
// lets deterministic test models answer per profile.
func (p ReasoningProfile) PromptMarker() string {
	return "Reasoning profile: " + string(p.Normalize()) + "."
}
