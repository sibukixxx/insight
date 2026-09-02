package domain

import (
	"sort"
	"strings"
	"time"
)

type SourceType string

const (
	SourceInterview  SourceType = "interview"
	SourceReview     SourceType = "review"
	SourceSupport    SourceType = "support"
	SourceSales      SourceType = "sales"
	SourceSurvey     SourceType = "survey"
	SourceJobPosting SourceType = "job_posting" // 案件・募集文（発注者の悩み）
	SourceSocialPost SourceType = "social_post" // SNS投稿・伸びている投稿の観察
)

func (s SourceType) Valid() bool {
	switch s {
	case SourceInterview, SourceReview, SourceSupport, SourceSales, SourceSurvey,
		SourceJobPosting, SourceSocialPost:
		return true
	}
	return false
}

// Provenance says whether a document is the person's own words or someone
// else's account of them. A sales rep's call notes are already an
// interpretation - the method's "observable fact" layer is missing - so
// evidence drawn from secondhand documents is weighted down (see
// confidence.go) and an insight resting only on such evidence is flagged.
type Provenance string

const (
	ProvenanceFirsthand  Provenance = "firsthand"  // 本人の発言・記述そのもの
	ProvenanceSecondhand Provenance = "secondhand" // 第三者による要約・メモ
)

func (p Provenance) Valid() bool {
	return p == ProvenanceFirsthand || p == ProvenanceSecondhand
}

// DefaultProvenance is the assumption made when an importer does not say:
// sales logs are usually written by the salesperson, everything else is
// usually the customer's own voice.
func DefaultProvenance(source SourceType) Provenance {
	if source == SourceSales {
		return ProvenanceSecondhand
	}
	return ProvenanceFirsthand
}

// SpeakerRole classifies who is talking in a span of a document. Only
// RoleCustomer text is analyzable: an interviewer's question or a support
// agent's reply must never be quoted as the customer's observation.
type SpeakerRole string

const (
	RoleCustomer    SpeakerRole = "customer"    // 分析対象の声（回答者・顧客・投稿者）
	RoleInterviewer SpeakerRole = "interviewer" // 質問者・モデレーター
	RoleAgent       SpeakerRole = "agent"       // サポート担当・営業担当
	RoleOther       SpeakerRole = "other"       // 司会進行・システムメッセージなど
)

func (r SpeakerRole) Valid() bool {
	switch r {
	case RoleCustomer, RoleInterviewer, RoleAgent, RoleOther:
		return true
	}
	return false
}

// Span is a contiguous region of Document.Content attributed to one
// speaker. Offsets are rune indices (End exclusive), the same unit
// Observation and Evidence use, so a grounded quote can be checked
// against a span without conversion.
type Span struct {
	Start   int         `json:"start"`
	End     int         `json:"end"`
	Speaker string      `json:"speaker,omitempty"` // label as it appeared in the source ("Q", "面接官", "田中")
	Role    SpeakerRole `json:"role"`
}

// Reserved metadata keys. Importers map their own columns onto these so
// the pipeline can describe a speaker's situation (feeding the "how would
// such a person normally behave" expectation) and, later, group insights
// by segment. Any other key is kept as free-form context.
const (
	MetaParticipantID = "participant_id"
	MetaRole          = "role"         // 発言者の役職・立場（経理担当、個人事業主）
	MetaCompanySize   = "company_size" // 30名、1名
	MetaSegment       = "segment"      // 任意のセグメント名（中小製造業、フリーランス）
	MetaPlan          = "plan"         // 契約プラン
	MetaDate          = "date"
	MetaRating        = "rating"
	MetaVolume        = "volume" // 利用量（月150件発行、など）
)

// situationKeys is the order in which reserved keys are rendered into a
// one-line description of the speaker's situation.
var situationKeys = []string{MetaRole, MetaCompanySize, MetaSegment, MetaPlan, MetaVolume}

type Document struct {
	ID         string
	ProjectID  string
	Source     SourceType
	Provenance Provenance
	Title      string
	// Content is the analyzable text: what grounding verifies quotes
	// against, what the UI highlights, and what is sent to the model.
	// When PII masking was applied at intake this is the masked text.
	Content string
	// RawContent is the pre-masking original, kept locally only when
	// masking changed something (empty otherwise). It is never sent to
	// the model.
	RawContent string
	// Spans attribute regions of Content to speakers. Empty means the whole
	// document is the customer's voice (a review, a survey answer, a paste
	// of a single person's words).
	Spans     []Span
	Metadata  map[string]string
	CreatedAt time.Time
}

// CustomerSpans returns the regions of Content that may be quoted,
// sorted by Start. A document without speaker attribution is one
// customer span covering everything.
func (d *Document) CustomerSpans() []Span {
	if len(d.Spans) == 0 {
		return []Span{{Start: 0, End: len([]rune(d.Content)), Role: RoleCustomer}}
	}
	var out []Span
	for _, s := range d.Spans {
		if s.Role == RoleCustomer && s.End > s.Start {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

// IsSecondhand reports whether the document is someone else's account of
// the customer rather than the customer's own words.
func (d *Document) IsSecondhand() bool {
	return d.Provenance == ProvenanceSecondhand
}

// Situation renders the reserved metadata into a short description of
// who is speaking ("経理担当 / 30名 / 月150件発行"), used by the trace
// detection step to form a situation-specific expectation. Empty when no
// reserved key is set.
func (d *Document) Situation() string {
	var parts []string
	for _, k := range situationKeys {
		if v := strings.TrimSpace(d.Metadata[k]); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, " / ")
}
