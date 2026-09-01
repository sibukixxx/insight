package domain

import "time"

type SourceType string

const (
	SourceInterview SourceType = "interview"
	SourceReview    SourceType = "review"
	SourceSupport   SourceType = "support"
	SourceSales     SourceType = "sales"
	SourceSurvey    SourceType = "survey"
)

func (s SourceType) Valid() bool {
	switch s {
	case SourceInterview, SourceReview, SourceSupport, SourceSales, SourceSurvey:
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
