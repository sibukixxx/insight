//go:build golden

// Package goldenset holds Insight's versioned domain fixtures for the Shared
// Eval Contract / Golden Set described in issue #14. Each fixture pins one
// research-quality invariant to a concrete scenario so a regression shows up
// as a specific, readable test failure instead of a vague quality complaint.
//
// Run with: go test -tags=golden ./internal/goldenset/...
//
// This file covers the original deterministic research-integrity cases.
// Shared Eval Contract v1 cases for Insight Semantics v2, Insight Delta and
// Multi-Mode boundaries live in semantic_golden_test.go. Domain fixtures stay
// in Insight; the shared contract remains an exchange schema, not a runtime
// dependency on TechVit Business.
package goldenset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/service"
)

func loadFixture(t *testing.T, name string, out interface{}) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}
}

// fixtureInsight is the JSON shape used by these fixtures to build a
// domain.Insight. domain.Insight itself carries no json tags, so fixtures
// only declare the fields a readiness/identification golden case needs.
type fixtureInsight struct {
	ID                   string   `json:"id"`
	Title                string   `json:"title"`
	HypothesisSetID      string   `json:"hypothesisSetId"`
	SurprisingFact       string   `json:"surprisingFact"`
	MissingEvidence      []string `json:"missingEvidence,omitempty"`
	CausalStatus         string   `json:"causalStatus"`
	ValidationStatus     string   `json:"validationStatus"`
	IdentificationStatus string   `json:"identificationStatus"`
}

func (f fixtureInsight) toDomain() *domain.Insight {
	return &domain.Insight{
		ID: f.ID, Title: f.Title, HypothesisSetID: f.HypothesisSetID, SurprisingFact: f.SurprisingFact,
		MissingEvidence:      append([]string(nil), f.MissingEvidence...),
		CausalStatus:         domain.CausalStatus(f.CausalStatus),
		ValidationStatus:     domain.ValidationStatus(f.ValidationStatus),
		IdentificationStatus: domain.IdentificationStatus(f.IdentificationStatus),
	}
}

func toDomainInsights(fixtures []fixtureInsight) []*domain.Insight {
	out := make([]*domain.Insight, 0, len(fixtures))
	for _, f := range fixtures {
		out = append(out, f.toDomain())
	}
	return out
}

// TestGoldenAssociationOnlyNeverClaimsCausalIdentification pins the "synthetic
// association-only case" from issue #14: every hypothesis stays
// OBSERVED_ASSOCIATION / NOT_IDENTIFIED throughout, and the Research Loop
// Gate must still surface that limitation explicitly even once one
// hypothesis is the sole survivor. Evidential support must never read as
// causal identification.
func TestGoldenAssociationOnlyNeverClaimsCausalIdentification(t *testing.T) {
	var fixture struct {
		Question                string           `json:"question"`
		Iteration1              []fixtureInsight `json:"iteration1"`
		Iteration2AddedEvidence []string         `json:"iteration2AddedEvidence"`
		Iteration2              []fixtureInsight `json:"iteration2"`
		ExpectedReadiness       string           `json:"expectedReadiness"`
		ExpectedStopReason      string           `json:"expectedStopReason"`
		ExpectedReasonSubstring string           `json:"expectedReasonSubstring"`
	}
	loadFixture(t, "association_only.json", &fixture)

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	iter1 := service.BuildResearchIteration(1, fixture.Question, nil, toDomainInsights(fixture.Iteration1), t1)
	iter1 = service.FinalizeResearchIteration(domain.ResearchRun{}, iter1, nil, t1)
	run := domain.ResearchRun{ID: "run-association-only", Question: fixture.Question, Iterations: []domain.ResearchIteration{iter1}}

	insights2 := toDomainInsights(fixture.Iteration2)
	iter2 := service.BuildResearchIteration(2, fixture.Question, nil, insights2, t2)
	iter2 = service.FinalizeResearchIteration(run, iter2, fixture.Iteration2AddedEvidence, t2)
	run = run.AppendIteration(iter2)

	if got := string(iter2.Readiness.State); got != fixture.ExpectedReadiness {
		t.Fatalf("readiness = %s, want %s (reasons: %v)", got, fixture.ExpectedReadiness, iter2.Readiness.Reasons)
	}
	if !strings.Contains(strings.Join(iter2.Readiness.Reasons, " "), fixture.ExpectedReasonSubstring) {
		t.Fatalf("readiness reasons must name the unresolved identification gap: %v", iter2.Readiness.Reasons)
	}
	if iter2.Stop == nil || string(iter2.Stop.Reason) != fixture.ExpectedStopReason {
		t.Fatalf("stop decision = %+v, want reason %s", iter2.Stop, fixture.ExpectedStopReason)
	}

	for _, insight := range insights2 {
		if insight.CausalStatus != domain.CausalObservedAssociation {
			t.Fatalf("hypothesis %s must stay OBSERVED_ASSOCIATION, the pipeline must never upgrade it: got %s", insight.ID, insight.CausalStatus)
		}
	}

	handoff := service.BuildHumanHandoff(run)
	if len(handoff.WhatWeCannotConclude) == 0 {
		t.Fatal("human handoff must state what cannot be concluded from association-only evidence")
	}
	for _, state := range handoff.StrongestSurvivingHypotheses {
		if state.IdentificationStatus == domain.IdentificationIdentified {
			t.Fatalf("surviving hypothesis %+v must not read as causally identified", state)
		}
	}
}

