package service

import (
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
)

func validManifest() AcquisitionManifest {
	return AcquisitionManifest{
		SourceName:      "e-Stat",
		SourceURL:       "https://www.e-stat.go.jp/stat-search/files?stat_infid=000032143614",
		DatasetID:       "000032143614",
		RetrievalMethod: RetrievalDownload,
		RetrievedAt:     time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC),
		QueryParameters: map[string]string{"area": "13229", "year": "2021"},
		Geography:       "西東京市",
		Period:          "2021",
		Unit:            "enterprises",
		PopulationScope: "all private enterprises (economic census)",
		KnownCaveats:    []string{"2021 census counts differ from 2024 preliminary population"},
		SchemaID:        "estat-economic-census-enterprise",
		SchemaVersion:   "2021",
		RecipeRef:       "recipes/estat-economic-census.md",
	}
}

func TestAcquisitionManifestValidateAcceptsCompleteManifest(t *testing.T) {
	if err := validManifest().Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestAcquisitionManifestValidateRejectsMissingRequiredFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*AcquisitionManifest)
		want   string
	}{
		{"source name", func(m *AcquisitionManifest) { m.SourceName = " " }, "sourceName is required"},
		{"dataset id", func(m *AcquisitionManifest) { m.DatasetID = "" }, "datasetId is required"},
		{"retrieval method", func(m *AcquisitionManifest) { m.RetrievalMethod = "" }, "retrievalMethod is required"},
		{"unknown retrieval method", func(m *AcquisitionManifest) { m.RetrievalMethod = "telepathy" }, "retrievalMethod must be one of"},
		{"retrieved at", func(m *AcquisitionManifest) { m.RetrievedAt = time.Time{} }, "retrievedAt is required"},
		{"schema id", func(m *AcquisitionManifest) { m.SchemaID = "" }, "schemaId is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifest()
			tc.mutate(&m)
			err := m.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() = %v, want error containing %q", err, tc.want)
			}
		})
	}
}

func TestAcquisitionManifestValidateRejectsCredentialsInQueryParameters(t *testing.T) {
	for _, key := range []string{"appId", "api_key", "X-Token", "secret", "password", "Authorization"} {
		t.Run(key, func(t *testing.T) {
			m := validManifest()
			m.QueryParameters = map[string]string{key: "abc"}
			err := m.Validate()
			if err == nil || !strings.Contains(err.Error(), "must not contain credentials") {
				t.Fatalf("Validate() = %v, want credential rejection for %q", err, key)
			}
		})
	}
}

func TestAcquisitionManifestValidateRejectsCredentialsInSourceURL(t *testing.T) {
	m := validManifest()
	m.SourceURL = "https://api.houjin-bangou.nta.go.jp/4/name?id=SECRETAPPID&name=x"
	err := m.Validate()
	if err == nil || !strings.Contains(err.Error(), "must not contain credentials") {
		t.Fatalf("Validate() = %v, want credential rejection in URL", err)
	}
}

func TestParseAcquisitionManifestDecodesJSONAndValidates(t *testing.T) {
	input := `{"sourceName":"e-Stat","datasetId":"000032143614","retrievalMethod":"download",
		"retrievedAt":"2026-09-01T09:00:00Z","schemaId":"estat-economic-census-enterprise","unit":"enterprises"}`

	m, err := ParseAcquisitionManifest(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseAcquisitionManifest: %v", err)
	}
	if m.SourceName != "e-Stat" || m.Unit != "enterprises" || m.RetrievedAt.Year() != 2026 {
		t.Fatalf("unexpected manifest: %+v", m)
	}

	if _, err := ParseAcquisitionManifest(strings.NewReader(`{"sourceName":"x"}`)); err == nil {
		t.Fatal("expected validation error for an incomplete manifest")
	}
	if _, err := ParseAcquisitionManifest(strings.NewReader(`not json`)); err == nil {
		t.Fatal("expected decode error for malformed JSON")
	}
}

