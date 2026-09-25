package service

import "insight-lab/internal/domain"

// customerInsightMarker opens every CUSTOMER_INSIGHT prompt extension. It
// makes the explicitly selected profile visible in the recorded prompt and
// lets model-backed test doubles (internal/llm/scripted) answer per profile.
var customerInsightMarker = domain.ReasoningProfileCustomerInsight.PromptMarker()

// customerInsightPreamble is shared by every CUSTOMER_INSIGHT stage. It adds
// customer-research vocabulary on top of the shared invariants; it never
// relaxes grounding, evidence, counter-evidence or causal guardrails.
var customerInsightPreamble = "\n\n" + customerInsightMarker + `
The caller explicitly selected customer-insight research. Every rule above still applies unchanged: ground claims in cited source ids, keep counter-evidence and alternative explanations, and do not treat correlation or a single quote as causation.
Additionally treat the material as evidence about customers, users or buyers where it supports that reading.`

// customerInsightExtensions are appended per stage name. Stages without an
// entry receive only the shared preamble.
var customerInsightExtensions = map[string]string{
	"observation_extraction": `
Prefer observations of what people said, did, chose, avoided or worked around. Keep what they said and what they did as separate observations.`,
	"trace_detection": `
Behavioral traces include workarounds, repeated manual steps, abandoned flows, switching between alternatives and spending that contradicts stated preferences.`,
	"hypothesis_generation": `
For this profile the research concerns customer needs, so statedNeed and jtbd are relevant fields rather than empty legacy fields.
Fill statedNeed with what customers explicitly said they want, and latentNeed with the underlying need the evidence suggests; latentNeed must not restate statedNeed.
Fill jtbd as a Jobs To Be Done statement ("When <situation>, I want to <motivation>, so I can <outcome>") only when the evidence supports it.
A stated need is not proof of a latent need: keep at least one alternative explanation that does not rely on an unmet need.`,
	"insight_writeup": `
For this profile only, the earlier rule that productOpportunity and monetizationAngle must be empty strings is lifted: they are optional commercial projections. Fill them only when the evidence directly supports them, write them as untested ideas, and never let them change the interpretation, readiness or promotion of the insight.`,
}

// applyProfile returns step with the selected profile's extension appended.
// GENERAL_RESEARCH (and the empty default) returns step unchanged so its
// prompt and fingerprint stay identical to the domain-neutral baseline.
func applyProfile(step llmStep, profile domain.ReasoningProfile) llmStep {
	if profile.Normalize() != domain.ReasoningProfileCustomerInsight {
		return step
	}
	step.SystemPrompt += customerInsightPreamble + customerInsightExtensions[step.Name]
	return step
}

// pipelineLLMStepsFor returns the same stages as pipelineLLMSteps with the
// profile applied. Profiles never add, remove or reorder stages.
func pipelineLLMStepsFor(profile domain.ReasoningProfile) []llmStep {
	steps := pipelineLLMSteps()
	for i := range steps {
		steps[i] = applyProfile(steps[i], profile)
	}
	return steps
}

// projectWriteup enforces that commercial projections exist only under
// CUSTOMER_INSIGHT, whatever the model returned.
func projectWriteup(w insightWriteup, profile domain.ReasoningProfile) insightWriteup {
	if profile.Normalize() != domain.ReasoningProfileCustomerInsight {
		w.ProductOpportunity, w.MonetizationAngle = "", ""
	}
	return w
}
