// PII masking at intake. The tool's promise is "data stays local; only
// the text needed for analysis goes to the model". For a delivery build
// running on a customer's real interviews that promise is only credible
// if names, addresses and contact details never leave the machine - so
// masking is deterministic, happens before anything is stored as
// Document.Content, and the unmasked original is kept in RawContent
// locally, never sent to the model.
//
// Two layers: built-in patterns that are the same for every project
// (email, URL, phone, postal code, card number, a Japanese name followed
// by an honorific), and a per-project dictionary (IntakeProfile.MaskTerms)
// for the client's own names and company names.
package service

import (
	"regexp"
	"sort"
	"strings"
)

// MaskKind labels what a placeholder replaced, so the preview can say
// "メール 2 件、電話 1 件" and the researcher can judge whether the pass was
// too aggressive.
type MaskKind string

const (
	MaskEmail    MaskKind = "email"
	MaskURL      MaskKind = "url"
	MaskPhone    MaskKind = "phone"
	MaskPostal   MaskKind = "postal"
	MaskCard     MaskKind = "card"
	MaskName     MaskKind = "name"
	MaskTerm     MaskKind = "term"
	maskKindNone MaskKind = ""
)

// Placeholders are Japanese so a masked quote still reads naturally in
// the UI ("[氏名]さんに確認してから送ります").
var maskPlaceholders = map[MaskKind]string{
	MaskEmail:  "[メールアドレス]",
	MaskURL:    "[URL]",
	MaskPhone:  "[電話番号]",
	MaskPostal: "[郵便番号]",
	MaskCard:   "[カード番号]",
	MaskName:   "[氏名]",
	MaskTerm:   "[固有名詞]",
}

type maskRule struct {
	kind MaskKind
	re   *regexp.Regexp
	// group, when > 0, is the submatch to replace instead of the whole
	// match (used to keep the honorific after a masked name).
	group int
}

var builtinMaskRules = []maskRule{
	{kind: MaskEmail, re: regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)},
	{kind: MaskURL, re: regexp.MustCompile(`https?://[^\s<>"'）)]+`)},
	{kind: MaskCard, re: regexp.MustCompile(`\b(?:\d{4}[ \-]){3}\d{4}\b`)},
	{kind: MaskPostal, re: regexp.MustCompile(`〒\s?\d{3}[\-‐]?\d{4}`)},
	// Japanese phone numbers: 0X-XXXX-XXXX, 0XX-XXX-XXXX, 0X0-XXXX-XXXX,
	// 10-11 digits starting with 0, or +81. Requiring the leading 0/+81 and
	// 9+ digits keeps dates (2026-08-01) and amounts (150件, 1,200円) out.
	{kind: MaskPhone, re: regexp.MustCompile(`(?:\+81[\-\s]?\d{1,4}|0\d{1,4})[\-\s(]?\d{1,4}[\-\s)]?\d{3,4}\b`)},
	// A 1-3 kanji run followed by an honorific, when the run starts a
	// kanji sequence (so 経理部長 or 営業担当 are not split). The honorific
	// is kept: "田中さん" -> "[氏名]さん".
	{kind: MaskName, re: regexp.MustCompile(`(?:^|[^\p{Han}々])([\p{Han}々]{1,3})(さん|様|さま|氏|くん|ちゃん)`), group: 1},
}

// nameStoplist are kanji runs that precede an honorific in ordinary words
// and are not names.
var nameStoplist = set("客", "皆", "奥", "神", "王", "殿", "御", "貴", "先", "彼", "姫", "母", "父", "兄", "姉", "弟", "妹", "嫁", "婿", "仏", "旦那", "奥", "お客", "皆", "各", "諸", "貴社", "御社", "弊社", "当社", "他")

// Masker applies the built-in rules plus a project dictionary.
type Masker struct {
	terms []string
}

// NewMasker builds a masker for a project. Dictionary terms are applied
// longest first so "株式会社サンプル" is masked as a whole before "サンプル"
// would be.
func NewMasker(terms []string) *Masker {
	var clean []string
	seen := map[string]bool{}
	for _, t := range terms {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		clean = append(clean, t)
	}
	sort.SliceStable(clean, func(i, j int) bool { return len([]rune(clean[i])) > len([]rune(clean[j])) })
	return &Masker{terms: clean}
}

type MaskResult struct {
	Masked string
	// Raw is the original text when anything was masked, "" otherwise -
	// the same convention as Document.RawContent.
	Raw    string
	Count  int
	ByKind map[MaskKind]int
}

// Mask replaces PII in s. It is pure: same input, same output.
func (m *Masker) Mask(s string) MaskResult {
	res := MaskResult{Masked: s, ByKind: map[MaskKind]int{}}
	out := s
	// Dictionary first: a client's company name may contain something a
	// built-in rule would otherwise mangle (an email-like handle, digits).
	for _, term := range m.terms {
		n := strings.Count(out, term)
		if n > 0 {
			out = strings.ReplaceAll(out, term, maskPlaceholders[MaskTerm])
			res.ByKind[MaskTerm] += n
			res.Count += n
		}
	}
	for _, rule := range builtinMaskRules {
		out = replaceRule(out, rule, &res)
	}
	res.Masked = out
	if res.Count > 0 {
		res.Raw = s
	}
	return res
}

func replaceRule(s string, rule maskRule, res *MaskResult) string {
	placeholder := maskPlaceholders[rule.kind]
	var b strings.Builder
	last := 0
	for _, loc := range rule.re.FindAllStringSubmatchIndex(s, -1) {
		start, end := loc[0], loc[1]
		if rule.group > 0 {
			start, end = loc[2*rule.group], loc[2*rule.group+1]
			if start < 0 {
				continue
			}
			if rule.kind == MaskName {
				if _, stop := nameStoplist[s[start:end]]; stop {
					continue
				}
			}
		}
		b.WriteString(s[last:start])
		b.WriteString(placeholder)
		last = end
		res.ByKind[rule.kind]++
		res.Count++
	}
	b.WriteString(s[last:])
	return b.String()
}

// MaskFunc adapts the masker to ImportOptions.Masker.
func (m *Masker) MaskFunc() func(string) (string, string, int) {
	return func(s string) (string, string, int) {
		r := m.Mask(s)
		return r.Masked, r.Raw, r.Count
	}
}
