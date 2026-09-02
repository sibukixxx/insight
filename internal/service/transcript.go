// Transcript parsing: rule-based speaker separation for interview
// transcripts and support threads. No model is involved - the result is
// deterministic, so the same paste always yields the same spans, and a
// wrong guess is something the user corrects once in the preview and the
// project remembers (IntakeProfile.SpeakerRoles).
package service

import (
	"regexp"
	"sort"
	"strings"

	"insight-lab/internal/domain"
)

// Turn is one speaker's contiguous utterance. Start/End are rune offsets
// into the original content covering the utterance text only (the label
// and any timestamp are excluded), so the resulting Span highlights what
// the person said and never the "面接官:" prefix.
type Turn struct {
	Speaker string             `json:"speaker"`
	Role    domain.SpeakerRole `json:"role"`
	Guessed bool               `json:"guessed"` // role came from a heuristic, not a known label or the profile
	Start   int                `json:"start"`
	End     int                `json:"end"`
	Text    string             `json:"text"`
}

type SpeakerSummary struct {
	Label   string             `json:"label"`
	Role    domain.SpeakerRole `json:"role"`
	Guessed bool               `json:"guessed"`
	Turns   int                `json:"turns"`
	Chars   int                `json:"chars"`
}

type TranscriptParse struct {
	// Detected is false when the text does not look like a transcript;
	// Turns is then empty and the whole document is the customer's voice.
	Detected bool             `json:"detected"`
	Turns    []Turn           `json:"turns"`
	Speakers []SpeakerSummary `json:"speakers"`
	// Warnings are human-readable intake problems (no customer speech,
	// ambiguous roles) for the preview UI.
	Warnings []string `json:"warnings"`
}

// Spans converts the turns into the document contract's speaker spans.
func (t TranscriptParse) Spans() []domain.Span {
	out := make([]domain.Span, 0, len(t.Turns))
	for _, turn := range t.Turns {
		out = append(out, domain.Span{Start: turn.Start, End: turn.End, Speaker: turn.Speaker, Role: turn.Role})
	}
	return out
}

// speakerLine matches the start of a turn: an optional timestamp
// ("00:12:34", "[12:34]", "(1:02:03)"), then a short label followed by a
// colon (half- or full-width), or a label in 【】/[] brackets, or "Q./A."
// style. The label group is the speaker.
var speakerLine = regexp.MustCompile(
	`^\s*(?:[\[(]?\d{1,2}:\d{2}(?::\d{2})?[\])]?\s*)?` +
		`(?:【([^】]{1,20})】|\[([^\]]{1,20})\]|([QA])\d{0,2}[.．]|([^\s:：\[\]【】/#|]{1,20})\s*[:：])\s*`)

var (
	interviewerLabels = set("q", "question", "interviewer", "moderator", "i", "m", "mod",
		"質問", "質問者", "面接官", "インタビュアー", "インタビュワー", "聞き手", "モデレーター", "司会", "進行", "調査員", "リサーチャー")
	customerLabels = set("a", "answer", "respondent", "participant", "user", "customer", "client", "r", "p", "interviewee",
		"回答", "回答者", "話し手", "参加者", "ユーザー", "ユーザ", "利用者", "顧客", "お客様", "お客さま", "客", "被験者", "対象者", "インタビュイー", "投稿者")
	agentLabels = set("agent", "support", "cs", "sales", "operator", "staff", "rep",
		"担当", "担当者", "オペレーター", "オペレータ", "サポート", "営業", "スタッフ", "窓口", "カスタマーサポート", "cs担当", "対応者")
)

func set(items ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(items))
	for _, it := range items {
		m[it] = struct{}{}
	}
	return m
}