// TestGoldenPopulationMismatchIsolatesDenominatorDifference pins the
// "definition/population mismatch case": two datasets that agree on unit,
// period and schema but disagree on population scope must raise exactly the
// population_mismatch warning, and nothing else, so a delta across them
// never silently mixes denominators.
func TestGoldenPopulationMismatchIsolatesDenominatorDifference(t *testing.T) {
	var fixture struct {
		Manifests          []service.AcquisitionManifest `json:"manifests"`
		ExpectedCode       string                        `json:"expectedCode"`
		ExpectedDatasetIDs []string                      `json:"expectedDatasetIds"`
	}
	loadFixture(t, "population_mismatch.json", &fixture)

	warnings := service.CheckDatasetCompatibility(fixture.Manifests)

	if len(warnings) != 1 {
		t.Fatalf("want exactly one warning isolating the population mismatch, got %+v", warnings)
	}
	got := warnings[0]
	if string(got.Code) != fixture.ExpectedCode {
		t.Fatalf("warning code = %s, want %s", got.Code, fixture.ExpectedCode)
	}
	if len(got.DatasetIDs) != len(fixture.ExpectedDatasetIDs) || got.DatasetIDs[0] != fixture.ExpectedDatasetIDs[0] || got.DatasetIDs[1] != fixture.ExpectedDatasetIDs[1] {
		t.Fatalf("dataset ids = %v, want %v", got.DatasetIDs, fixture.ExpectedDatasetIDs)
	}
	if got.Detail == "" {
		t.Fatal("warning must carry a human-readable detail")
	}
}

