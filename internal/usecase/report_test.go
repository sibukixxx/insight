package usecase

import (
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/service"
)

func TestRenderProjectMarkdownIncludesDecisionContextAndGroundedEvidence(t *testing.T) {
	documentID := "doc_1"
	report := ProjectReport{
		Project: &domain.Project{Name: "解約理由 *調査*"},
		Documents: map[string]*domain.Document{
			documentID: {ID: documentID, Title: "Interview 01"},
		},
		Insights: []*InsightDetail{{
			Insight: &domain.Insight{
				Title: "安心より可逆性", LatentNeed: "失敗しても戻せる状態を保ちたい",
				Expectation: "不満なら解約する", SurprisingFact: "不満なのに利用を続けた",
				Rationale: "履歴を失う方が怖いなら継続が自然になる", Confidence: .82,
				ProductOpportunity: "解約前のデータ保管プラン", MonetizationAngle: "低価格の休眠プラン",
				QualityFlags:     []domain.QualityFlag{{Code: domain.QualityNoTrace, Detail: "要確認"}},
				ExpectationBasis: domain.ExpectationModelProposed, CausalStatus: domain.CausalHypothesis,
				ValidationStatus: domain.ValidationPartiallySupported, IdentificationStatus: domain.IdentificationNotIdentified,
				MissingEvidence: []string{"対照地域"}, FalsificationCriteria: []string{"対照地域も同じ増加を示す"},
			},
			Evidence: []*domain.Evidence{{DocumentID: documentID, Type: domain.EvidenceSupport, Quote: "不満だけど、履歴が消えるのは困る"}},
		}},
		Metrics:     &service.Metrics{EvidenceCoverage: 1, TraceBackedInsightRate: .75, GroundedObservations: 3, TotalObservationCandidates: 4},
		GeneratedAt: time.Date(2026, 9, 2, 3, 4, 5, 0, time.UTC),
	}

	got := string(renderProjectMarkdown(report))
	for _, want := range []string{
		"# 解約理由 \\*調査\\* — Insight Report",
		"Trace-backed Insights: 75%",
		"**Product Opportunity:** 解約前のデータ保管プラン",
		"`no_trace`: 要確認",
		"**support / Interview 01**",
		"**Identification status:** NOT\\_IDENTIFIED",
		"not a probability that a causal claim is true",
		"> 不満だけど、履歴が消えるのは困る",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report does not contain %q:\n%s", want, got)
		}
	}
}

func TestRenderProjectMarkdownHandlesNoInsights(t *testing.T) {
	got := string(renderProjectMarkdown(ProjectReport{
		Project: &domain.Project{Name: "Empty"}, GeneratedAt: time.Unix(0, 0),
	}))
	if !strings.Contains(got, "No analyzed insights are available") {
		t.Fatalf("unexpected empty report: %s", got)
	}
}

func TestRenderProjectMarkdownComparesHypothesisSet(t *testing.T) {
	report := ProjectReport{
		Project: &domain.Project{Name: "Policy"},
		Insights: []*InsightDetail{
			{Insight: &domain.Insight{Title: "Policy effect", SurprisingFact: "designations rose", HypothesisSetID: "hset_1", HypothesisRole: domain.HypothesisPrimary, ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified, MissingEvidence: []string{"control regions"}}, Evidence: []*domain.Evidence{{Type: domain.EvidenceSupport}}},
			{Insight: &domain.Insight{Title: "Population inflow", SurprisingFact: "designations rose", HypothesisSetID: "hset_1", HypothesisRole: domain.HypothesisCompeting, ValidationStatus: domain.ValidationInsufficientEvidence, IdentificationStatus: domain.IdentificationNotIdentified}, Evidence: []*domain.Evidence{{Type: domain.EvidenceCounter}}},
			{Insight: &domain.Insight{Title: "Registration artifact", SurprisingFact: "designations rose", HypothesisSetID: "hset_1", HypothesisRole: domain.HypothesisCompeting, ValidationStatus: domain.ValidationUntested, IdentificationStatus: domain.IdentificationNotIdentified}},
		},
		GeneratedAt: time.Unix(0, 0),
	}

	got := string(renderProjectMarkdown(report))
	for _, want := range []string{"## Competing hypothesis comparison", "| PRIMARY | Policy effect", "| COMPETING | Population inflow", "| COMPETING | Registration artifact", "| 1 | 0 | 1 |"} {
		if !strings.Contains(got, want) {
			t.Errorf("comparison report does not contain %q:\n%s", want, got)
		}
	}
}