// ParseTranscript splits content into speaker turns. roles overrides the
// role for a label (from the intake profile or the user's choice in the
// preview) and always wins over the built-in label lists and heuristics.
func ParseTranscript(content string, roles map[string]domain.SpeakerRole) TranscriptParse {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	runes := []rune(content)
	lines := strings.Split(content, "\n")

	type rawTurn struct {
		speaker    string
		start, end int
	}
	var raw []rawTurn
	labeledLines := 0
	pos := 0 // rune offset of the current line start
	for _, line := range lines {
		lineLen := len([]rune(line))
		if m := speakerLine.FindStringSubmatchIndex(line); m != nil {
			label := firstGroup(line, m)
			if label != "" {
				labeledLines++
				textStart := pos + len([]rune(line[:m[1]]))
				raw = append(raw, rawTurn{speaker: label, start: textStart, end: pos + lineLen})
				pos += lineLen + 1
				continue
			}
		}
		if len(raw) > 0 && strings.TrimSpace(line) != "" {
			raw[len(raw)-1].end = pos + lineLen
		}
		pos += lineLen + 1
	}

	result := TranscriptParse{}
	distinct := map[string]bool{}
	for _, t := range raw {
		distinct[t.speaker] = true
	}
	// Prose with an occasional "注意:" line is not a transcript. Require
	// either two speakers taking turns or a known transcript label.
	knownLabel := false
	for label := range distinct {
		if _, ok := roles[label]; ok {
			knownLabel = true
		}
		if _, ok := builtinRole(label); ok {
			knownLabel = true
		}
	}
	if len(raw) == 0 || (len(distinct) < 2 && !knownLabel) || (labeledLines < 2 && !knownLabel) {
		return result
	}
	result.Detected = true

	// Resolve roles: explicit mapping > built-in label lists > heuristic.
	assigned := map[string]domain.SpeakerRole{}
	guessed := map[string]bool{}
	var unknown []string
	for label := range distinct {
		if r, ok := roles[label]; ok && r.Valid() {
			assigned[label] = r
			continue
		}
		if r, ok := builtinRole(label); ok {
			assigned[label] = r
			continue
		}
		unknown = append(unknown, label)
	}
	chars := map[string]int{}
	for _, t := range raw {
		chars[t.speaker] += len([]rune(strings.TrimSpace(string(runes[t.start:t.end]))))
	}
	resolveUnknown(unknown, assigned, guessed, chars)

	for _, t := range raw {
		text := strings.TrimSpace(string(runes[t.start:t.end]))
		if text == "" {
			continue
		}
		// Shrink the span to the trimmed text so highlights are tight.
		start := t.start + leadingSpace(runes[t.start:t.end])
		end := start + len([]rune(text))
		result.Turns = append(result.Turns, Turn{
			Speaker: t.speaker, Role: assigned[t.speaker], Guessed: guessed[t.speaker],
			Start: start, End: end, Text: text,
		})
	}

	summary := map[string]*SpeakerSummary{}
	for _, turn := range result.Turns {
		s := summary[turn.Speaker]
		if s == nil {
			s = &SpeakerSummary{Label: turn.Speaker, Role: turn.Role, Guessed: turn.Guessed}
			summary[turn.Speaker] = s
		}
		s.Turns++
		s.Chars += len([]rune(turn.Text))
	}
	for _, s := range summary {
		result.Speakers = append(result.Speakers, *s)
	}
	sort.Slice(result.Speakers, func(i, j int) bool {
		if result.Speakers[i].Chars != result.Speakers[j].Chars {
			return result.Speakers[i].Chars > result.Speakers[j].Chars
		}
		return result.Speakers[i].Label < result.Speakers[j].Label
	})

	customerChars := 0
	for _, s := range result.Speakers {
		if s.Role == domain.RoleCustomer {
			customerChars += s.Chars
		}
		if s.Guessed {
			result.Warnings = append(result.Warnings, "話者「"+s.Label+"」の役割は推定です。プレビューで確認してください")
		}
	}
	if customerChars == 0 {
		result.Warnings = append(result.Warnings, "回答者（分析対象）の発言がありません。話者の役割を割り当ててください")
	}
	return result
}

func firstGroup(line string, m []int) string {
	for g := 1; g < len(m)/2; g++ {
		if m[2*g] >= 0 {
			return strings.TrimSpace(line[m[2*g]:m[2*g+1]])
		}
	}
	return ""
}

func builtinRole(label string) (domain.SpeakerRole, bool) {
	key := strings.ToLower(strings.TrimSpace(label))
	// "回答者A" / "参加者2" / "Q1" style: strip a trailing index.
	stripped := strings.TrimRight(key, "0123456789abcdefghijｂｃ１２３４５６７８９０")
	for _, k := range []string{key, stripped} {
		if _, ok := interviewerLabels[k]; ok {
			return domain.RoleInterviewer, true
		}
		if _, ok := customerLabels[k]; ok {
			return domain.RoleCustomer, true
		}
		if _, ok := agentLabels[k]; ok {
			return domain.RoleAgent, true
		}
	}
	return "", false
}

// resolveUnknown assigns roles to labels nothing recognized. With one
// unknown speaker next to a known interviewer or agent, it is the
// customer. With exactly two unknown speakers and nobody known, the one
// who talks more is the customer (interviewees talk more than
// interviewers). Anything else defaults to customer, flagged as a guess.
func resolveUnknown(unknown []string, assigned map[string]domain.SpeakerRole, guessed map[string]bool, chars map[string]int) {
	if len(unknown) == 0 {
		return
	}
	hasCustomer := false
	for _, r := range assigned {
		if r == domain.RoleCustomer {
			hasCustomer = true
		}
	}
	if len(unknown) == 2 && len(assigned) == 0 {
		a, b := unknown[0], unknown[1]
		if chars[a] < chars[b] {
			a, b = b, a
		}
		assigned[a] = domain.RoleCustomer
		assigned[b] = domain.RoleInterviewer
		guessed[a], guessed[b] = true, true
		return
	}
	for _, label := range unknown {
		if hasCustomer && len(unknown) == 1 {
			// Everyone analyzable is already known; a lone extra voice is
			// most likely the person asking questions.
			assigned[label] = domain.RoleInterviewer
		} else {
			assigned[label] = domain.RoleCustomer
		}
		guessed[label] = true
	}
}

func leadingSpace(r []rune) int {
	n := 0
	for n < len(r) && (r[n] == ' ' || r[n] == '\t' || r[n] == '　' || r[n] == '\n') {
		n++
	}
	return n
}
