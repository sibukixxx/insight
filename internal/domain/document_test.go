package domain

import "testing"

func TestSourceTypeValid(t *testing.T) {
	valid := []SourceType{SourceInterview, SourceReview, SourceSupport, SourceSales, SourceSurvey}
	for _, s := range valid {
		if !s.Valid() {
			t.Errorf("expected %q to be valid", s)
		}
	}

	invalid := []SourceType{"", "email", "chat_log"}
	for _, s := range invalid {
		if s.Valid() {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}
