package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"

	"insight-lab/internal/domain"
	"insight-lab/internal/triage"
)

// Import kinds accepted by the project document import endpoints. The
// browser UI reads them from /api/import-formats instead of keeping its own
// copy, so a format it offers is always one the server accepts.
const (
	ImportKindDocuments = "documents"
	ImportKindAnalysis  = "analysis"
)

type ImportColumn struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
}

// ImportFormat describes one file format the project import endpoints
// accept: the columns, the accepted extension and media type, and the
// source types a documents CSV may carry.
type ImportFormat struct {
	Kind        string         `json:"kind"`
	Extensions  []string       `json:"extensions"`
	MediaTypes  []string       `json:"mediaTypes"`
	Encoding    string         `json:"encoding"`
	Columns     []ImportColumn `json:"columns"`
	SourceTypes []string       `json:"sourceTypes,omitempty"`
}

// analysisOptionalColumns are read by the external-registry-export CSV but
// not required; the template lists them so a filled template round-trips.
var analysisOptionalColumns = []string{"name", "kind", "prefecture_code", "city_code", "close_cause"}

// ImportFormats lists the supported import formats. Only CSV (UTF-8, BOM
// tolerated) is accepted; other file types are not ingested directly.
func ImportFormats() []ImportFormat {
	sources := make([]string, 0, len(domain.SourceTypes()))
	for _, s := range domain.SourceTypes() {
		sources = append(sources, string(s))
	}
	documentColumns := make([]ImportColumn, 0, len(csvHeader))
	for _, name := range csvHeader {
		documentColumns = append(documentColumns, ImportColumn{Name: name, Required: true})
	}
	analysisColumns := make([]ImportColumn, 0, len(analysisRequiredColumns)+len(analysisOptionalColumns))
	for _, name := range analysisRequiredColumns {
		analysisColumns = append(analysisColumns, ImportColumn{Name: name, Required: true})
	}
	for _, name := range analysisOptionalColumns {
		analysisColumns = append(analysisColumns, ImportColumn{Name: name})
	}
	csvFormat := func(kind string, columns []ImportColumn, sourceTypes []string) ImportFormat {
		return ImportFormat{
			Kind: kind, Extensions: []string{".csv"}, MediaTypes: []string{"text/csv"},
			Encoding: "UTF-8", Columns: columns, SourceTypes: sourceTypes,
		}
	}
	return []ImportFormat{
		csvFormat(ImportKindDocuments, documentColumns, sources),
		csvFormat(ImportKindAnalysis, analysisColumns, nil),
	}
}

// FindImportFormat returns the format for kind, or false when the kind is
// not supported.
func FindImportFormat(kind string) (ImportFormat, bool) {
	for _, f := range ImportFormats() {
		if f.Kind == kind {
			return f, true
		}
	}
	return ImportFormat{}, false
}

// ImportTemplateCSV is the header-only CSV for kind. It carries no sample
// rows, so delivery builds ship no sample data through it.
func ImportTemplateCSV(kind string) ([]byte, error) {
	format, ok := FindImportFormat(kind)
	if !ok {
		return nil, fmt.Errorf("unsupported import kind %q", kind)
	}
	header := make([]string, 0, len(format.Columns))
	for _, c := range format.Columns {
		header = append(header, c.Name)
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(header); err != nil {
		return nil, err
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// MaxImportPreviewBytes bounds the file a preview reads into memory.
const MaxImportPreviewBytes = 32 << 20

// PreviewDocumentLimit is how many of the documents an import would create
// a preview returns.
const PreviewDocumentLimit = 5

// ImportPreview is what an import would do, computed by the same importer
// against a repository that keeps nothing. Profile is the Data Triage
// dataset profile of the same bytes; ProfileError explains a missing one.
type ImportPreview struct {
	Kind           string             `json:"kind"`
	RecordsRead    int                `json:"recordsRead"`
	Importable     int                `json:"importable"`
	Skipped        int                `json:"skipped"`
	Errors         []ImportRowError   `json:"errors"`
	FileHash       string             `json:"fileHash"`
	Documents      []*domain.Document `json:"-"`
	TotalDocuments int                `json:"totalDocuments"`
	Profile        *triage.Profile    `json:"profile,omitempty"`
	ProfileError   string             `json:"profileError,omitempty"`
}

// discardDocuments satisfies DocumentRepository for a dry run: writes are
// captured, never stored.
type discardDocuments struct{ created []*domain.Document }

func (d *discardDocuments) Create(_ context.Context, doc *domain.Document) error {
	d.created = append(d.created, doc)
	return nil
}

func (d *discardDocuments) CreateBatch(_ context.Context, docs []*domain.Document) error {
	d.created = append(d.created, docs...)
	return nil
}

func (d *discardDocuments) Get(context.Context, string) (*domain.Document, error) {
	return nil, fmt.Errorf("a preview stores no documents")
}

func (d *discardDocuments) ListByProject(context.Context, string) ([]*domain.Document, error) {
	return nil, nil
}

// PreviewImport runs the importer for kind without storing anything. A
// file the importer rejects outright (wrong header, empty) returns the
// importer's own error.
func PreviewImport(ctx context.Context, kind, projectID string, r io.Reader) (*ImportPreview, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxImportPreviewBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}
	if len(data) > MaxImportPreviewBytes {
		return nil, fmt.Errorf("the file is larger than the %d MB preview limit", MaxImportPreviewBytes>>20)
	}
	sink := &discardDocuments{}
	preview := &ImportPreview{Kind: kind}
	switch kind {
	case ImportKindDocuments:
		res, err := ImportCSV(ctx, sink, projectID, bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		preview.RecordsRead = res.Imported + res.Skipped
		preview.Importable, preview.Skipped, preview.Errors, preview.FileHash = res.Imported, res.Skipped, res.Errors, res.FileHash
	case ImportKindAnalysis:
		res, err := ImportAnalysisCSV(ctx, sink, projectID, bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		preview.RecordsRead = res.RecordsRead
		preview.Importable, preview.Skipped, preview.Errors, preview.FileHash = res.Imported, res.Skipped, res.Errors, res.FileHash
	default:
		return nil, fmt.Errorf("unsupported import kind %q", kind)
	}
	if preview.Errors == nil {
		preview.Errors = []ImportRowError{}
	}
	preview.TotalDocuments = len(sink.created)
	preview.Documents = sink.created[:min(len(sink.created), PreviewDocumentLimit)]
	if profile, err := triage.ProfileCSV(data); err != nil {
		preview.ProfileError = err.Error()
	} else {
		preview.Profile = &profile
	}
	return preview, nil
}
