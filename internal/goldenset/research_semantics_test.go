//go:build golden

package goldenset

import (
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/service"
)

func TestGoldenNonObviousConnectionDoesNotSelfValidate(t *testing.T) {
	insight := &domain.Insight{
		ID: "h-connection",
		Title: "Cross-context scarce information",
		Connections: []domain.InsightConnection{{
			Kind: domain.ConnectionInteraction,
			Statement: "first-hand domain experience becomes scarce information when connected to a professional audience",
			From: []domain.ConnectionReference{{Kind: domain.ConnectionRefContext, ID: "ctx-a", Label: "first-hand domain experience"}},
			To: []domain.ConnectionReference{{Kind: domain.ConnectionRefContext, ID: "ctx-b", Label: "professional information demand"}},
		}},
		Mechanisms: []domain.MechanismCandidate{{
			Statement: "scarcity of first-hand detail may increase attention and trust",
			BridgeAssumptions: []string{"the audience lacks equivalent first-hand information"},
			UnresolvedGaps: []string{"whether the connection generalizes beyond this audience"},
		}},
		CausalStatus: domain.CausalHypothesis,
		ValidationStatus: domain.ValidationUntested,
		IdentificationStatus: domain.IdentificationNotIdentified,
	}

	iteration := service.BuildResearchIteration(1, "why did the unusual audience respond?", nil, []*domain.Insight{insight}, time.Now())
	if len(iteration.InsightIDs) != 1 {
		t.Fatalf("connection-bearing insight must remain available to the research loop: %+v", iteration)
	}
	if insight.CausalStatus != domain.CausalHypothesis || insight.IdentificationStatus != domain.IdentificationNotIdentified {
		t.Fatalf("narrative/mechanism coherence must never promote causal status: %+v", insight)
	}
}

func TestGoldenGeneralizationRequiresHumanReview(t *testing.T) {
	candidate := domain.GeneralizationCandidate{
		Principle: "cross-domain first-hand experience can create scarce professional information",
		Status: domain.GeneralizationStatusSupported,
		HumanReviewed: false,
	}
	if candidate.PermitsTransferClaim() {
		t.Fatal("a model-supplied generalization must not self-certify transferability")
	}
	candidate.HumanReviewed = true
	if !candidate.PermitsTransferClaim() {
		t.Fatal("a supported generalization may be presented as transferable only after human review")
	}
}

func TestGoldenValidationEvidenceRejectsSamePassPostHocEvidence(t *testing.T) {
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	e := domain.Expectation{
		ID: "exp-posthoc", Statement: "housing affordability explains the observed change",
		Provenance: domain.ExpectationModelProposedPostHoc, AuthorType: domain.AuthorModel,
		ResearchIterationID: "it-1", ObservedDataAvailableAtCreation: true,
		FalsificationCriteria: []string{"the same pattern is absent in an independent comparison"},
		CreatedAt: now,
	}
	record := domain.ValidationEvidenceProvenance{
		ExpectationID: e.ID, EvidenceReference: "same-pass.csv", EvidenceIterationID: "it-1",
		Relation: domain.ValidationEvidencePreObservationSameIteration,
		Rationale: "attempted reuse of the same observed data", RecordedBy: domain.AuthorHuman, RecordedAt: now,
	}
	if err := record.ValidateAgainst(e); err == nil {
		t.Fatal("same-iteration post-hoc evidence must not count as independent validation evidence")
	}
}

func TestGoldenGapResolutionRequiresExactEvidenceLink(t *testing.T) {
	prior := domain.ResearchIteration{ID: "it-1", ResearchGaps: []domain.ResearchGap{
		service.ResearchGapFromMissingEvidence("gap-housing", domain.ResearchGapConfounder, "household income", "distinguish affordability explanations", []string{"h1", "h2"}),
		service.ResearchGapFromMissingEvidence("gap-destination", domain.ResearchGapComparison, "destination municipality", "distinguish local from regional movement", []string{"h1", "h2"}),
	}}
	current := domain.ResearchIteration{ID: "it-2"}
	got := service.CarryForwardResearchGapsWithLinks(prior, current, []domain.EvidenceAddition{{
		Reference: "income.csv", GapIDs: []string{"gap-housing"}, AddedAt: time.Now(),
	}})
	if len(got.ResearchGaps) != 2 {
		t.Fatalf("both prior gaps must remain auditable: %+v", got.ResearchGaps)
	}
	status := map[string]bool{}
	for _, gap := range got.ResearchGaps { status[gap.ID] = gap.Resolved }
	if !status["gap-housing"] || status["gap-destination"] {
		t.Fatalf("only the gap named by EvidenceAddition may resolve: %+v", status)
	}
}

