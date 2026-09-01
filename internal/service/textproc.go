package service

import "strings"

// maxChunkRunes bounds how much text is sent to the LLM in a single
// Observation Extraction call. Chunking happens on the text fed to the
// model only; grounding always verifies quotes against the untouched
// Document.Content (see grounding.go).
const maxChunkRunes = 8000

// Chunk splits content into pieces no longer than maxChunkRunes, breaking
// at paragraph boundaries where possible so a quote is never split across
// chunk edges when the model reads it back.
func Chunk(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if len([]rune(content)) <= maxChunkRunes {
		return []string{content}
	}

	paragraphs := strings.Split(content, "\n\n")
	var chunks []string
	var current strings.Builder
	currentLen := 0

	flush := func() {
		if current.Len() > 0 {
			chunks = append(chunks, current.String())
			current.Reset()
			currentLen = 0
		}
	}

	for _, p := range paragraphs {
		pLen := len([]rune(p))
		if pLen > maxChunkRunes {
			flush()
			chunks = append(chunks, splitOversizedParagraph(p)...)
			continue
		}
		if currentLen+pLen+2 > maxChunkRunes && currentLen > 0 {
			flush()
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
			currentLen += 2
		}
		current.WriteString(p)
		currentLen += pLen
	}
	flush()

	if len(chunks) == 0 {
		return []string{content}
	}
	return chunks
}

func splitOversizedParagraph(p string) []string {
	runes := []rune(p)
	var out []string
	for len(runes) > 0 {
		n := maxChunkRunes
		if n > len(runes) {
			n = len(runes)
		}
		out = append(out, string(runes[:n]))
		runes = runes[n:]
	}
	return out
}