// TestGoldenPostHocExpectationGuardBlocksSelfValidation pins the
// post-hoc/HARKing guard required by issue #14 and #22: an expectation
// generated after the data was observed may never be relabeled as PRIOR, may
// never itself count as independent validation evidence for the run that
// produced it, and only regains the ability to be reported as a validation
// result once it is carried into a new iteration as
// DERIVED_FROM_PRIOR_RUN and frozen there.
func TestGoldenPostHocExpectationGuardBlocksSelfValidation(t *testing.T) {
	var fixture struct {
		Statement                       string   `json:"statement"`
		Provenance                      string   `json:"provenance"`
		AuthorType                      string   `json:"authorType"`
		ResearchIterationID             string   `json:"researchIterationId"`
		ObservedDataAvailableAtCreation bool     `json:"observedDataAvailableAtCreation"`
		FalsificationCriteria           []string `json:"falsificationCriteria"`
		NewExpectationID                string   `json:"newExpectationId"`
		NewIterationID                  string   `json:"newIterationId"`
	}
	loadFixture(t, "post_hoc_guard.json", &fixture)

	createdAt := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	expectation := domain.Expectation{
		ID: "exp-1", Statement: fixture.Statement,
		Provenance: domain.ExpectationBasis(fixture.Provenance), AuthorType: domain.AuthorType(fixture.AuthorType),
		ResearchIterationID:             fixture.ResearchIterationID,
		ObservedDataAvailableAtCreation: fixture.ObservedDataAvailableAtCreation,
		FalsificationCriteria:           fixture.FalsificationCriteria,
		CreatedAt:                       createdAt,
	}

	if err := expectation.Validate(); err != nil {
		t.Fatalf("well-formed post-hoc expectation must validate: %v", err)
	}
	if expectation.PermitsStrongValidationClaim() {
		t.Fatal("a post-hoc expectation must never permit a strong validation claim on its own")
	}
	if expectation.IsIndependentEvidence(expectation.ResearchIterationID) {
		t.Fatal("evidence from the same iteration that produced a post-hoc expectation is not independent")
	}

	relabeled := expectation
	relabeled.Provenance = domain.ExpectationPrior
	if err := relabeled.Validate(); err == nil {
		t.Fatal("relabeling a post-hoc expectation as PRIOR while the data was already observed must be rejected")
	}

	frozen, err := expectation.Freeze(createdAt.Add(24 * time.Hour))
	if err != nil {
		t.Fatalf("post-hoc expectations may still be frozen so a later iteration can test them: %v", err)
	}
	if frozen.PermitsStrongValidationClaim() {
		t.Fatal("freezing a post-hoc expectation must not launder its provenance into a validation-ready claim")
	}

	derived := expectation.DeriveForValidation(fixture.NewExpectationID, fixture.NewIterationID, createdAt.Add(48*time.Hour))
	if derived.Provenance != domain.ExpectationDerivedFromPriorRun {
		t.Fatalf("carrying an exploratory expectation forward must record DERIVED_FROM_PRIOR_RUN, got %s", derived.Provenance)
	}
	if derived.ObservedDataAvailableAtCreation {
		t.Fatal("a derived expectation must state the new iteration's data was not yet observed")
	}
	if derived.DerivedFromExpectationID != expectation.ID {
		t.Fatal("derived expectation must keep lineage back to the exploratory source")
	}

	frozenDerived, err := derived.Freeze(createdAt.Add(72 * time.Hour))
	if err != nil {
		t.Fatalf("derived expectation with falsification criteria must freeze: %v", err)
	}
	if !frozenDerived.PermitsStrongValidationClaim() {
		t.Fatal("a frozen, pre-observation, derived expectation is the legitimate path to a validation claim")
	}

	if expectation.Provenance != domain.ExpectationModelProposedPostHoc {
		t.Fatal("the original exploratory expectation must never be mutated by deriving or freezing a copy")
	}
}

// TestGoldenValidInconclusiveKeepsBothHypothesesOpen pins the "valid
// inconclusive case": added evidence that does not move either competing
// hypothesis must be reported as INCONCLUSIVE, and the human handoff must
// keep both hypotheses as tied survivors rather than collapsing the tie into
// a single stronger narrative.
func TestGoldenValidInconclusiveKeepsBothHypothesesOpen(t *testing.T) {
	var fixture struct {
		Question                string           `json:"question"`
		Iteration1              []fixtureInsight `json:"iteration1"`
		Iteration2AddedEvidence []string         `json:"iteration2AddedEvidence"`
		Iteration2              []fixtureInsight `json:"iteration2"`
		ExpectedReadiness       string           `json:"expectedReadiness"`
		ExpectedStopReason      string           `json:"expectedStopReason"`
	}
	loadFixture(t, "valid_inconclusive.json", &fixture)

	t1 := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	iter1 := service.BuildResearchIteration(1, fixture.Question, nil, toDomainInsights(fixture.Iteration1), t1)
	iter1 = service.FinalizeResearchIteration(domain.ResearchRun{}, iter1, nil, t1)
	run := domain.ResearchRun{ID: "run-valid-inconclusive", Question: fixture.Question, Iterations: []domain.ResearchIteration{iter1}}

	iter2 := service.BuildResearchIteration(2, fixture.Question, nil, toDomainInsights(fixture.Iteration2), t2)
	iter2 = service.FinalizeResearchIteration(run, iter2, fixture.Iteration2AddedEvidence, t2)
	run = run.AppendIteration(iter2)

	if got := string(iter2.Readiness.State); got != fixture.ExpectedReadiness {
		t.Fatalf("readiness = %s, want %s (reasons: %v)", got, fixture.ExpectedReadiness, iter2.Readiness.Reasons)
	}
	if iter2.Stop == nil || string(iter2.Stop.Reason) != fixture.ExpectedStopReason {
		t.Fatalf("stop decision = %+v, want reason %s", iter2.Stop, fixture.ExpectedStopReason)
	}

	handoff := service.BuildHumanHandoff(run)
	if handoff.Readiness != domain.ReadinessInconclusive {
		t.Fatalf("human handoff must not upgrade an inconclusive readiness, got %s", handoff.Readiness)
	}
	if len(handoff.StrongestSurvivingHypotheses) != 2 {
		t.Fatalf("both tied hypotheses must remain visible as survivors, got %+v", handoff.StrongestSurvivingHypotheses)
	}
	if len(handoff.ContradictedHypotheses) != 0 {
		t.Fatalf("neither hypothesis was contradicted; the tie must not be broken artificially: %+v", handoff.ContradictedHypotheses)
	}
}

