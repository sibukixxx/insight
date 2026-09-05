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
				QualityFlags: []domain.QualityFlag{{Code: domain.QualityNoTrace, Detail: "要確認"}},
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
