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

func TestSourceTypesListsEveryValidSourceOnceWhenEnumerated(t *testing.T) {
	seen := map[SourceType]bool{}
	for _, s := range SourceTypes() {
		if !s.Valid() {
			t.Errorf("SourceTypes() includes invalid %q", s)
		}
		if seen[s] {
			t.Errorf("SourceTypes() lists %q twice", s)
		}
		seen[s] = true
	}
	if len(seen) != 14 {
		t.Errorf("SourceTypes() has %d entries, want 14", len(seen))
	}
}
