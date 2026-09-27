package service

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
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
	// FileHash is the sha256 of the imported bytes. Every created document
	// carries it as metadata so a run can be traced back to the exact file.
	FileHash string `json:"fileHash"`
}

// ImportCSV parses r as the fixed id,source,title,content CSV and inserts
// every valid row as a Document under projectID. The document's own ID is
// generated fresh rather than trusting the CSV's id column, since that
// column isn't guaranteed unique across separate imports/projects; the
// original value is kept in metadata["csv_id"] for traceability.
func ImportCSV(ctx context.Context, documents repository.DocumentRepository, projectID string, r io.Reader) (*ImportResult, error) {
	return ImportCSVWithManifest(ctx, documents, projectID, r, nil)
}

// ImportCSVWithManifest is ImportCSV plus an optional acquisition manifest
// (Issue #16). The manifest is validated before any row is read and, with
// the file hash filled in, attached to every created document.
func ImportCSVWithManifest(ctx context.Context, documents repository.DocumentRepository, projectID string, r io.Reader, manifest *AcquisitionManifest) (*ImportResult, error) {
	if manifest != nil {
		if err := manifest.Validate(); err != nil {
			return nil, err
		}
	}
	hashed := newHashingReader(r)
	reader := csv.NewReader(stripBOM(hashed))
	reader.FieldsPerRecord = -1
	if err := readDocumentsHeader(reader); err != nil {
		return nil, err
	}

	result := &ImportResult{}
	var toInsert []*domain.Document
	err := scanDocumentRows(reader, projectID,
		func(doc *domain.Document) error {
			toInsert = append(toInsert, doc)
			return nil
		},
		func(e ImportRowError) error {
			result.Skipped++
			result.Errors = append(result.Errors, e)
			return nil
		})
	if err != nil {
		return nil, err
	}

	result.FileHash = hashed.Sum()
	for _, doc := range toInsert {
		doc.Metadata = provenanceMetadata(doc.Metadata, result.FileHash, manifest)
	}
	if len(toInsert) > 0 {
		if err := documents.CreateBatch(ctx, toInsert); err != nil {
			return nil, fmt.Errorf("save document: %w", err)
		}
	}
	result.Imported = len(toInsert)
	return result, nil
}

// readDocumentsHeader consumes and checks the id,source,title,content header.
func readDocumentsHeader(reader *csv.Reader) error {
	header, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return fmt.Errorf("CSV is empty")
		}
		return fmt.Errorf("read CSV: %w", err)
	}
	if !headerMatches(header) {
		return fmt.Errorf("CSV header must be %s", strings.Join(csvHeader, ","))
	}
	return nil
}

// scanDocumentRows reads the rows after the header, handing each valid row
// to emit as a Document without provenance metadata and every other row to
// reject. Nothing is retained between rows, so the caller decides whether
// documents accumulate. An error returned by emit or reject stops the scan.
func scanDocumentRows(reader *csv.Reader, projectID string, emit func(*domain.Document) error, reject func(ImportRowError) error) error {
	row := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			return nil
		}
		row++
		var reason string
		switch {
		case err != nil:
			if isFatalReadError(err) {
				return err
			}
			reason = err.Error()
		case len(record) < 4:
			reason = "not enough columns"
		case !domain.SourceType(strings.TrimSpace(record[1])).Valid():
			reason = fmt.Sprintf("invalid source: %q", record[1])
		case strings.TrimSpace(record[3]) == "":
			reason = "content is empty"
		}
		if reason != "" {
			if err := reject(ImportRowError{Row: row, Reason: reason}); err != nil {
				return err
			}
			continue
		}

		csvID, title, content := record[0], record[2], record[3]
		meta := map[string]string{}
		if csvID != "" {
			meta["csv_id"] = csvID
		}
		doc := &domain.Document{
			ID: newID("doc"), ProjectID: projectID, Source: domain.SourceType(strings.TrimSpace(record[1])),
			Title: title, Content: content, Metadata: meta, CreatedAt: time.Now().UTC(),
		}
		if err := emit(doc); err != nil {
			return err
		}
	}
}

// isFatalReadError reports whether a csv.Reader error came from the
// underlying stream rather than from the row's CSV syntax. A syntax error
// skips one row; a stream error (a cancelled or over-limit read) must stop
// the import instead of being recorded against every remaining row.
func isFatalReadError(err error) bool {
	var parseErr *csv.ParseError
	return !errors.As(err, &parseErr)
}

// provenanceMetadata adds the file hash and, when present, the manifest to
// base. The hash is written twice on purpose: once as a flat key any reader
// can grep for, once inside the manifest so the manifest is self-contained.
func provenanceMetadata(base map[string]string, fileHash string, manifest *AcquisitionManifest) map[string]string {
	if manifest == nil {
		out := make(map[string]string, len(base)+1)
		for k, v := range base {
			out[k] = v
		}
		out[MetadataDatasetHash] = fileHash
		return out
	}
	stamped := *manifest
	stamped.FileHash = fileHash
	return stamped.DocumentMetadata(base)
}

// hashingReader computes the sha256 of everything read through it, so the
// importer gets the file hash without buffering the upload in memory.
type hashingReader struct {
	r io.Reader
	h hash.Hash
}

func newHashingReader(r io.Reader) *hashingReader {
	h := sha256.New()
	return &hashingReader{r: io.TeeReader(r, h), h: h}
}

func (h *hashingReader) Read(p []byte) (int, error) { return h.r.Read(p) }

// Sum returns the hex digest of the bytes read so far. Call it after the
// CSV reader has hit EOF; the whole file has been consumed by then.
func (h *hashingReader) Sum() string { return hex.EncodeToString(h.h.Sum(nil)) }

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