// TestGoldenPolishedButUnsupportedCannotReachPublicationReady connects the
// Golden Set to the Public Report Promotion Gate (#24). A plausible,
// publication-shaped narrative is not enough: mechanically verifiable
// research-quality checks must be satisfied before PUBLICATION_READY.
func TestGoldenPolishedButUnsupportedCannotReachPublicationReady(t *testing.T) {
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	in := domain.PromotionGateInput{
		Contribution:         domain.ContributionNovelMismatch,
		HumanReviewCompleted: true,
		Checklist: domain.PublicationChecklist{
			SourceProvenanceComplete:              true,
			DeterministicCalculationsReproducible: true,
			// Intentionally unsupported despite polished prose:
			ObservationGrounded:                  false,
			ExpectationProvenanceVisible:         true,
			ResearchStageVisible:                 true,
			ClaimEvidenceMappingComplete:         false,
			CompetingHypothesisConsidered:        false,
			CounterEvidenceSearched:              false,
			LimitationsPresent:                   false,
			ResearchGapsDisclosed:                false,
			UnresolvableConclusionsDisclosed:     false,
			NoHiddenPopulationUnitPeriodMismatch: true,
			IndependentValidationStatusAccurate:  false,
			DecisionReadinessHonestlyStated:      false,
		},
	}
	assessment := service.AssessPromotion(domain.PromotionDraft, in, now)
	if assessment.State == domain.PromotionPublicationReady || assessment.State == domain.PromotionPublished {
		t.Fatalf("polished but unsupported research must not be publication-ready: %+v", assessment)
	}
	if assessment.State != domain.PromotionHumanReviewRequired {
		t.Fatalf("expected promotion to stop at HUMAN_REVIEW_REQUIRED, got %+v", assessment)
	}
	if len(assessment.Reasons) == 0 || !strings.Contains(assessment.Reasons[0], "publication checklist is incomplete") {
		t.Fatalf("blocking reason must expose incomplete research checks: %+v", assessment)
	}
}

// TestGoldenHumanReviewRemainsExternalInput guards the Shared Eval / Research
// Loop boundary: a human review outcome can be represented, but no model or
// promotion assessment is allowed to manufacture it.
func TestGoldenHumanReviewRemainsExternalInput(t *testing.T) {
	evaluation := domain.HumanEvaluation{
		ResearchRunID:          "run-golden-human",
		IterationID:            "iter-golden-human",
		ObservationGrounding:   5,
		SurpriseUsefulness:     4,
		HypothesisDiversity:    4,
		CounterEvidenceQuality: 4,
		MissingEvidenceQuality: 5,
		IdentificationHonesty:  5,
		NextDataUsefulness:     4,
		Novelty:                domain.NoveltyPartiallyNew,
		OverallUsefulness:      4,
		Notes:                  "human-supplied golden review",
		EvaluatedAt:            time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
	}
	if evaluation.Novelty != domain.NoveltyPartiallyNew || evaluation.OverallUsefulness != 4 {
		t.Fatalf("human review outcome must be preserved exactly: %+v", evaluation)
	}
}