func TestAcquisitionManifestRoundTripsThroughDocumentMetadata(t *testing.T) {
	m := validManifest()
	m.FileHash = "abc123"

	meta := m.DocumentMetadata(map[string]string{"record_count": "2"})
	if meta["record_count"] != "2" {
		t.Errorf("existing metadata must be preserved, got %v", meta)
	}
	if meta[MetadataDatasetHash] != "abc123" {
		t.Errorf("file hash must be exposed as %s, got %v", MetadataDatasetHash, meta)
	}
	doc := &domain.Document{Source: domain.SourceDataset, Metadata: meta}

	got, ok := ManifestFromDocument(doc)
	if !ok {
		t.Fatalf("ManifestFromDocument: manifest not found in %v", meta)
	}
	if got.SourceName != m.SourceName || got.DatasetID != m.DatasetID || got.Unit != m.Unit ||
		got.PopulationScope != m.PopulationScope || got.FileHash != "abc123" || !got.RetrievedAt.Equal(m.RetrievedAt) ||
		len(got.KnownCaveats) != 1 || got.QueryParameters["area"] != "13229" {
		t.Fatalf("manifest did not round-trip: got %+v want %+v", got, m)
	}

	if _, ok := ManifestFromDocument(&domain.Document{Metadata: map[string]string{"csv_id": "1"}}); ok {
		t.Error("a document without a manifest must report ok=false")
	}
}

func TestCheckDatasetCompatibilityReportsNothingForSingleOrIdenticalDatasets(t *testing.T) {
	a := validManifest()
	b := validManifest()
	b.DatasetID = "000032143615"
	b.Period = "2024"

	if warnings := CheckDatasetCompatibility([]AcquisitionManifest{a}); len(warnings) != 0 {
		t.Errorf("single dataset should be compatible with itself, got %+v", warnings)
	}
	if warnings := CheckDatasetCompatibility([]AcquisitionManifest{a, b}); len(warnings) != 0 {
		t.Errorf("same unit/population/granularity should be compatible, got %+v", warnings)
	}
}

func TestCheckDatasetCompatibilityFlagsUnitPopulationPeriodAndSchemaMismatch(t *testing.T) {
	a := validManifest()
	b := validManifest()
	b.DatasetID = "000032143615"
	b.Unit = "establishments"
	b.PopulationScope = "establishments with employees"
	b.Period = "2024-06"
	b.SchemaVersion = "2024"

	warnings := CheckDatasetCompatibility([]AcquisitionManifest{a, b})

	got := map[CompatibilityCode]DatasetCompatibilityWarning{}
	for _, w := range warnings {
		got[w.Code] = w
	}
	for _, code := range []CompatibilityCode{CompatibilityUnitMismatch, CompatibilityPopulationMismatch, CompatibilityPeriodGranularityMismatch, CompatibilitySchemaVersionMismatch} {
		w, ok := got[code]
		if !ok {
			t.Errorf("missing warning %s in %+v", code, warnings)
			continue
		}
		if len(w.DatasetIDs) != 2 || w.DatasetIDs[0] != "000032143614" || w.DatasetIDs[1] != "000032143615" {
			t.Errorf("%s should cite both datasets in stable order, got %v", code, w.DatasetIDs)
		}
		if w.Detail == "" {
			t.Errorf("%s should carry a human-readable detail", code)
		}
	}
	if len(warnings) != 4 {
		t.Errorf("want exactly 4 warnings, got %d: %+v", len(warnings), warnings)
	}
}

func TestCheckDatasetCompatibilityIgnoresUnspecifiedValues(t *testing.T) {
	a := validManifest()
	b := validManifest()
	b.DatasetID = "other"
	b.Unit = ""
	b.PopulationScope = ""
	b.Period = ""

	if warnings := CheckDatasetCompatibility([]AcquisitionManifest{a, b}); len(warnings) != 0 {
		t.Errorf("unspecified values are unknown, not incompatible; got %+v", warnings)
	}
}

func TestPeriodGranularityClassifiesCommonPeriodShapes(t *testing.T) {
	cases := map[string]string{
		"2021": "year", "2024-06": "month", "2026-01-15": "day",
		"2021..2024": "year", "2026-01/2026-06": "month", "FY2021": "unknown", "": "unknown",
	}
	for input, want := range cases {
		if got := periodGranularity(input); got != want {
			t.Errorf("periodGranularity(%q) = %q, want %q", input, got, want)
		}
	}
}
