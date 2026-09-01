package service

import (
	"insight-lab/internal/domain"
)

// ConfidenceInput carries everything the app-side confidence formula
// needs. The LLM is never asked for a confidence number (see
// docs/design-review.md P0-2 / design §7): every input here comes from
// grounded Evidence and simple project-wide counts, not model opinion.
type ConfidenceInput struct {
	SupportingEvidence []*domain.Evidence
	CounterEvidence    []*domain.Evidence
	// SupportingSourceTypes is the SourceType of each document a piece of
	// supporting evidence came from (duplicates allowed; diversity counts
	// distinct values).
	SupportingSourceTypes []domain.SourceType
	// DocumentsWithSupport / TotalDocuments give document-level coverage
	// (the draft design used per-participant coverage, but the domain
	// model has no participant concept - see design-review.md P0-2).
	DocumentsWithSupport int
	TotalDocuments       int
	// PatternDocumentCount is how many distinct documents exhibited the
	// pattern this insight was built from.
	PatternDocumentCount int
}

// sourceDiversitySlots is the number of distinct domain.SourceType values
// that exist, used to normalize how many kinds of source an insight's
// supporting evidence spans.
const sourceDiversitySlots = 5

// Confidence combines evidence strength, coverage, source diversity and
// pattern frequency into a single 0..1 score, then discounts it slightly
// for each piece of counter-evidence found (a hypothesis with real
// counter-evidence is less certain than one where none turned up).
func Confidence(in ConfidenceInput) float64 {
	strength := evidenceStrength(in.SupportingEvidence)
	coverage := ratio(in.DocumentsWithSupport, in.TotalDocuments)
	diversity := clamp01(float64(distinctSourceTypes(in.SupportingSourceTypes)) / sourceDiversitySlots)
	frequency := clamp01(float64(in.PatternDocumentCount) / 5.0)

	score := strength*0.35 + coverage*0.25 + diversity*0.20 + frequency*0.20

	counterPenalty := 1.0 - 0.1*float64(min(len(in.CounterEvidence), 3))
	score *= counterPenalty

	return clamp01(score)
}

func evidenceStrength(evidence []*domain.Evidence) float64 {
	if len(evidence) == 0 {
		return 0
	}
	var sum float64
	for _, e := range evidence {
		sum += e.RelevanceScore
	}
	return clamp01(sum / float64(len(evidence)))
}

func distinctSourceTypes(types []domain.SourceType) int {
	seen := map[domain.SourceType]struct{}{}
	for _, t := range types {
		seen[t] = struct{}{}
	}
	return len(seen)
}

func ratio(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return clamp01(float64(numerator) / float64(denominator))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
