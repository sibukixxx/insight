package service

import (
	"context"
	"strings"
	"testing"
)

func TestImportFormatsDescribesDocumentAndAnalysisCSVWhenListed(t *testing.T) {
	documents, ok := FindImportFormat(ImportKindDocuments)
	if !ok || strings.Join(columnNames(documents.Columns), ",") != "id,source,title,content" {
		t.Fatalf("documents format = %+v", documents)
	}
	if len(documents.SourceTypes) != 14 || documents.SourceTypes[0] != "document" {
		t.Errorf("documents source types = %v", documents.SourceTypes)
	}
	analysis, ok := FindImportFormat(ImportKindAnalysis)
	if !ok {
		t.Fatal("analysis format missing")
	}
	required := 0
	for _, c := range analysis.Columns {
		if c.Required {
			required++
		}
	}
	if required != len(analysisRequiredColumns) {
		t.Errorf("analysis required columns = %d, want %d", required, len(analysisRequiredColumns))
	}
	if _, ok := FindImportFormat("pdf"); ok {
		t.Error("pdf must not be reported as supported")
	}
}

func TestImportTemplateCSVReturnsHeaderOnlyThatTheImporterAcceptsWhenGenerated(t *testing.T) {
	for _, kind := range []string{ImportKindDocuments, ImportKindAnalysis} {
		template, err := ImportTemplateCSV(kind)
		if err != nil {
			t.Fatal(err)
		}
		if lines := strings.Count(string(template), "\n"); lines != 1 {
			t.Errorf("%s template has %d lines, want header only", kind, lines)
		}
		preview, err := PreviewImport(context.Background(), kind, "p1", strings.NewReader(string(template)))
		if err != nil {
			t.Fatalf("%s template is rejected by its own importer: %v", kind, err)
		}
		if preview.RecordsRead != 0 || preview.Importable != 0 {
			t.Errorf("%s template preview = %+v", kind, preview)
		}
	}
	if _, err := ImportTemplateCSV("xlsx"); err == nil {
		t.Error("xlsx template must be an error")
	}
}

func TestPreviewImportReportsImportableRowsAndProfileWithoutStoringWhenDocumentsCSV(t *testing.T) {
	input := "id,source,title,content\n1,interview,A,hello\n2,bogus,B,world\n3,web,C,\n"
	preview, err := PreviewImport(context.Background(), ImportKindDocuments, "p1", strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if preview.RecordsRead != 3 || preview.Importable != 1 || preview.Skipped != 2 || len(preview.Errors) != 2 {
		t.Errorf("preview counts = %+v", preview)
	}
	if preview.TotalDocuments != 1 || preview.Documents[0].Title != "A" {
		t.Errorf("preview documents = %+v", preview.Documents)
	}
	if preview.Profile == nil || preview.Profile.RowCount != 3 || len(preview.Profile.Columns) != 4 {
		t.Errorf("profile = %+v (error %q)", preview.Profile, preview.ProfileError)
	}
}

func TestPreviewImportAggregatesLikeTheImporterWhenAnalysisCSV(t *testing.T) {
	input := analysisCSVHeader +
		"1000000000001,A,株式会社,13,サンプル都,229,サンプル市,2026-01-10,,,,,ASSIGNED,sample_registry,v4,2026-03-01T00:00:00Z\n" +
		"1000000000002,B,株式会社,13,サンプル都,229,サンプル市,2026-01-20,,,,,ASSIGNED,sample_registry,v4,2026-03-01T00:00:00Z\n"
	preview, err := PreviewImport(context.Background(), ImportKindAnalysis, "p1", strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if preview.RecordsRead != 2 || preview.Importable != 1 || preview.TotalDocuments != 1 {
		t.Errorf("preview = %+v", preview)
	}
	if !strings.Contains(preview.Documents[0].Content, "record_count=2") {
		t.Errorf("aggregate content = %q", preview.Documents[0].Content)
	}
}

func TestPreviewImportReturnsImporterErrorWhenHeaderIsWrong(t *testing.T) {
	if _, err := PreviewImport(context.Background(), ImportKindDocuments, "p1", strings.NewReader("a,b\n1,2\n")); err == nil || !strings.Contains(err.Error(), "header") {
		t.Errorf("err = %v", err)
	}
	if _, err := PreviewImport(context.Background(), "pdf", "p1", strings.NewReader("x")); err == nil {
		t.Error("unsupported kind must be an error")
	}
}

func columnNames(columns []ImportColumn) []string {
	out := make([]string, 0, len(columns))
	for _, c := range columns {
		out = append(out, c.Name)
	}
	return out
}
