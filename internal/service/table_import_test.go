package service

import (
	"context"
	"strings"
	"testing"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository/sqlite"
)

func TestParseTableDetectsDelimiterAndPadsRows(t *testing.T) {
	tsv := "\uFEFFid\tテキスト\t評価\n1\tとても良い\t5\n2\t短い\n"
	tb, err := ParseTable(strings.NewReader(tsv))
	if err != nil {
		t.Fatal(err)
	}
	if tb.Delimiter != '\t' || len(tb.Headers) != 3 || tb.Headers[0] != "id" {
		t.Errorf("headers=%v delimiter=%q", tb.Headers, tb.Delimiter)
	}
	if len(tb.Rows) != 2 || len(tb.Rows[1]) != 3 || tb.Rows[1][2] != "" {
		t.Errorf("rows not padded: %v", tb.Rows)
	}
	if _, err := ParseTable(strings.NewReader("   \n")); err == nil {
		t.Error("empty file should error")
	}
}

func TestSuggestMappingJapaneseSurveyExport(t *testing.T) {
	csvData := "回答ID,回答日,役職,従業員数,満足度,自由記述\n1,2026-08-01,経理担当,30名,3,検算に時間がかかる\n"
	tb, err := ParseTable(strings.NewReader(csvData))
	if err != nil {
		t.Fatal(err)
	}
	m, guessed := SuggestMapping(tb)
	if guessed {
		t.Error("自由記述 is a known content header; must not be a guess")
	}
	if m.ContentColumn != "自由記述" {
		t.Errorf("ContentColumn = %q", m.ContentColumn)
	}
	want := map[string]string{"回答日": domain.MetaDate, "役職": domain.MetaRole, "従業員数": domain.MetaCompanySize, "満足度": domain.MetaRating}
	for col, key := range want {
		if m.MetadataColumns[col] != key {
			t.Errorf("MetadataColumns[%q] = %q, want %q", col, m.MetadataColumns[col], key)
		}
	}
	if m.MetadataColumns["回答ID"] != domain.MetaParticipantID {
		t.Errorf("回答ID should map to participant_id (weak match), got %q", m.MetadataColumns["回答ID"])
	}
	if m.SourceColumn != "" || m.DefaultSource != domain.SourceInterview {
		t.Errorf("no source column: DefaultSource should be proposed, got %q/%q", m.SourceColumn, m.DefaultSource)
	}
}

func TestSuggestMappingLegacyAndFallback(t *testing.T) {
	tb, _ := ParseTable(strings.NewReader("id,source,title,content\n1,interview,t,c\n"))
	m, guessed := SuggestMapping(tb)
	if guessed || m.ContentColumn != "content" || m.TitleColumn != "title" || m.IDColumn != "id" || m.SourceColumn != "source" {
		t.Errorf("legacy layout mapping = %+v", m)
	}

	// No recognizable header: the column with the most text is proposed.
	tb, _ = ParseTable(strings.NewReader("a,b,c\n1,x,毎月月末になると請求書の作成に半日かかります\n2,y,検算に時間がかかります\n"))
	m, guessed = SuggestMapping(tb)
	if !guessed || m.ContentColumn != "c" {
		t.Errorf("fallback mapping = %+v guessed=%v", m, guessed)
	}
}

func TestImportTableWithMappingMetadataAndTranscriptCells(t *testing.T) {
	db, err := sqlite.Open(t.TempDir() + "/import.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	p := &domain.Project{ID: "proj_1", Name: "p"}
	if err := sqlite.NewProjectRepository(db).Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	documents := sqlite.NewDocumentRepository(db)

	csvData := "チケットID,種別,役職,本文\n" +
		"T-1,support,経理担当,\"顧客: PDFが文字化けします。\nサポート: ブラウザは何ですか？\n顧客: Safariです。急いでいます。\"\n" +
		"T-2,,営業事務,月150件発行しているので自動化したいが、目視確認はやめられない。\n" +
		"T-3,bogus,x,本文あり\n" +
		"T-4,support,x,\n"
	tb, err := ParseTable(strings.NewReader(csvData))
	if err != nil {
		t.Fatal(err)
	}
	mapping := domain.ColumnMapping{
		ContentColumn: "本文", IDColumn: "チケットID", SourceColumn: "種別", DefaultSource: domain.SourceSupport,
		MetadataColumns: map[string]string{"役職": domain.MetaRole},
	}
	masked := 0
	res, err := ImportTable(ctx, documents, p.ID, tb, mapping, ImportOptions{
		Masker: func(s string) (string, string, int) {
			if strings.Contains(s, "Safari") {
				masked++
				return strings.ReplaceAll(s, "Safari", "[ブラウザ]"), s, 1
			}
			return s, "", 0
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Imported != 2 || res.Skipped != 2 || len(res.Errors) != 2 {
		t.Fatalf("result = %+v", res)
	}
	if res.WithSpeakers != 1 || res.Masked != 1 {
		t.Errorf("WithSpeakers=%d Masked=%d, want 1 and 1", res.WithSpeakers, res.Masked)
	}

	docs, err := documents.ListByProject(ctx, p.ID)
	if err != nil || len(docs) != 2 {
		t.Fatalf("docs = %v, %v", docs, err)
	}
	var thread, note *domain.Document
	for _, d := range docs {
		if d.Metadata["csv_id"] == "T-1" {
			thread = d
		} else {
			note = d
		}
	}
	if thread == nil || note == nil {
		t.Fatalf("missing docs: %+v", docs)
	}
	if thread.Source != domain.SourceSupport || thread.Metadata[domain.MetaRole] != "経理担当" {
		t.Errorf("thread = %+v", thread)
	}
	if len(thread.Spans) != 3 || thread.Spans[1].Role != domain.RoleAgent {
		t.Errorf("ticket thread should have speaker spans: %+v", thread.Spans)
	}
	if !strings.Contains(thread.Content, "[ブラウザ]") || !strings.Contains(thread.RawContent, "Safari") {
		t.Errorf("masking not applied: content=%q raw=%q", thread.Content, thread.RawContent)
	}
	// Spans were computed on the masked content, so they must index it.
	runes := []rune(thread.Content)
	last := thread.Spans[2]
	if got := string(runes[last.Start:last.End]); got != "[ブラウザ]です。急いでいます。" {
		t.Errorf("span text = %q", got)
	}
	if note.Source != domain.SourceSupport || note.Provenance != domain.ProvenanceFirsthand || note.RawContent != "" || note.Spans != nil {
		t.Errorf("note = %+v", note)
	}

	if _, err := ImportTable(ctx, documents, p.ID, tb, domain.ColumnMapping{ContentColumn: "存在しない"}, ImportOptions{}); err == nil {
		t.Error("unknown content column should error")
	}
	if _, err := ImportTable(ctx, documents, p.ID, tb, domain.ColumnMapping{ContentColumn: "本文"}, ImportOptions{}); err == nil {
		t.Error("no source column and no default source should error")
	}
}
