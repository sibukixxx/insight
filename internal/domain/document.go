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
)

func (s SourceType) Valid() bool {
	switch s {
	case SourceInterview, SourceReview, SourceSupport, SourceSales, SourceSurvey,
		SourceJobPosting, SourceSocialPost:
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
