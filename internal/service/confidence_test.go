package service

import (
	"testing"

	"insight-lab/internal/domain"
)

func TestConfidenceStrongEvidenceHighCoverage(t *testing.T) {
	in := ConfidenceInput{
		SupportingEvidence: []*domain.Evidence{
			{RelevanceScore: 0.9}, {RelevanceScore: 0.85}, {RelevanceScore: 0.95},
		},
		SupportingSourceTypes: []domain.SourceType{
			domain.SourceInterview, domain.SourceReview, domain.SourceSupport,
		},
		DocumentsWithSupport: 8,
		TotalDocuments:       10,
		PatternDocumentCount: 6,
	}
	got := Confidence(in)
	if got < 0.6 {
		t.Errorf("Confidence = %f, want a high score for strong/diverse/well-covered evidence", got)
	}
	if got > 1.0 {
		t.Errorf("Confidence = %f, must not exceed 1.0", got)
	}
}

func TestConfidenceNoEvidenceIsZero(t *testing.T) {
	got := Confidence(ConfidenceInput{TotalDocuments: 10})
	if got != 0 {
		t.Errorf("Confidence with no evidence = %f, want 0", got)
	}
}

func TestConfidenceCounterEvidenceReducesScore(t *testing.T) {
	base := ConfidenceInput{
		SupportingEvidence: []*domain.Evidence{
			{RelevanceScore: 0.9}, {RelevanceScore: 0.9},
		},
		SupportingSourceTypes: []domain.SourceType{domain.SourceInterview, domain.SourceInterview},
		DocumentsWithSupport:  5,
		TotalDocuments:        10,
		PatternDocumentCount:  4,
	}
	withoutCounter := Confidence(base)

	withCounter := base
	withCounter.CounterEvidence = []*domain.Evidence{{RelevanceScore: 0.7}}
	got := Confidence(withCounter)

	if got >= withoutCounter {
		t.Errorf("Confidence with counter-evidence (%f) should be lower than without (%f)", got, withoutCounter)
	}
}

func TestConfidenceCounterEvidencePenaltyCaps(t *testing.T) {
	base := ConfidenceInput{
		SupportingEvidence:    []*domain.Evidence{{RelevanceScore: 1.0}},
		SupportingSourceTypes: []domain.SourceType{domain.SourceInterview},
		DocumentsWithSupport:  1,
		TotalDocuments:        1,
		PatternDocumentCount:  5,
	}
	threeCounters := base
	threeCounters.CounterEvidence = make([]*domain.Evidence, 3)
	tenCounters := base
	tenCounters.CounterEvidence = make([]*domain.Evidence, 10)

	c3 := Confidence(threeCounters)
	c10 := Confidence(tenCounters)
	if c3 != c10 {
		t.Errorf("penalty should cap at 3 counter-evidence items: c3=%f c10=%f", c3, c10)
	}
}

func TestConfidenceSourceDiversityRewardsMultipleSourceTypes(t *testing.T) {
	single := ConfidenceInput{
		SupportingEvidence:    []*domain.Evidence{{RelevanceScore: 0.8}, {RelevanceScore: 0.8}, {RelevanceScore: 0.8}},
		SupportingSourceTypes: []domain.SourceType{domain.SourceInterview, domain.SourceInterview, domain.SourceInterview},
		DocumentsWithSupport:  3,
		TotalDocuments:        10,
		PatternDocumentCount:  3,
	}
	diverse := single
	diverse.SupportingSourceTypes = []domain.SourceType{domain.SourceInterview, domain.SourceReview, domain.SourceSupport}

	if Confidence(diverse) <= Confidence(single) {
		t.Error("diverse sources should score at least as high as a single repeated source")
	}
}

func TestConfidenceNeverExceedsOne(t *testing.T) {
	huge := make([]*domain.Evidence, 50)
	for i := range huge {
		huge[i] = &domain.Evidence{RelevanceScore: 1.0}
	}
	in := ConfidenceInput{
		SupportingEvidence:    huge,
		SupportingSourceTypes: []domain.SourceType{domain.SourceInterview, domain.SourceReview, domain.SourceSupport, domain.SourceSales, domain.SourceSurvey},
		DocumentsWithSupport:  100,
		TotalDocuments:        1,
		PatternDocumentCount:  100,
	}
	got := Confidence(in)
	if got > 1.0 {
		t.Errorf("Confidence = %f, must not exceed 1.0", got)
	}
}
