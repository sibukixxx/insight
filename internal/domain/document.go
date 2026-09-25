package domain

import "time"

type SourceType string

const (
	SourceInterview  SourceType = "interview"
	SourceReview     SourceType = "review"
	SourceSupport    SourceType = "support"
	SourceSales      SourceType = "sales"
	SourceSurvey     SourceType = "survey"
	SourceJobPosting SourceType = "job_posting" // 案件・募集文（発注者の悩み）
	SourceSocialPost SourceType = "social_post" // SNS投稿・伸びている投稿の観察
	SourceDataset    SourceType = "dataset"     // structured/pre-aggregated data
	// Generic evidence sources. Legacy domain-specific values above remain
	// valid for v1 compatibility, but the research core must not require them.
	SourceDocument SourceType = "document"
	SourceReport   SourceType = "report"
	SourcePaper    SourceType = "paper"
	SourceWeb      SourceType = "web"
	SourceRecord   SourceType = "record"
	SourceOther    SourceType = "other"
)

func (s SourceType) Valid() bool {
	switch s {
	case SourceInterview, SourceReview, SourceSupport, SourceSales, SourceSurvey,
		SourceJobPosting, SourceSocialPost, SourceDataset,
		SourceDocument, SourceReport, SourcePaper, SourceWeb, SourceRecord, SourceOther:
		return true
	}
	return false
}

type Document struct {
	ID        string
	ProjectID string
	Source    SourceType
	Title     string
	Content   string
	Metadata  map[string]string
	CreatedAt time.Time
}
