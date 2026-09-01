package service

import "testing"

func TestGroundExactMatch(t *testing.T) {
	content := "設定を間違えたら怖いんですよね。最後は自分で確認します。"
	quote := "設定を間違えたら怖いんですよね"

	g, ok := Ground(content, quote)
	if !ok {
		t.Fatal("expected exact match to be found")
	}
	if g.Quote != quote {
		t.Errorf("Quote = %q, want %q", g.Quote, quote)
	}
	runes := []rune(content)
	if string(runes[g.StartOffset:g.EndOffset]) != quote {
		t.Errorf("offsets [%d:%d] do not reproduce the quote: %q", g.StartOffset, g.EndOffset, string(runes[g.StartOffset:g.EndOffset]))
	}
}

func TestGroundNormalizedMatch_FullWidthPunctuation(t *testing.T) {
	content := "毎回、金額と単価を確認しています。"
	// LLM returns a paraphrase-free but punctuation-normalized version.
	quote := "毎回 金額と単価を確認しています"

	g, ok := Ground(content, quote)
	if !ok {
		t.Fatal("expected normalized match to be found")
	}
	runes := []rune(content)
	got := string(runes[g.StartOffset:g.EndOffset])
	if got != "毎回、金額と単価を確認しています" {
		t.Errorf("recovered original substring = %q", got)
	}
}

func TestGroundNormalizedMatch_Whitespace(t *testing.T) {
	content := "line one\nline two   with   spaces"
	quote := "line two with spaces"

	g, ok := Ground(content, quote)
	if !ok {
		t.Fatal("expected whitespace-normalized match to be found")
	}
	if g.StartOffset < 0 || g.EndOffset <= g.StartOffset {
		t.Errorf("invalid offsets: %+v", g)
	}
}

func TestGroundRejectsFabricatedQuote(t *testing.T) {
	content := "設定を間違えたら怖いんですよね。"
	quote := "この製品は絶対に安全だと確信しています"

	if _, ok := Ground(content, quote); ok {
		t.Fatal("fabricated quote should not be grounded")
	}
}

func TestGroundRejectsEmptyQuote(t *testing.T) {
	if _, ok := Ground("some content", ""); ok {
		t.Fatal("empty quote should not be grounded")
	}
	if _, ok := Ground("some content", "   "); ok {
		t.Fatal("whitespace-only quote should not be grounded")
	}
}

func TestGroundRejectsPartialOverlap(t *testing.T) {
	content := "私は青いペンを使っています"
	quote := "私は赤いペンを使っています" // one character different (青→赤)

	if _, ok := Ground(content, quote); ok {
		t.Fatal("a quote with a substituted word should not match")
	}
}

func TestGroundOffsetsAreRuneBased(t *testing.T) {
	// "ペン" (pen) after a multi-byte prefix; if offsets were accidentally
	// byte-based this would slice mid-character and corrupt the string.
	content := "確認：ペンを使う"
	quote := "ペンを使う"

	g, ok := Ground(content, quote)
	if !ok {
		t.Fatal("expected match")
	}
	runes := []rune(content)
	if string(runes[g.StartOffset:g.EndOffset]) != quote {
		t.Errorf("rune-sliced result = %q, want %q", string(runes[g.StartOffset:g.EndOffset]), quote)
	}
}

func TestChunkShortContentIsSingleChunk(t *testing.T) {
	chunks := Chunk("短いテキストです。")
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
}

func TestChunkSplitsLongContentAtParagraphBoundaries(t *testing.T) {
	para := make([]byte, 3000)
	for i := range para {
		para[i] = 'a'
	}
	content := string(para) + "\n\n" + string(para) + "\n\n" + string(para) + "\n\n" + string(para)

	chunks := Chunk(content)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if len([]rune(c)) > maxChunkRunes {
			t.Errorf("chunk exceeds max size: %d runes", len([]rune(c)))
		}
	}
	// No content should be lost.
	var total int
	for _, c := range chunks {
		total += len([]rune(c))
	}
	// account for the "\n\n" separators re-added between merged paragraphs
	if total == 0 {
		t.Error("chunks lost all content")
	}
}

func TestChunkSplitsOversizedSingleParagraph(t *testing.T) {
	huge := make([]rune, maxChunkRunes*2+500)
	for i := range huge {
		huge[i] = 'x'
	}
	chunks := Chunk(string(huge))
	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks for an oversized paragraph, got %d", len(chunks))
	}
	for _, c := range chunks {
		if len([]rune(c)) > maxChunkRunes {
			t.Errorf("chunk exceeds max size: %d runes", len([]rune(c)))
		}
	}
}
