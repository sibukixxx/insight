package service

import (
	"testing"

	"insight-lab/internal/domain"
)

func codes(flags []domain.QualityFlag) []domain.QualityFlagCode {
	out := make([]domain.QualityFlagCode, 0, len(flags))
	for _, f := range flags {
		out = append(out, f.Code)
	}
	return out
}

func hasCode(flags []domain.QualityFlag, code domain.QualityFlagCode) bool {
	for _, f := range flags {
		if f.Code == code {
			return true
		}
	}
	return false
}

func trace() *domain.Pattern {
	return &domain.Pattern{ID: "pat_t", Kind: domain.PatternDeviation, Title: "忙しいのに時間をかける"}
}

func repetition() *domain.Pattern {
	return &domain.Pattern{ID: "pat_r", Kind: domain.PatternRepetition, Title: "確認行動"}
}

func TestAssessQualityCleanInsightHasNoFlags(t *testing.T) {
	flags := AssessQuality(QualityInput{
		StatedNeed:     "請求書作成を早く終わらせたい",
		LatentNeed:     "多く請求して顧客に『この会社大丈夫か』と思われる事態を絶対に避けたい",
		Expectation:    "忙しいなら自動計算を信じてそのまま送るはず",
		SurprisingFact: "半日かかると言いながら毎回電卓で検算している",
		Patterns:       []*domain.Pattern{trace(), repetition()},
	})
	if len(flags) != 0 {
		t.Fatalf("expected no flags, got %v", codes(flags))
	}
}

func TestAssessQualityStatedNeedEcho(t *testing.T) {
	cases := []struct {
		name, stated, latent string
		want                 bool
	}{
		{"identical", "確認作業を減らしたい", "確認作業を減らしたい", true},
		{"punctuation and width only", "確認作業を減らしたい。", "確認 作業を減らしたい", true},
		{"latent contains stated", "確認作業を減らしたい", "月末の確認作業を減らしたいと感じている", true},
		{"reworded", "確認作業を減らしたい", "確認作業の負担を減らしたい", true},
		{"different need", "作業を早く終わらせたい", "失敗して信頼を失うことを避けたい", false},
		{"stated need empty", "", "失敗して信頼を失うことを避けたい", false},
		{"stated need too short to compare", "楽", "楽に安心して提出したい", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isStatedNeedEcho(tc.stated, tc.latent)
			if got != tc.want {
				t.Errorf("isStatedNeedEcho(%q, %q) = %v, want %v (similarity %.2f)",
					tc.stated, tc.latent, got, tc.want,
					bigramSimilarity(normalizeForCompare(tc.stated), normalizeForCompare(tc.latent)))
			}
		})
	}
}

func TestAssessQualityGenericTerm(t *testing.T) {
	cases := []struct {
		latent string
		want   string
	}{
		{"承認欲求を満たしたい", "承認欲求"},
		{"自分らしさを表現したい", "自分らしさ"},
		{"コスパの良いものを選びたい", "コスパ"},
		{"コストパフォーマンスを重視している", "コストパフォーマンス"},
		{"とにかく安心したい", "安心"},
		{"家族の食事を作ったのは自分だという感覚を手放したくない", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := firstGenericTerm(tc.latent); got != tc.want {
			t.Errorf("firstGenericTerm(%q) = %q, want %q", tc.latent, got, tc.want)
		}
	}

	flags := AssessQuality(QualityInput{
		StatedNeed: "早く終わらせたい", LatentNeed: "承認欲求を満たしたい",
		Expectation: "x", SurprisingFact: "y", Patterns: []*domain.Pattern{trace()},
	})
	if len(flags) != 1 || flags[0].Code != domain.QualityGenericTerm || flags[0].Detail != "承認欲求" {
		t.Fatalf("flags = %+v, want a single generic_term flag with detail 承認欲求", flags)
	}
}

func TestAssessQualityNoTraceAndIncompleteAbduction(t *testing.T) {
	flags := AssessQuality(QualityInput{
		StatedNeed: "早く終わらせたい",
		LatentNeed: "失敗して信頼を失うことを避けたい",
		Patterns:   []*domain.Pattern{repetition()},
	})
	if !hasCode(flags, domain.QualityNoTrace) {
		t.Errorf("repetition-only hypothesis should be flagged no_trace: %v", codes(flags))
	}
	if !hasCode(flags, domain.QualityAbductionIncomplete) {
		t.Errorf("missing expectation/surprisingFact should be flagged abduction_incomplete: %v", codes(flags))
	}
	if hasCode(flags, domain.QualityStatedNeedEcho) || hasCode(flags, domain.QualityGenericTerm) {
		t.Errorf("unexpected content flags: %v", codes(flags))
	}

	// A hypothesis citing at least one deviation pattern clears no_trace,
	// even when repetition patterns are cited alongside it.
	flags = AssessQuality(QualityInput{
		StatedNeed: "早く終わらせたい", LatentNeed: "失敗して信頼を失うことを避けたい",
		Expectation: "x", SurprisingFact: "y",
		Patterns: []*domain.Pattern{repetition(), trace()},
	})
	if len(flags) != 0 {
		t.Errorf("expected no flags, got %v", codes(flags))
	}
}

func TestAssessQualityFlagOrderIsStable(t *testing.T) {
	flags := AssessQuality(QualityInput{
		StatedNeed: "安心したい", LatentNeed: "安心したい",
	})
	want := []domain.QualityFlagCode{
		domain.QualityStatedNeedEcho, domain.QualityGenericTerm, domain.QualityNoTrace, domain.QualityAbductionIncomplete,
	}
	got := codes(flags)
	if len(got) != len(want) {
		t.Fatalf("flags = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("flags = %v, want %v", got, want)
		}
	}
}