func TestRenderProjectMarkdownShowsRunProvenance(t *testing.T) {
	retrievedAt := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	report := ProjectReport{
		Project: &domain.Project{Name: "Dataset run"},
		Metrics: &service.Metrics{
			Provenance: service.RunProvenance{
				Mode: service.ExecutionModeModelBacked, Model: "gpt-x", PromptFingerprint: "abc123",
				RuleVersion: "dataset-preanalysis/v1", DatasetHashes: []string{"hash_a", "hash_b"},
				Datasets: []service.DatasetProvenance{{
					SourceName: "e-Stat", DatasetID: "000032143614", RetrievalMethod: service.RetrievalDownload,
					RetrievedAt: retrievedAt, SchemaID: "estat-economic-census-enterprise", SchemaVersion: "2021",
					RecipeRef: "recipes/estat-economic-census.md", FileHash: "hash_a",
				}},
				CompatibilityWarnings: []service.DatasetCompatibilityWarning{{
					Code: service.CompatibilityUnitMismatch, DatasetIDs: []string{"000032143614", "000032143615"},
					Detail: "unit \"enterprises\" differs from \"establishments\"",
				}},
				Notes: []string{"document doc_x: record_count \"many\" is not a number; skipped"},
			},
		},
		GeneratedAt: time.Unix(0, 0),
	}

	got := string(renderProjectMarkdown(report))
	for _, want := range []string{
		"## Run provenance",
		"model_backed", "gpt-x", "abc123", "dataset-preanalysis/v1",
		"hash_a", "hash\\_b",
		"e-Stat", "000032143614", "download", "estat-economic-census-enterprise", "2021", "recipes/estat-economic-census.md",
		"unit_mismatch", "unit \"enterprises\" differs from \"establishments\"",
		"document doc\\_x: record\\_count \"many\" is not a number; skipped",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report does not contain %q:\n%s", want, got)
		}
	}
}

func TestRenderProjectMarkdownOmitsRunProvenanceWhenAbsent(t *testing.T) {
	got := string(renderProjectMarkdown(ProjectReport{
		Project: &domain.Project{Name: "No metrics"}, GeneratedAt: time.Unix(0, 0),
	}))
	if strings.Contains(got, "## Run provenance") {
		t.Errorf("a report with no metrics must not claim any provenance: %s", got)
	}
}

func TestRenderProjectMarkdownShowsExpectationProvenanceAndObservationTiming(t *testing.T) {
	report := ProjectReport{
		Project: &domain.Project{Name: "Provenance"},
		Insights: []*InsightDetail{{Insight: &domain.Insight{
			Title: "x", LatentNeed: "y", ExpectationBasis: domain.ExpectationModelProposedPostHoc,
		}}},
		GeneratedAt: time.Unix(0, 0),
	}

	got := string(renderProjectMarkdown(report))
	for _, want := range []string{
		"**Expectation provenance:** MODEL\\_PROPOSED\\_POST\\_HOC",
		"**Observed relative to this data:** created after observing this dataset",
		"**Finding kind:** EXPLORATORY\\_FINDING",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report does not contain %q:\n%s", want, got)
		}
	}
}

