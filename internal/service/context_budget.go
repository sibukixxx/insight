package service

import (
	"strings"
	"unicode"

	"insight-lab/internal/domain"
)

// contextBudgetRuleVersion is recorded in every run's execution snapshot. Bump it
// whenever context reduction limits or filters change in a way that can change results.
const contextBudgetRuleVersion = "context-budget/v1"

const defaultContextRuneLimit = 64_000

const (
	contextDropDuplicate = "duplicate"
	contextDropNoise     = "noise"
	contextDropLimit     = "context_limit"
)

// ContextStats makes every Context omission visible in the persisted analysis
// metrics. Estimated tokens deliberately use a deterministic approximation so
// collecting the metric never invokes a tokenizer or paid model.
type ContextStats struct {
	BeforeDocuments       int            `json:"beforeDocuments"`
	AfterDocuments        int            `json:"afterDocuments"`
	BeforeCharacters      int            `json:"beforeCharacters"`
	AfterCharacters       int            `json:"afterCharacters"`
	BeforeEstimatedTokens int            `json:"beforeEstimatedTokens"`
	AfterEstimatedTokens  int            `json:"afterEstimatedTokens"`
	DroppedPassages       map[string]int `json:"droppedPassages"`
}

type contextPassage struct {
	document  *domain.Document
	content   string
	order     int
	protected bool
}

func prepareLLMContext(docs []*domain.Document, runeLimit int) ([]contextPassage, ContextStats) {
	stats := ContextStats{
		BeforeDocuments: len(docs),
		DroppedPassages: map[string]int{},
	}
	if runeLimit <= 0 {
		runeLimit = defaultContextRuneLimit
	}

	seen := map[string]struct{}{}
	candidates := make([]contextPassage, 0, len(docs))
	order := 0
	for _, doc := range docs {
		if doc == nil {
			stats.DroppedPassages[contextDropNoise]++
			continue
		}
		stats.BeforeCharacters += len([]rune(doc.Content))
		// SourceDataset contains the deterministic aggregate produced by the
		// importer, not its raw CSV rows. Treating it exactly once here avoids
		// re-introducing the already-analysed raw input.
		for _, chunk := range contextPassages(doc.Content) {
			trimmed := strings.TrimSpace(chunk)
			if isContextNoise(trimmed) {
				stats.DroppedPassages[contextDropNoise]++
				continue
			}
			key := normalizeContextKey(trimmed)
			if _, exists := seen[key]; exists {
				stats.DroppedPassages[contextDropDuplicate]++
				continue
			}
			seen[key] = struct{}{}
			candidates = append(candidates, contextPassage{
				document: doc, content: trimmed, order: order,
				protected: isCounterEvidenceOrLimitation(trimmed),
			})
			order++
		}
	}
	stats.BeforeEstimatedTokens = estimateTokens(stats.BeforeCharacters)
	candidates = packContextPassages(candidates)

	// Counter-evidence and limitations get first claim on the budget. Original
	// order is restored afterwards so the model still receives stable context.
	prioritized := make([]contextPassage, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.protected {
			prioritized = append(prioritized, candidate)
		}
	}
	for _, candidate := range candidates {
		if !candidate.protected {
			prioritized = append(prioritized, candidate)
		}
	}

	kept := make([]contextPassage, 0, len(prioritized))
	used := 0
	for _, candidate := range prioritized {
		characters := len([]rune(candidate.content))
		if used+characters > runeLimit {
			stats.DroppedPassages[contextDropLimit]++
			continue
		}
		used += characters
		kept = append(kept, candidate)
	}
	sortContextPassages(kept)

	documentIDs := map[string]struct{}{}
	for _, passage := range kept {
		documentIDs[passage.document.ID] = struct{}{}
		stats.AfterCharacters += len([]rune(passage.content))
	}
	stats.AfterDocuments = len(documentIDs)
	stats.AfterEstimatedTokens = estimateTokens(stats.AfterCharacters)
	return kept, stats
}

func contextPassages(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	paragraphs := strings.Split(content, "\n\n")
	passages := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		passages = append(passages, Chunk(paragraph)...)
	}
	return passages
}

// packContextPassages limits prompt overhead after paragraph-level filtering.
// Only adjacent passages from the same source document are joined.
func packContextPassages(passages []contextPassage) []contextPassage {
	packed := make([]contextPassage, 0, len(passages))
	for _, passage := range passages {
		if len(packed) == 0 {
			packed = append(packed, passage)
			continue
		}
		last := &packed[len(packed)-1]
		joinedLength := len([]rune(last.content)) + 2 + len([]rune(passage.content))
		if last.document.ID == passage.document.ID && joinedLength <= maxChunkRunes {
			last.content += "\n\n" + passage.content
			last.protected = last.protected || passage.protected
			continue
		}
		packed = append(packed, passage)
	}
	return packed
}

func isContextNoise(s string) bool {
	if s == "" {
		return true
	}
	meaningful := 0
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			meaningful++
		}
	}
	return meaningful == 0
}

func normalizeContextKey(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

func isCounterEvidenceOrLimitation(s string) bool {
	lower := strings.ToLower(s)
	markers := []string{
		"counter", "contradict", "however", "limitation", "caveat", "exception",
		"反証", "矛盾", "一方で", "しかし", "限界", "制約", "例外", "注意",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func estimateTokens(characters int) int {
	if characters <= 0 {
		return 0
	}
	// A conservative language-neutral approximation for mixed Japanese and
	// Latin text; this metric is for comparing runs, not billing reconciliation.
	return (characters + 1) / 2
}

func sortContextPassages(passages []contextPassage) {
	for i := 1; i < len(passages); i++ {
		for j := i; j > 0 && passages[j].order < passages[j-1].order; j-- {
			passages[j], passages[j-1] = passages[j-1], passages[j]
		}
	}
}
