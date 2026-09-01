// Grounding verifies that a quote an LLM returned actually appears in the
// source document it was supposedly drawn from. This is the structural
// defense against a hallucinated customer quote: an LLM step never gets to
// put text in front of the user unless that text was found here first.
// Offsets are rune positions into the original Document.Content (not
// bytes), so they stay meaningful for both Go and JS consumers as long as
// the text is within the Basic Multilingual Plane, which Japanese is.
package service

import (
	"strings"
	"unicode"
)

type Grounded struct {
	Quote       string // the exact substring as it appears in the source (not the LLM's version)
	StartOffset int
	EndOffset   int // exclusive
}

// Ground looks for quote inside content, first as an exact substring, then
// under normalization (whitespace collapsed, full-width ASCII folded to
// half-width, common Japanese/Latin punctuation dropped). It reports
// whether the quote could be verified at all; callers must discard
// (not display) any quote Ground rejects.
func Ground(content, quote string) (Grounded, bool) {
	if strings.TrimSpace(quote) == "" {
		return Grounded{}, false
	}

	contentRunes := []rune(content)
	quoteRunes := []rune(quote)

	if start := runeIndex(contentRunes, quoteRunes); start >= 0 && len(quoteRunes) > 0 {
		end := start + len(quoteRunes)
		return Grounded{Quote: string(contentRunes[start:end]), StartOffset: start, EndOffset: end}, true
	}

	normContent := buildNormalizedIndex(contentRunes)
	normQuote := buildNormalizedIndex(quoteRunes)
	if len(normQuote.runes) == 0 {
		return Grounded{}, false
	}

	pos := runeIndex(normContent.runes, normQuote.runes)
	if pos < 0 {
		return Grounded{}, false
	}

	start := normContent.origIndex[pos]
	end := normContent.origIndex[pos+len(normQuote.runes)-1] + 1
	return Grounded{Quote: string(contentRunes[start:end]), StartOffset: start, EndOffset: end}, true
}

type normalizedIndex struct {
	runes []rune
	// origIndex[i] is the index into the original rune slice that
	// produced runes[i].
	origIndex []int
}

// buildNormalizedIndex drops whitespace and punctuation entirely (rather
// than collapsing them to a single space) so that "毎回、確認します" and
// "毎回 確認します" normalize identically regardless of which kind of
// separator - or none - a model reproduces between words.
func buildNormalizedIndex(orig []rune) normalizedIndex {
	var idx normalizedIndex
	for i, r := range orig {
		nr, keep := normalizeRune(r)
		if !keep {
			continue
		}
		idx.runes = append(idx.runes, nr)
		idx.origIndex = append(idx.origIndex, i)
	}
	return idx
}

func normalizeRune(r rune) (rune, bool) {
	if unicode.IsSpace(r) {
		return 0, false
	}
	switch r {
	case '、', '，', '。', '．',
		'「', '」', '『', '』',
		'"', '\'', '“', '”', '‘', '’',
		'(', ')', '（', '）':
		return 0, false
	}
	// Fold full-width ASCII (！-～, U+FF01-FF5E) to half-width.
	if r >= 0xFF01 && r <= 0xFF5E {
		r -= 0xFEE0
	}
	return unicode.ToLower(r), true
}

// runeIndex finds the first occurrence of needle in haystack, or -1.
func runeIndex(haystack, needle []rune) int {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return -1
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
