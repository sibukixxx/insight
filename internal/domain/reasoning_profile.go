package domain

// ReasoningProfile selects the semantic specialization applied by the shared
// evidence-reasoning pipeline. It is independent from AnalysisMode,
// ResearchStage, ExecutionMode and ExecutionProfile.
//
// GENERAL_RESEARCH is the default and must remain domain-neutral.
// CUSTOMER_INSIGHT explicitly enables the legacy customer hidden-need/JTBD
// specialization without creating a second research engine.
type ReasoningProfile string

const (
	ReasoningGeneralResearch ReasoningProfile = "GENERAL_RESEARCH"
	ReasoningCustomerInsight ReasoningProfile = "CUSTOMER_INSIGHT"
)

func (p ReasoningProfile) Normalize() ReasoningProfile {
	if p == "" {
		return ReasoningGeneralResearch
	}
	return p
}

func (p ReasoningProfile) Valid() bool {
	switch p.Normalize() {
	case ReasoningGeneralResearch, ReasoningCustomerInsight:
		return true
	}
	return false
}
