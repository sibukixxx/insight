package service

import (
	"context"
	"strings"
	"testing"

	"insight-lab/internal/domain"
)

const analysisCSVHeader = "corporate_number,name,kind,prefecture_code,prefecture_name,city_code,city_name,assignment_date,update_date,change_date,close_date,close_cause,event_type,source_provider,source_version,source_fetched_at\n"

func TestImportAnalysisCSVAggregatesAdministrativeRecordsWithoutCausalMeaning(t *testing.T) {
	documents, project := newTestDocumentRepo(t)
	input := analysisCSVHeader +
		"1000000000001,A,株式会社,13,サンプル都,229,サンプル市,2026-01-02,,,,,ASSIGNED,sample_registry,v4,2026-03-01T00:00:00Z\n" +
		"1000000000002,B,株式会社,13,サンプル都,229,サンプル市,2026-01-10,,,,,ASSIGNED,sample_registry,v4,2026-03-01T00:00:00Z\n" +
		"1000000000003,C,株式会社,13,サンプル都,229,サンプル市,2026-01-01,2026-02-03,,,,UPDATED,sample_registry,v4,2026-03-01T00:00:00Z\n"

	result, err := ImportAnalysisCSV(context.Background(), documents, project.ID, strings.NewReader(input))
	if err != nil {
		t.Fatalf("ImportAnalysisCSV: %v", err)
	}
	if result.RecordsRead != 3 || result.Imported != 2 || result.Skipped != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	docs, err := documents.ListByProject(context.Background(), project.ID)
	if err != nil || len(docs) != 2 {
		t.Fatalf("documents = %v, %v", docs, err)
	}
	var assigned *domain.Document
	for _, document := range docs {
		if document.Metadata["event_type"] == "ASSIGNED" {
			assigned = document
		}
		if document.Source != domain.SourceDataset {
			t.Errorf("source = %q, want dataset", document.Source)
		}
	}
	if assigned == nil || assigned.Metadata["record_count"] != "2" {
		t.Fatalf("assigned aggregate missing: %+v", assigned)
	}
	for _, forbidden := range []string{"startup_count", "起業数", "policy caused"} {
		if strings.Contains(assigned.Content, forbidden) {
			t.Errorf("adapter invented meaning %q: %s", forbidden, assigned.Content)
		}
	}
	for _, required := range []string{"record_count=2", "does not by itself represent company founding", "causal impact"} {
		if !strings.Contains(assigned.Content, required) {
			t.Errorf("content lacks guardrail %q: %s", required, assigned.Content)
		}
	}
}

func TestImportAnalysisCSVWithManifestRecordsHashAndManifestOnAggregates(t *testing.T) {
	documents, project := newTestDocumentRepo(t)
	input := analysisCSVHeader +
		"1000000000001,A,株式会社,13,サンプル都,229,サンプル市,2026-01-02,,,,,ASSIGNED,sample_registry,v4,2026-03-01T00:00:00Z\n" +
		"1000000000002,B,株式会社,13,サンプル都,229,サンプル市,2026-02-10,,,,,ASSIGNED,sample_registry,v4,2026-03-01T00:00:00Z\n"
	manifest := validManifest()
	manifest.SourceName = "external-registry-export"
	manifest.SchemaID = "corporate-event-analysis-csv"
	manifest.Unit = "administrative records"

	result, err := ImportAnalysisCSVWithManifest(context.Background(), documents, project.ID, strings.NewReader(input), &manifest)
	if err != nil {
		t.Fatalf("ImportAnalysisCSVWithManifest: %v", err)
	}
	if result.Imported != 2 || result.FileHash != sha256Hex(input) {
		t.Fatalf("unexpected result: %+v", result)
	}
	docs, _ := documents.ListByProject(context.Background(), project.ID)
	for _, doc := range docs {
		got, ok := ManifestFromDocument(doc)
		if !ok || got.FileHash != result.FileHash || got.Unit != "administrative records" {
			t.Errorf("aggregate %s lacks manifest provenance: %v", doc.Title, doc.Metadata)
		}
		if doc.Metadata[MetadataDatasetHash] != result.FileHash || doc.Metadata["record_count"] != "1" {
			t.Errorf("aggregate %s lost hash or count metadata: %v", doc.Title, doc.Metadata)
		}
	}
}

func TestImportAnalysisCSVRejectsMissingContractColumn(t *testing.T) {
	documents, project := newTestDocumentRepo(t)
	_, err := ImportAnalysisCSV(context.Background(), documents, project.ID, strings.NewReader("corporate_number,event_type\n1,ASSIGNED\n"))
	if err == nil || !strings.Contains(err.Error(), "missing required column") {
		t.Fatalf("expected contract error, got %v", err)
	}
}

func TestImportAnalysisCSVSkipsUnknownEventWithoutUsableDate(t *testing.T) {
	documents, project := newTestDocumentRepo(t)
	input := analysisCSVHeader + "1000000000001,A,株式会社,13,サンプル都,229,サンプル市,,,,,,,UNKNOWN,sample_registry,v4,2026-03-01T00:00:00Z\n"
	result, err := ImportAnalysisCSV(context.Background(), documents, project.ID, strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if result.Imported != 0 || result.Skipped != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}
