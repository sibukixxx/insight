// Quality gate for insights.
//
// Quality checks are deterministic app-side guardrails after model output.
// The generic core checks whether a hypothesis is anchored to a mismatch and
// whether its abductive chain is inspectable. Legacy customer-research checks
// (stated-need echo / generic need terms) run only when the model actually
// supplied a statedNeed, so policy/science/operations hypotheses are not
// judged by a customer-needs vocabulary.
package service

import (
	"strings"

	"insight-lab/internal/domain"
)

// qualityRuleVersion is recorded in every run's execution snapshot. Bump it
// whenever the quality flag rules or thresholds change in a way that can change results.
const qualityRuleVersion = "quality/v3"

// genericNeedTerms are labels that, on their own, are not insights: the
// first group is what the customer already consciously wants (so it is a
// stated need, not a latent one); the second is abstractions so broad that
// satisfying them "moves" nobody in particular. Longer terms come first so
// the reported detail is the most specific match.
var genericNeedTerms = []string{
	// stated / conscious needs
	"コストパフォーマンス", "コスパ", "安心", "便利", "効率", "時短", "手軽", "楽をしたい", "楽したい", "お得",
	// abstractions
	"承認欲求", "自己実現", "自己肯定感", "自分らしさ", "帰属意識", "満足感",
}

// statedNeedEchoThreshold is the character-bigram Jaccard similarity above
// which latentNeed is considered a rewording of statedNeed.
const statedNeedEchoThreshold = 0.5

// minEchoRunes guards the containment check against trivially short
// stated needs ("楽") matching everything.
const minEchoRunes = 4

type QualityInput struct {
	StatedNeed     string
	LatentNeed     string
	Expectation    string
	SurprisingFact string
	// Patterns are the patterns the hypothesis cited (already filtered to
	// ones that exist).
	Patterns []*domain.Pattern
	// Profile gates customer-research checks; empty means GENERAL_RESEARCH.
	Profile domain.ReasoningProfile
}

// AssessQuality returns the quality warnings for one insight. The order
// is stable (echo, generic, no trace, incomplete abduction) so the UI and
// tests can rely on it.
func AssessQuality(in QualityInput) []domain.QualityFlag {
	var flags []domain.QualityFlag

	// These two checks belong to the CUSTOMER_INSIGHT profile. Generic
	// research must not be penalized by a customer-needs vocabulary.
	if in.Profile.Normalize() == domain.ReasoningProfileCustomerInsight && strings.TrimSpace(in.StatedNeed) != "" {
		if isStatedNeedEcho(in.StatedNeed, in.LatentNeed) {
			flags = append(flags, domain.QualityFlag{Code: domain.QualityStatedNeedEcho})
		}
		if term := firstGenericTerm(in.LatentNeed); term != "" {
			flags = append(flags, domain.QualityFlag{Code: domain.QualityGenericTerm, Detail: term})
		}
	}
	if !citesTrace(in.Patterns) {
		flags = append(flags, domain.QualityFlag{Code: domain.QualityNoTrace})
	}
	if strings.TrimSpace(in.Expectation) == "" || strings.TrimSpace(in.SurprisingFact) == "" {
		flags = append(flags, domain.QualityFlag{Code: domain.QualityAbductionIncomplete})
	}
	return flags
}

func citesTrace(patterns []*domain.Pattern) bool {
	for _, p := range patterns {
		if p != nil && p.IsTrace() {
			return true
		}
	}
	return false
}

func firstGenericTerm(latentNeed string) string {
	norm := normalizeForCompare(latentNeed)
	if len(norm) == 0 {
		return ""
	}
	s := string(norm)
	for _, term := range genericNeedTerms {
		if strings.Contains(s, string(normalizeForCompare(term))) {
			return term
		}
	}
	return ""
}

// isStatedNeedEcho decides whether latentNeed merely restates statedNeed.
// It uses the same normalization as grounding (whitespace and punctuation
// dropped, width folded) and then either substring containment or a
// character-bigram Jaccard overlap.
func isStatedNeedEcho(statedNeed, latentNeed string) bool {
	a := normalizeForCompare(statedNeed)
	b := normalizeForCompare(latentNeed)
	if len(a) < minEchoRunes || len(b) < minEchoRunes {
		return false
	}
	as, bs := string(a), string(b)
	if strings.Contains(as, bs) || strings.Contains(bs, as) {
		return true
	}
	return bigramSimilarity(a, b) >= statedNeedEchoThreshold
}

func normalizeForCompare(s string) []rune {
	var out []rune
	for _, r := range s {
		nr, keep := normalizeRune(r)
		if !keep {
			continue
		}
		out = append(out, nr)
	}
	return out
}

func bigrams(r []rune) map[[2]rune]struct{} {
	set := make(map[[2]rune]struct{}, len(r))
	for i := 0; i+1 < len(r); i++ {
		set[[2]rune{r[i], r[i+1]}] = struct{}{}
	}
	return set
}

// bigramSimilarity is the Jaccard index of the two strings' character
// bigram sets. Character bigrams work reasonably for Japanese without a
// tokenizer, which is the point: no external dependency, fully
// deterministic.
func bigramSimilarity(a, b []rune) float64 {
	sa, sb := bigrams(a), bigrams(b)
	if len(sa) == 0 || len(sb) == 0 {
		return 0
	}
	var inter int
	for k := range sa {
		if _, ok := sb[k]; ok {
			inter++
		}
	}
	union := len(sa) + len(sb) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
