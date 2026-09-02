// Quality gate for insights.
//
// An insight is "an unconscious desire that moves people". Two kinds of
// output routinely get mislabeled as insights: needs the customer already
// says out loud ("コスパ", "安心") and abstract labels that explain
// everything and therefore nothing ("自分らしさ", "承認欲求"). Neither can be
// caught by asking the model whether its own output is good. The checks
// here are deterministic, run on the app side after the model has spoken,
// and produce warnings for the human reader - they never silently drop
// an insight, because the final judgment belongs to the researcher.
package service

import (
	"strings"

	"insight-lab/internal/domain"
)

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
	// SupportingFirsthand / SupportingTotal count supporting evidence rows
	// by provenance of their source document.
	SupportingFirsthand int
	SupportingTotal     int
}

// AssessQuality returns the quality warnings for one insight. The order
// is stable (echo, generic, no trace, incomplete abduction, secondhand
// only) so the UI and tests can rely on it.
func AssessQuality(in QualityInput) []domain.QualityFlag {
	var flags []domain.QualityFlag

	if isStatedNeedEcho(in.StatedNeed, in.LatentNeed) {
		flags = append(flags, domain.QualityFlag{Code: domain.QualityStatedNeedEcho})
	}
	if term := firstGenericTerm(in.LatentNeed); term != "" {
		flags = append(flags, domain.QualityFlag{Code: domain.QualityGenericTerm, Detail: term})
	}
	if !citesTrace(in.Patterns) {
		flags = append(flags, domain.QualityFlag{Code: domain.QualityNoTrace})
	}
	if strings.TrimSpace(in.Expectation) == "" || strings.TrimSpace(in.SurprisingFact) == "" {
		flags = append(flags, domain.QualityFlag{Code: domain.QualityAbductionIncomplete})
	}
	if in.SupportingTotal > 0 && in.SupportingFirsthand == 0 {
		flags = append(flags, domain.QualityFlag{Code: domain.QualitySecondhandOnly})
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
