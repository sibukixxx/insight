package service

import (
	"strings"
	"testing"

	"insight-lab/internal/domain"
)

func TestPrepareLLMContextRemovesDuplicateAndNoisePassages(t *testing.T) {
	docs := []*domain.Document{
		{ID: "doc_1", Content: "Useful customer behavior with enough detail."},
		{ID: "doc_2", Content: "  Useful customer behavior with enough detail.  "},
		{ID: "doc_3", Content: "---"},
	}

	passages, stats := prepareLLMContext(docs, defaultContextRuneLimit)

	if len(passages) != 1 {
		t.Fatalf("passages = %d, want 1", len(passages))
	}
	if stats.DroppedPassages[contextDropDuplicate] != 1 {
		t.Errorf("duplicate drops = %d, want 1", stats.DroppedPassages[contextDropDuplicate])
	}
	if stats.DroppedPassages[contextDropNoise] != 1 {
		t.Errorf("noise drops = %d, want 1", stats.DroppedPassages[contextDropNoise])
	}
}

func TestPrepareLLMContextKeepsCounterEvidenceAheadOfOrdinaryTextAtLimit(t *testing.T) {
	docs := []*domain.Document{
		{ID: "ordinary", Content: strings.Repeat("ordinary evidence ", 20)},
		{ID: "counter", Content: "However, this contradicts the hypothesis and is a limitation."},
	}

	passages, stats := prepareLLMContext(docs, 80)

	if len(passages) != 1 || passages[0].document.ID != "counter" {
		t.Fatalf("kept passages = %+v, want protected counter-evidence", passages)
	}
	if stats.DroppedPassages[contextDropLimit] != 1 {
		t.Errorf("limit drops = %d, want 1", stats.DroppedPassages[contextDropLimit])
	}
}

func TestPrepareLLMContextRecordsBeforeAndAfterSize(t *testing.T) {
	docs := []*domain.Document{
		{ID: "doc_1", Content: "Useful customer behavior with enough detail."},
		{ID: "doc_2", Content: "Useful customer behavior with enough detail."},
	}

	_, stats := prepareLLMContext(docs, defaultContextRuneLimit)

	if stats.BeforeDocuments != 2 || stats.AfterDocuments != 1 {
		t.Errorf("document counts = %d -> %d, want 2 -> 1", stats.BeforeDocuments, stats.AfterDocuments)
	}
	if stats.AfterCharacters >= stats.BeforeCharacters {
		t.Errorf("characters = %d -> %d, want a reduction", stats.BeforeCharacters, stats.AfterCharacters)
	}
	if stats.BeforeEstimatedTokens != estimateTokens(stats.BeforeCharacters) || stats.AfterEstimatedTokens != estimateTokens(stats.AfterCharacters) {
		t.Errorf("estimated token counts do not match character counts: %+v", stats)
	}
}