func TestGoldenInsightDeltaTracksInputAndInterpretationChangeWithoutCausalAttribution(t *testing.T) {
	previous := domain.ResearchIteration{
		ID: "it-1",
		InputSnapshot: domain.ResearchInputSnapshot{Variables: []string{"population"}, EvidenceReferences: []string{"population.csv"}},
		InsightIDs: []string{"h1"},
		HypothesisStates: []domain.HypothesisState{{
			HypothesisID: "h1", ComparisonKey: "housing explanation",
			ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified,
		}},
	}
	current := domain.ResearchIteration{
		ID: "it-2",
		InputSnapshot: domain.ResearchInputSnapshot{Variables: []string{"population", "housing_price"}, EvidenceReferences: []string{"population.csv", "housing.csv"}},
		InsightIDs: []string{"h1b", "h2"},
		HypothesisStates: []domain.HypothesisState{
			{HypothesisID: "h1b", ComparisonKey: "housing explanation", ValidationStatus: domain.ValidationInsufficientEvidence, IdentificationStatus: domain.IdentificationNotIdentified},
			{HypothesisID: "h2", ComparisonKey: "income interaction", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified},
		},
	}
	current.HypothesisChanges = service.CompareHypothesisStates(previous.HypothesisStates, current.HypothesisStates)
	delta := service.CompareResearchIterations(previous, current)

	var sawVariable, sawEvidence bool
	for _, change := range delta.InputChanges {
		if change.Category == "variable" && change.Kind == domain.DeltaAdded && change.Value == "housing_price" { sawVariable = true }
		if change.Category == "evidence" && change.Kind == domain.DeltaAdded && change.Value == "housing.csv" { sawEvidence = true }
	}
	if !sawVariable || !sawEvidence {
		t.Fatalf("delta must expose the exact input additions: %+v", delta.InputChanges)
	}
	if len(delta.Result.HypothesisChanges) == 0 {
		t.Fatal("delta must retain hypothesis evolution")
	}
	for _, state := range current.HypothesisStates {
		if state.IdentificationStatus != domain.IdentificationNotIdentified {
			t.Fatalf("input/result succession must not be converted into causal identification: %+v", state)
		}
	}
}

func TestGoldenInsightDeltaAllowsNoChange(t *testing.T) {
	snapshot := domain.ResearchInputSnapshot{Variables: []string{"population"}, EvidenceReferences: []string{"population.csv"}}
	previous := domain.ResearchIteration{ID: "it-1", InputSnapshot: snapshot, InsightIDs: []string{"h1"}}
	current := domain.ResearchIteration{ID: "it-2", InputSnapshot: snapshot, InsightIDs: []string{"h1"}}
	delta := service.CompareResearchIterations(previous, current)
	if len(delta.InputChanges) != 0 || len(delta.Result.InsightAdded) != 0 || len(delta.Result.InsightRemoved) != 0 {
		t.Fatalf("the system must allow evidence to leave the interpretation unchanged: %+v", delta)
	}
}

func TestGoldenSemanticAnalysisModesStayOrthogonalToResearchStage(t *testing.T) {
	mode, err := domain.ResolveAnalysisMode("", domain.ArtifactClassStructuredDataset)
	if err != nil || mode != domain.AnalysisModeDatasetAnalysis {
		t.Fatalf("structured datasets must select DATASET_ANALYSIS: mode=%s err=%v", mode, err)
	}
	mode, err = domain.ResolveAnalysisMode("", domain.ArtifactClassResearchArtifact)
	if err != nil || mode != domain.AnalysisModeResearchReview {
		t.Fatalf("external research artifacts must select RESEARCH_REVIEW: mode=%s err=%v", mode, err)
	}
	if _, err := domain.ResolveAnalysisMode("", domain.ArtifactClassMixed); err == nil {
		t.Fatal("mixed semantic inputs require an explicit mode rather than silent classification")
	}
	iteration := service.BuildResearchIterationWithMode(1, "review this report", nil, domain.ResearchInputSnapshot{}, domain.AnalysisModeResearchReview, domain.ArtifactClassResearchArtifact, []domain.Claim{{
		ID: "claim-1", Statement: "the intervention caused the increase", Kind: domain.ClaimInterpretation, SourceReference: "external-report.pdf",
	}}, nil, time.Now())
	if iteration.Stage != domain.StageExploratory || iteration.AnalysisMode != domain.AnalysisModeResearchReview {
		t.Fatalf("analysis mode and research stage are orthogonal: %+v", iteration)
	}
	if iteration.Claims[0].HasUnderlyingEvidence() {
		t.Fatal("an imported claim without underlying evidence must not become primary evidence")
	}
}

func TestGoldenPromotionGateRejectsPolishedButUnsupportedOutput(t *testing.T) {
	state := domain.PromotionHumanReviewRequired
	input := domain.PromotionGateInput{
		Contribution: domain.ContributionNovelMismatch,
		HumanReviewCompleted: true,
		Checklist: domain.PublicationChecklist{
			SourceProvenanceComplete: true,
			ResearchStageVisible: true,
		},
	}
	if err := state.Transition(domain.PromotionPublicationReady, input); err == nil {
		t.Fatal("a polished narrative with incomplete evidence/checklist must not become PUBLICATION_READY")
	}
}
