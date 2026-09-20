package service

import "insight-lab/internal/domain"

// PromotionArtifactFacts are the structural facts about a research artifact
// that can be verified mechanically, from data the application already
// holds. They never encode a judgment call about whether the research is
// good; they only report whether a mechanically checkable box is filled in.
type PromotionArtifactFacts struct {
	Stage                            domain.ResearchStage
	ProvenanceMode                   string
	DatasetHashesPresent             bool
	GroundedObservations             int
	TotalObservationCandidates       int
	AllInsightsHaveExpectationBasis  bool
	AllInsightsHaveEvidence          bool
	CounterEvidenceSearched          bool
	AnyLimitationsDisclosed          bool
	ResearchGapsDisclosed            bool
	UnresolvableConclusionsDisclosed bool
	CompatibilityWarningsPresent     bool
}

// BuildPublicationChecklist derives the mechanically-verifiable items of a
// domain.PublicationChecklist from facts and carries forward the items that
// require a human judgment call (competing hypothesis consideration,
// independent validation accuracy, decision-readiness honesty) from human
// as-is. Those three are never inferred here: doing so would let the system
// approve its own quality, which issue #24 explicitly rules out.
func BuildPublicationChecklist(facts PromotionArtifactFacts, human domain.PublicationChecklist) domain.PublicationChecklist {
	return domain.PublicationChecklist{
		SourceProvenanceComplete:              facts.DatasetHashesPresent,
		DeterministicCalculationsReproducible: facts.ProvenanceMode == string(ExecutionModeDeterministic),
		ObservationGrounded:                   facts.TotalObservationCandidates > 0 && facts.GroundedObservations == facts.TotalObservationCandidates,
		ExpectationProvenanceVisible:          facts.AllInsightsHaveExpectationBasis,
		ResearchStageVisible:                  facts.Stage != "",
		ClaimEvidenceMappingComplete:          facts.AllInsightsHaveEvidence,
		CounterEvidenceSearched:               facts.CounterEvidenceSearched,
		LimitationsPresent:                    facts.AnyLimitationsDisclosed,
		ResearchGapsDisclosed:                 facts.ResearchGapsDisclosed,
		UnresolvableConclusionsDisclosed:      facts.UnresolvableConclusionsDisclosed,
		NoHiddenPopulationUnitPeriodMismatch:  !facts.CompatibilityWarningsPresent,

		CompetingHypothesisConsidered:       human.CompetingHypothesisConsidered,
		IndependentValidationStatusAccurate: human.IndependentValidationStatusAccurate,
		DecisionReadinessHonestlyStated:     human.DecisionReadinessHonestlyStated,
	}
}
