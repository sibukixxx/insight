package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

var analysisRequiredColumns = []string{
	"corporate_number", "event_type", "prefecture_name", "city_name",
	"assignment_date", "update_date", "change_date", "close_date",
	"source_provider", "source_version", "source_fetched_at",
}

type AnalysisImportResult struct {
	RecordsRead int              `json:"recordsRead"`
	Imported    int              `json:"imported"`
	Skipped     int              `json:"skipped"`
	Errors      []ImportRowError `json:"errors"`
	FileHash    string           `json:"fileHash"` // sha256 of the imported bytes, see ImportResult.FileHash
}

type analysisGroup struct {
	period          string
	eventType       string
	prefecture      string
	city            string
	sourceProvider  string
	sourceVersion   string
	sourceFetchedAt string
	count           int
}

// ImportAnalysisCSV is a boundary adapter for the CSV contract exported by
// external-registry-export. It has no Go dependency on that repository and persists
// only generic dataset Documents. Every document is a deterministic count of
// input rows, never an inferred count of startups or business commencements.
func ImportAnalysisCSV(ctx context.Context, documents repository.DocumentRepository, projectID string, input io.Reader) (*AnalysisImportResult, error) {
	return ImportAnalysisCSVWithManifest(ctx, documents, projectID, input, nil)
}

// ImportAnalysisCSVWithManifest is ImportAnalysisCSV plus an optional
// acquisition manifest describing where the export came from. Row-level
// provider/version columns stay as they are; the manifest is file-level
// provenance layered on top of them.
func ImportAnalysisCSVWithManifest(ctx context.Context, documents repository.DocumentRepository, projectID string, input io.Reader, manifest *AcquisitionManifest) (*AnalysisImportResult, error) {
	if manifest != nil {
		if err := manifest.Validate(); err != nil {
			return nil, err
		}
	}
	hashed := newHashingReader(input)
	reader := csv.NewReader(stripBOM(hashed))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("analysis CSV is empty")
		}
		return nil, fmt.Errorf("read analysis CSV: %w", err)
	}
	columns := indexCSVColumns(header)
	for _, required := range analysisRequiredColumns {
		if _, ok := columns[required]; !ok {
			return nil, fmt.Errorf("analysis CSV is missing required column %q", required)
		}
	}

	result := &AnalysisImportResult{}
	groups := map[string]*analysisGroup{}
	row := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		row++
		result.RecordsRead++
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: row, Reason: err.Error()})
			continue
		}
		value := func(name string) string {
			index := columns[name]
			if index >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[index])
		}
		if value("corporate_number") == "" {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: row, Reason: "corporate_number is empty"})
			continue
		}
		eventType := value("event_type")
		eventDate := analysisEventDate(eventType, value)
		if _, err := time.Parse("2006-01-02", eventDate); err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: row, Reason: fmt.Sprintf("event date is unavailable or invalid for %q", eventType)})
			continue
		}
		provider := value("source_provider")
		if provider == "" {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowError{Row: row, Reason: "source_provider is empty"})
			continue
		}
		period := eventDate[:7]
		key := strings.Join([]string{period, eventType, value("prefecture_name"), value("city_name"), provider, value("source_version")}, "\x00")
		group := groups[key]
		if group == nil {
			group = &analysisGroup{period: period, eventType: eventType, prefecture: value("prefecture_name"), city: value("city_name"), sourceProvider: provider, sourceVersion: value("source_version"), sourceFetchedAt: value("source_fetched_at")}
			groups[key] = group
		}
		group.count++
	}

	result.FileHash = hashed.Sum()
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	docs := make([]*domain.Document, 0, len(keys))
	for _, key := range keys {
		group := groups[key]
		location := strings.TrimSpace(group.prefecture + " " + group.city)
		if location == "" {
			location = "unspecified"
		}
		content := fmt.Sprintf("Dataset observation: period=%s; location=%s; event_type=%s; record_count=%d; source_provider=%s; source_version=%s. The count represents exported administrative records classified by the source adapter. It does not by itself represent company founding, business commencement, policy effect, or causal impact.", group.period, location, group.eventType, group.count, group.sourceProvider, group.sourceVersion)
		docs = append(docs, &domain.Document{
			ID: newID("doc"), ProjectID: projectID, Source: domain.SourceDataset,
			Title:   fmt.Sprintf("%s %s %s (%d records)", group.period, location, group.eventType, group.count),
			Content: content,
			Metadata: provenanceMetadata(map[string]string{
				"adapter": "corporate-event-analysis-csv", "period": group.period, "event_type": group.eventType,
				"prefecture_name": group.prefecture, "city_name": group.city, "record_count": fmt.Sprint(group.count),
				"source_provider": group.sourceProvider, "source_version": group.sourceVersion, "source_fetched_at": group.sourceFetchedAt,
			}, result.FileHash, manifest),
			CreatedAt: time.Now().UTC(),
		})
	}
	if len(docs) > 0 {
		if err := documents.CreateBatch(ctx, docs); err != nil {
			return nil, fmt.Errorf("save analysis documents: %w", err)
		}
	}
	result.Imported = len(docs)
	return result, nil
}

func indexCSVColumns(header []string) map[string]int {
	columns := make(map[string]int, len(header))
	for i, name := range header {
		columns[strings.ToLower(strings.TrimSpace(name))] = i
	}
	return columns
}

func analysisEventDate(eventType string, value func(string) string) string {
	switch eventType {
	case "ASSIGNED":
		return value("assignment_date")
	case "UPDATED":
		return value("update_date")
	case "CHANGED":
		return value("change_date")
	case "CLOSED":
		return value("close_date")
	default:
		return ""
	}
}
