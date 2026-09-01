package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository/sqlite"
)

func newTestDocumentRepo(t *testing.T) (*sqlite.DocumentRepository, *domain.Project) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "csv_test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	projects := sqlite.NewProjectRepository(db)
	ctx := context.Background()
	p := &domain.Project{ID: "proj_1", Name: "テスト", CreatedAt: time.Now().UTC()}
	if err := projects.Create(ctx, p); err != nil {
		t.Fatalf("create project: %v", err)
	}
	return sqlite.NewDocumentRepository(db), p
}

func TestImportCSVValidRows(t *testing.T) {
	documents, p := newTestDocumentRepo(t)
	csvData := "id,source,title,content\n" +
		"001,interview,Interview A,\"設定を間違えたら怖いです\"\n" +
		"002,review,Review 1,操作は簡単です\n"

	result, err := ImportCSV(context.Background(), documents, p.ID, strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("ImportCSV: %v", err)
	}
	if result.Imported != 2 || result.Skipped != 0 {
		t.Fatalf("result = %+v, want imported=2 skipped=0", result)
	}

	docs, err := documents.ListByProject(context.Background(), p.ID)
	if err != nil || len(docs) != 2 {
		t.Fatalf("ListByProject = %v, %v", docs, err)
	}
	found := false
	for _, d := range docs {
		if d.Metadata["csv_id"] == "001" && d.Source == domain.SourceInterview {
			found = true
		}
	}
	if !found {
		t.Error("expected a document with csv_id=001 and source=interview")
	}
}

func TestImportCSVStripsBOM(t *testing.T) {
	documents, p := newTestDocumentRepo(t)
	csvData := "\xEF\xBB\xBFid,source,title,content\n001,interview,A,本文です\n"

	result, err := ImportCSV(context.Background(), documents, p.ID, strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("ImportCSV: %v", err)
	}
	if result.Imported != 1 {
		t.Fatalf("Imported = %d, want 1", result.Imported)
	}
}

func TestImportCSVRejectsWrongHeader(t *testing.T) {
	documents, p := newTestDocumentRepo(t)
	csvData := "a,b,c,d\n1,2,3,4\n"

	if _, err := ImportCSV(context.Background(), documents, p.ID, strings.NewReader(csvData)); err == nil {
		t.Fatal("expected an error for a mismatched header")
	}
}

func TestImportCSVSkipsInvalidRowsButKeepsValidOnes(t *testing.T) {
	documents, p := newTestDocumentRepo(t)
	csvData := "id,source,title,content\n" +
		"001,interview,OK,有効な行です\n" +
		"002,not_a_source,Bad,不正なsourceです\n" +
		"003,review,Empty,\n"

	result, err := ImportCSV(context.Background(), documents, p.ID, strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("ImportCSV: %v", err)
	}
	if result.Imported != 1 {
		t.Errorf("Imported = %d, want 1", result.Imported)
	}
	if result.Skipped != 2 {
		t.Errorf("Skipped = %d, want 2", result.Skipped)
	}
	if len(result.Errors) != 2 {
		t.Errorf("Errors = %v, want 2 entries", result.Errors)
	}
}

func TestImportCSVEmptyFile(t *testing.T) {
	documents, p := newTestDocumentRepo(t)
	if _, err := ImportCSV(context.Background(), documents, p.ID, strings.NewReader("")); err == nil {
		t.Fatal("expected an error for an empty CSV")
	}
}
