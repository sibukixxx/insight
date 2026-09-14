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
