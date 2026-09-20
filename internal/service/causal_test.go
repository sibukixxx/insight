package service

import (
	"encoding/json"
	"os"
	"testing"

	"insight-lab/internal/domain"
)

func TestMunicipalPolicyFixtureRejectsNaiveCausalConclusion(t *testing.T) {
	raw, err := os.ReadFile("testdata/municipal_policy_causal.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		CompetingHypotheses    []string `json:"competingHypotheses"`
		MissingEvidence        []string `json:"missingEvidence"`
		ExpectedIdentification string   `json:"expectedIdentification"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	_, _, identification := AssessCausalReadiness(CausalAssessmentInput{
		SupportingCount: 1, MissingEvidence: fixture.MissingEvidence,
	})
	if len(fixture.CompetingHypotheses) < 2 || string(identification) != fixture.ExpectedIdentification {
		t.Fatalf("fixture must preserve alternatives and reject identification: %+v, %s", fixture, identification)
	}
}

func TestAssociationCannotBecomeCausallySupported(t *testing.T) {
	causal, validation, identification := AssessCausalReadiness(CausalAssessmentInput{SupportingCount: 3})
	if causal == domain.CausallySupported {
		t.Fatal("grounded association must not become CAUSALLY_SUPPORTED")
	}
	if causal != domain.CausalHypothesis || validation != domain.ValidationPlausible || identification != domain.IdentificationNotIdentified {
		t.Fatalf("unexpected assessment: %s, %s, %s", causal, validation, identification)
	}
}

func TestMissingEvidenceRemainsNotIdentified(t *testing.T) {
	_, validation, identification := AssessCausalReadiness(CausalAssessmentInput{
		MissingEvidence: []string{"population", "macroeconomic trend"},
	})
	if validation != domain.ValidationInsufficientEvidence || identification != domain.IdentificationNotIdentified {
		t.Fatalf("missing evidence should remain insufficient/not identified: %s, %s", validation, identification)
	}
}

func TestCounterEvidenceIsNotFalsificationCriterion(t *testing.T) {
	_, validation, _ := AssessCausalReadiness(CausalAssessmentInput{
		SupportingCount: 1, CounterCount: 1,
		FalsificationCriteria: []string{"a control region rises by the same amount"},
	})
	if validation != domain.ValidationContradicted {
		t.Fatalf("actual counter evidence must drive validation independently of criteria: %s", validation)
	}
}

func TestColliderIsNeverAutomaticControl(t *testing.T) {
	controls := CandidateControlVariables(domain.CandidateCausalStructure{Variables: []domain.CausalVariable{
		{ID: "c", Name: "selection", Role: domain.RoleCollider, Status: domain.ProposalSupported},
		{ID: "u", Name: "population", Role: domain.RoleConfounder, Status: domain.ProposalProposed},
		{ID: "s", Name: "baseline", Role: domain.RoleConfounder, Status: domain.ProposalSupported},
	}})
	if len(controls) != 1 || controls[0].ID != "s" {
		t.Fatalf("only supported confounders may be control candidates: %#v", controls)
	}
}

func TestConfidenceIsIndependentFromCausalStatus(t *testing.T) {
	high := Confidence(ConfidenceInput{
		SupportingEvidence:    []*domain.Evidence{{RelevanceScore: 1}},
		SupportingSourceTypes: []domain.SourceType{domain.SourceInterview},
		DocumentsWithSupport:  1, TotalDocuments: 1, PatternDocumentCount: 5,
	})
	causal, _, _ := AssessCausalReadiness(CausalAssessmentInput{SupportingCount: 1})
	if high <= 0 || causal == domain.CausallySupported {
		t.Fatalf("quality score %.2f must not promote causal status %s", high, causal)
	}
}

func TestCompetingHypothesesRemainCandidates(t *testing.T) {
	in := CausalAssessmentInput{SupportingCount: 1, Alternatives: []domain.CompetingHypothesis{
		{Title: "population", Explanation: "population inflow may explain the rise"},
		{Title: "artifact", Explanation: "registration changes may explain the rise"},
	}}
	causal, _, identification := AssessCausalReadiness(in)
	if len(in.Alternatives) < 2 || causal != domain.CausalHypothesis || identification != domain.IdentificationNotIdentified {
		t.Fatal("competing explanations must remain unproven candidates")
	}
}

func TestExpectationBasisAcceptsTheFullProvenanceVocabulary(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  domain.ExpectationBasis
	}{
		{"legacy source backed is kept as-is", "SOURCE_BACKED", domain.ExpectationSourceBacked},
		{"legacy model proposed is kept as-is", "MODEL_PROPOSED", domain.ExpectationModelProposed},
		{"prior is accepted", "PRIOR", domain.ExpectationPrior},
		{"literature is accepted", "LITERATURE", domain.ExpectationLiterature},
		{"domain knowledge is accepted", "DOMAIN_KNOWLEDGE", domain.ExpectationDomainKnowledge},
		{"model proposed post hoc is accepted", "MODEL_PROPOSED_POST_HOC", domain.ExpectationModelProposedPostHoc},
		{"human post hoc is accepted", "HUMAN_POST_HOC", domain.ExpectationHumanPostHoc},
		{"derived from prior run is accepted", "DERIVED_FROM_PRIOR_RUN", domain.ExpectationDerivedFromPriorRun},
		{"other is accepted", "OTHER", domain.ExpectationOther},
		{"an unrecognized value falls back to unknown", "not-a-real-value", domain.ExpectationUnknown},
		{"an empty value falls back to unknown", "", domain.ExpectationUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expectationBasis(tt.input); got != tt.want {
				t.Errorf("expectationBasis(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestInsightFindingKindNeverPromotesAPostHocExpectationToAValidationResult(t *testing.T) {
	tests := []struct {
		name  string
		stage domain.ResearchStage
		basis domain.ExpectationBasis
		want  domain.FindingKind
	}{
		{"exploratory stage is always an exploratory finding", domain.StageExploratory, domain.ExpectationPrior, domain.FindingExploratory},
		{"validation stage with pre-observation provenance is a validation result", domain.StageValidation, domain.ExpectationPrior, domain.FindingValidationResult},
		{"validation stage with model-proposed post-hoc provenance stays exploratory", domain.StageValidation, domain.ExpectationModelProposedPostHoc, domain.FindingExploratory},
		{"validation stage with legacy model-proposed provenance stays exploratory", domain.StageValidation, domain.ExpectationModelProposed, domain.FindingExploratory},
		{"validation stage with unknown provenance stays exploratory", domain.StageValidation, domain.ExpectationUnknown, domain.FindingExploratory},
		{"synthesis stage is unaffected by provenance", domain.StageSynthesis, domain.ExpectationModelProposedPostHoc, domain.FindingSynthesis},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InsightFindingKind(tt.stage, tt.basis); got != tt.want {
				t.Errorf("InsightFindingKind(%s, %s) = %s, want %s", tt.stage, tt.basis, got, tt.want)
			}
		})
	}
}

func TestExpandCompetingHypothesesCreatesIndependentCandidates(t *testing.T) {
	input := []hypothesisCandidate{{
		Title: "policy", LatentNeed: "policy effect", SurprisingFact: "designations rose",
		CandidateCausalStructure: domain.CandidateCausalStructure{Variables: []domain.CausalVariable{{ID: "x"}}},
		MissingEvidence:          []string{"policy timing"},
		AlternativeExplanations: []domain.CompetingHypothesis{
			{Title: "population", Explanation: "population inflow", MissingEvidence: []string{"migration data"}},
			{Title: "artifact", Explanation: "registration change", RequiredData: []string{"schema history"}},
		},
	}}

	got := expandCompetingHypotheses(input)
	if len(got) != 3 || got[0].HypothesisRole != domain.HypothesisPrimary || got[1].HypothesisRole != domain.HypothesisCompeting {
		t.Fatalf("unexpected expansion: %+v", got)
	}
	if got[0].HypothesisSetID == "" || got[0].HypothesisSetID != got[1].HypothesisSetID || got[0].HypothesisSetSize != 3 {
		t.Fatalf("candidates must share a three-item set: %+v", got)
	}
	if len(got[1].CandidateCausalStructure.Variables) != 0 || len(got[1].MissingEvidence) != 1 || got[1].MissingEvidence[0] != "migration data" {
		t.Fatalf("alternative must not inherit primary-specific causal claims: %+v", got[1])
	}
}
