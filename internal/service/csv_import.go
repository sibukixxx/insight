package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

// CSV documents use a fixed 4-column shape (id,source,title,content) per
// docs/detailed-design.md §17. UTF-8 only; a BOM is stripped automatically
// since Excel commonly adds one.
var csvHeader = []string{"id", "source", "title", "content"}

type ImportRowError struct {
	Row    int    `json:"row"` // 1-based, header excluded
	Reason string `json:"reason"`
}

type ImportResult struct {
	Imported int              `json:"imported"`
	Skipped  int              `json:"skipped"`
	Errors   []ImportRowError `json:"errors"`
}

// ImportCSV parses r as the fixed id,source,title,content CSV and inserts
// every valid row as a Document under projectID. The document's own ID is
// generated fresh rather than trusting the CSV's id column, since that
// column isn't guaranteed unique across separate imports/projects; the
// original value is kept in metadata["csv_id"] for traceability.
func ImportCSV(ctx context.Context, documents repository.DocumentRepository, projectID string, r io.Reader) (*ImportResult, error) {
	reader := csv.NewReader(stripBOM(r))
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("CSVが空です")
		}
		return nil, fmt.Errorf("CSVの読み込みに失敗しました: %w", err)
	}
	if !headerMatches(header) {
		return nil, fmt.Errorf("CSVのヘッダーは %s である必要があります", strings.Join(csvHeader, ","))
	}

	result := &ImportResult{}
	var toInsert []*domain.Document
	row := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		row++
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: row, Reason: err.Error()})
			continue
		}
		if len(record) < 4 {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: row, Reason: "列が不足しています"})
			continue
		}

		csvID, source, title, content := record[0], record[1], record[2], record[3]
		sourceType := domain.SourceType(strings.TrimSpace(source))
		if !sourceType.Valid() {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: row, Reason: fmt.Sprintf("不正なsource: %q", source)})
			continue
		}
		if strings.TrimSpace(content) == "" {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: row, Reason: "contentが空です"})
			continue
		}

		meta := map[string]string{}
		if csvID != "" {
			meta["csv_id"] = csvID
		}
		toInsert = append(toInsert, &domain.Document{
			ID: newID("doc"), ProjectID: projectID, Source: sourceType,
			Title: title, Content: content, Metadata: meta, CreatedAt: time.Now().UTC(),
		})
	}

	if len(toInsert) > 0 {
		if err := documents.CreateBatch(ctx, toInsert); err != nil {
			return nil, fmt.Errorf("保存に失敗しました: %w", err)
		}
	}
	result.Imported = len(toInsert)
	return result, nil
}

func headerMatches(header []string) bool {
	if len(header) < len(csvHeader) {
		return false
	}
	for i, want := range csvHeader {
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
