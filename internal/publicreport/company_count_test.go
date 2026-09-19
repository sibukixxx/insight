package publicreport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testCSV = `geography,year,enterprise_count
全国,2012,3863530
全国,2021,3375255
東京都,2012,447113
東京都,2021,423595
`

func TestAnalyzeCompanyCounts(t *testing.T) {
	records, err := LoadRecords(strings.NewReader(testCSV))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Analyze(records)
	if err != nil {
		t.Fatal(err)
	}
	if got.National.Delta != -488275 {
		t.Fatalf("national delta = %d", got.National.Delta)
	}
	if got.Tokyo.Delta != -23518 {
		t.Fatalf("Tokyo delta = %d", got.Tokyo.Delta)
	}
	if got.National.PercentChange > -12.63 || got.National.PercentChange < -12.65 {
		t.Fatalf("national percent = %f", got.National.PercentChange)
	}
	if got.TokyoShareFrom < 11.57 || got.TokyoShareFrom > 11.58 {
		t.Fatalf("Tokyo share 2012 = %f", got.TokyoShareFrom)
	}
	if got.TokyoShareTo < 12.55 || got.TokyoShareTo > 12.56 {
		t.Fatalf("Tokyo share 2021 = %f", got.TokyoShareTo)
	}
	if got.TokyoShareDeltaPP < 0.97 || got.TokyoShareDeltaPP > 0.99 {
		t.Fatalf("Tokyo share delta = %f", got.TokyoShareDeltaPP)
	}
}

func TestGenerateProducesTraceableArtifacts(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "source_extract.csv")
	meta := filepath.Join(dir, "source_metadata.json")
	out := filepath.Join(dir, "generated")
	if err := os.WriteFile(input, []byte(testCSV), 0o644); err != nil {
		t.Fatal(err)
	}
	metadata := `{
  "source_name": "fixture source",
  "source_url": "https://example.test/source",
  "publisher": "fixture publisher",
  "retrieved_at": "2026-09-18",
  "coverage_period": "2012, 2021",
  "geographic_scope": "Japan",
  "unit": "enterprises",
  "license_terms": "fixture only",
  "known_limitations": ["fixture limitation"]
}`
	if err := os.WriteFile(meta, []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Generate(input, meta, out); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"normalized.csv", "analysis.json", "evidence-ledger.json",
		"report.md", "note-draft-source.md", "sns-summary.md", "insight-import.csv",
	} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Fatalf("%s not generated: %v", name, err)
		}
	}
	report, err := os.ReadFile(filepath.Join(out, "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"3,863,530", "3,375,255", "12.55%", "反対の証拠", "Methodology"} {
		if !strings.Contains(string(report), want) {
			t.Fatalf("report missing %q", want)
		}
	}
	ledger, err := os.ReadFile(filepath.Join(out, "evidence-ledger.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ledger), "tokyo-national-share-2012-2021") {
		t.Fatal("ledger does not contain Tokyo share claim")
	}
}
