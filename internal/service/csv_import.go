package service

import (
	"context"
	"fmt"
	"io"
	"strings"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

// legacyCSVHeader is the original fixed layout (docs/detailed-design.md
// §17). ImportCSV keeps accepting exactly that shape; anything else goes
// through the mapping-based ImportTable so a user is never told their
// export "has the wrong header" when a mapping would do.
var legacyCSVHeader = []string{"id", "source", "title", "content"}

type ImportRowError struct {
	Row    int    `json:"row"` // 1-based, header excluded
	Reason string `json:"reason"`
}

type ImportResult struct {
	Imported int              `json:"imported"`
	Skipped  int              `json:"skipped"`
	Errors   []ImportRowError `json:"errors"`
	// Masked is the number of PII matches replaced across all rows.
	Masked int `json:"masked"`
	// WithSpeakers is how many rows looked like a transcript and were
	// stored with speaker spans.
	WithSpeakers int `json:"withSpeakers"`
}

// LegacyMapping is the ColumnMapping equivalent of the fixed
// id,source,title,content layout.
func LegacyMapping() domain.ColumnMapping {
	return domain.ColumnMapping{ContentColumn: "content", TitleColumn: "title", IDColumn: "id", SourceColumn: "source"}
}

// ImportCSV imports the fixed id,source,title,content layout. UTF-8 only;
// a BOM is stripped. Callers with other layouts use PreviewTable +
// ImportTable with an explicit mapping.
func ImportCSV(ctx context.Context, documents repository.DocumentRepository, projectID string, r io.Reader) (*ImportResult, error) {
	t, err := ParseTable(r)
	if err != nil {
		return nil, err
	}
	if !headerMatches(t.Headers) {
		return nil, fmt.Errorf("CSVのヘッダーは %s である必要があります（他の形式は列マッピング付きインポートを使ってください）", strings.Join(legacyCSVHeader, ","))
	}
	return ImportTable(ctx, documents, projectID, t, LegacyMapping(), ImportOptions{})
}

func headerMatches(header []string) bool {
	if len(header) < len(legacyCSVHeader) {
		return false
	}
	for i, want := range legacyCSVHeader {
		if strings.TrimSpace(strings.ToLower(header[i])) != want {
			return false
		}
	}
	return true
}

// stripBOM removes a leading UTF-8 byte-order mark, which Excel adds to
// CSV exports and encoding/csv otherwise treats as part of the first
// header cell.
func stripBOM(r io.Reader) io.Reader {
	br := &bomStrippingReader{r: r}
	return br
}

type bomStrippingReader struct {
	r       io.Reader
	checked bool
}

func (b *bomStrippingReader) Read(p []byte) (int, error) {
	if !b.checked {
		b.checked = true
		buf := make([]byte, 3)
		n, err := io.ReadFull(b.r, buf)
		bom := []byte{0xEF, 0xBB, 0xBF}
		if n == 3 && string(buf) == string(bom) {
			return b.r.Read(p)
		}
		// Not a BOM (or short read): replay whatever we consumed.
		b.r = io.MultiReader(strings.NewReader(string(buf[:n])), b.r)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return 0, err
		}
	}
	return b.r.Read(p)
}