func TestRenderProjectMarkdownShowsCurrentResearchStageAndAllowsAValidationResult(t *testing.T) {
	it := domain.ResearchIteration{ID: "it1", Sequence: 1, Stage: domain.StageValidation}
	report := ProjectReport{
		Project:     &domain.Project{Name: "Stage"},
		ResearchRun: &domain.ResearchRun{Question: "q", Iterations: []domain.ResearchIteration{it}},
		Insights: []*InsightDetail{{Insight: &domain.Insight{
			Title: "x", LatentNeed: "y", ExpectationBasis: domain.ExpectationPrior,
		}}},
		GeneratedAt: time.Unix(0, 0),
	}

	got := string(renderProjectMarkdown(report))
	for _, want := range []string{
		"## Research Stage",
		"**Current stage:** `VALIDATION`",
		"**Finding kind:** VALIDATION\\_RESULT",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report does not contain %q:\n%s", want, got)
		}
	}
}

func TestRenderProjectMarkdownNeverPromotesAPostHocInsightToAValidationResult(t *testing.T) {
	it := domain.ResearchIteration{ID: "it1", Sequence: 1, Stage: domain.StageValidation}
	report := ProjectReport{
		Project:     &domain.Project{Name: "Stage"},
		ResearchRun: &domain.ResearchRun{Question: "q", Iterations: []domain.ResearchIteration{it}},
		Insights: []*InsightDetail{{Insight: &domain.Insight{
			Title: "x", LatentNeed: "y", ExpectationBasis: domain.ExpectationModelProposed,
		}}},
		GeneratedAt: time.Unix(0, 0),
	}

	got := string(renderProjectMarkdown(report))
	if strings.Contains(got, "**Finding kind:** VALIDATION\\_RESULT") {
		t.Errorf("a post-hoc expectation must never become a validation result just because the run reached VALIDATION stage:\n%s", got)
	}
	if !strings.Contains(got, "**Finding kind:** EXPLORATORY\\_FINDING") {
		t.Errorf("expected an exploratory finding label:\n%s", got)
	}
}

func TestResearchReportShowsGapsRequirementsLimitsHistoryAndHumanEvaluation(t *testing.T) {
	it := domain.ResearchIteration{ID: "it1", Sequence: 1, InputReferences: []string{"synthetic.csv"}, ResearchGaps: []domain.ResearchGap{{Category: domain.ResearchGapComparison, Need: "comparison trend", WhyItMatters: "common trend remains possible"}}, DataRequirements: []domain.DataRequirement{{Need: "comparison outcomes", Reason: "test common trend", RequiredDimensions: []string{"group", "year"}, SuggestedSourceCategory: "comparison dataset"}}, WhatWeCannotConclude: []string{"causal treatment effect is not identified"}}
	report := ProjectReport{Project: &domain.Project{Name: "Policy"}, ResearchRun: &domain.ResearchRun{Question: "Did policy cause the increase?", Iterations: []domain.ResearchIteration{it}}, HumanEvaluations: map[string]*domain.HumanEvaluation{"it1": {Novelty: domain.NoveltyNew, OverallUsefulness: 4}}, GeneratedAt: time.Unix(0, 0)}
	got := string(renderProjectMarkdown(report))
	for _, want := range []string{"## Research Question", "## Research Gaps", "COMPARISON_CONTROL", "## Next Data Requirements", "## What We Cannot Conclude", "causal treatment effect is not identified", "## Iteration History", "## Human Evaluation", "novelty `NEW`"} {
		if !strings.Contains(got, want) {
			t.Errorf("research report missing %q:\n%s", want, got)
		}
	}
}

func TestResearchReportShowsDecisionReadinessAndStopReason(t *testing.T) {
	it := domain.ResearchIteration{
		ID: "it1", Sequence: 1,
		ResearchGaps: []domain.ResearchGap{{ID: "gap-1", Category: domain.ResearchGapComparison, Need: "comparison trend", Resolved: false}},
		Readiness: domain.ReadinessAssessment{
			State:            domain.ReadinessDecisionReadyWithLimitation,
			Reasons:          []string{"a single supported hypothesis survives"},
			UnresolvedGapIDs: []string{"gap-1"},
		},
		Stop:           &domain.StopDecision{Reason: domain.StopHypothesesDistinguished, Source: domain.StopSourceSystem, Note: "critical hypotheses are sufficiently distinguished", UnresolvedGapIDs: []string{"gap-1"}},
		HumanOverrides: []domain.HumanOverride{{Readiness: domain.ReadinessValidationRequired, Note: "reviewer disagreed with the system readiness"}},
	}
	report := ProjectReport{Project: &domain.Project{Name: "Policy"}, ResearchRun: &domain.ResearchRun{Question: "Did policy cause the increase?", Iterations: []domain.ResearchIteration{it}}, GeneratedAt: time.Unix(0, 0)}

	got := string(renderProjectMarkdown(report))
	for _, want := range []string{
		"## Decision Readiness",
		"DECISION_READY_WITH_LIMITATIONS",
		"a single supported hypothesis survives",
		"## Stop Decision",
		"CRITICAL_HYPOTHESES_DISTINGUISHED",
		"critical hypotheses are sufficiently distinguished",
		"gap-1",
		"## Human Overrides",
		"reviewer disagreed with the system readiness",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("research report missing %q:\n%s", want, got)
		}
	}
}

func TestResearchReportShowsPromotionStatusContributionAndBlockingReasons(t *testing.T) {
	it := domain.ResearchIteration{
		ID: "it1", Sequence: 1,
		PromotionGateInput: domain.PromotionGateInput{Contribution: domain.ContributionCorrection},
		Promotion: domain.PromotionAssessment{
			State:   domain.PromotionHumanReviewRequired,
			Reasons: []string{"promotion: human review has not been completed for this promoted output"},
		},
	}
	report := ProjectReport{Project: &domain.Project{Name: "Policy"}, ResearchRun: &domain.ResearchRun{Question: "Did policy cause the increase?", Iterations: []domain.ResearchIteration{it}}, GeneratedAt: time.Unix(0, 0)}

	got := string(renderProjectMarkdown(report))
	for _, want := range []string{
		"## Promotion Status",
		"**State:** `HUMAN_REVIEW_REQUIRED`",
		"**Contribution:** `CORRECTION`",
		"promotion: human review has not been completed for this promoted output",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("research report missing %q:\n%s", want, got)
		}
	}
}

func TestResearchReportListsUnmetPublicationChecklistItemsWhenPromotionIsBlocked(t *testing.T) {
	it := domain.ResearchIteration{
		ID: "it1", Sequence: 1,
		PromotionGateInput: domain.PromotionGateInput{
			Contribution: domain.ContributionCorrection,
			Checklist:    domain.PublicationChecklist{SourceProvenanceComplete: true},
		},
		Promotion: domain.PromotionAssessment{State: domain.PromotionHumanReviewRequired},
	}
	report := ProjectReport{Project: &domain.Project{Name: "Policy"}, ResearchRun: &domain.ResearchRun{Question: "q", Iterations: []domain.ResearchIteration{it}}, GeneratedAt: time.Unix(0, 0)}

	got := string(renderProjectMarkdown(report))
	for _, want := range []string{"Unmet publication checklist items", "deterministic\\_calculations\\_reproducible", "observation\\_grounded"} {
		if !strings.Contains(got, want) {
			t.Errorf("research report missing %q:\n%s", want, got)
		}
	}
}

func TestResearchReportOmitsStopSectionWhenResearchIsStillRunning(t *testing.T) {
	it := domain.ResearchIteration{ID: "it1", Sequence: 1, Readiness: domain.ReadinessAssessment{State: domain.ReadinessExploratoryOnly}}
	report := ProjectReport{Project: &domain.Project{Name: "Policy"}, ResearchRun: &domain.ResearchRun{Question: "q", Iterations: []domain.ResearchIteration{it}}, GeneratedAt: time.Unix(0, 0)}

	got := string(renderProjectMarkdown(report))
	if strings.Contains(got, "## Stop Decision") {
		t.Errorf("a run that has not stopped must not show a stop section:\n%s", got)
	}
	if !strings.Contains(got, "EXPLORATORY_ONLY") {
		t.Errorf("readiness must still be shown while research continues:\n%s", got)
	}
}
