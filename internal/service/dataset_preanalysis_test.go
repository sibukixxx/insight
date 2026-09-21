package service

import (
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
)

func datasetDoc(id, period, city, eventType string, count string, extra map[string]string) *domain.Document {
	meta := map[string]string{
		"adapter": "corporate-event-analysis-csv", "period": period, "event_type": eventType,
		"prefecture_name": "サンプル都", "city_name": city, "record_count": count,
		"source_provider": "sample_registry", "source_version": "v4",
	}
	for k, v := range extra {
		meta[k] = v
	}
	location := "サンプル都 " + city
	return &domain.Document{
		ID: id, ProjectID: "proj_1", Source: domain.SourceDataset,
		Title: period + " " + location + " " + eventType,
		Content: "Dataset observation: period=" + period + "; location=" + location + "; event_type=" + eventType +
			"; record_count=" + count + "; source_provider=sample_registry; source_version=v4. The count represents exported administrative records classified by the source adapter. It does not by itself represent company founding, business commencement, policy effect, or causal impact.",
		Metadata:  meta,
		CreatedAt: time.Now().UTC(),
	}
}

func TestMaterializeDatasetObservationsGroundsFirstSentenceOfCountableDatasetDocuments(t *testing.T) {
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	docs := []*domain.Document{
		datasetDoc("doc_a", "2026-01", "サンプル市", "ASSIGNED", "2", nil),
		{ID: "doc_interview", Source: domain.SourceInterview, Content: "設定を間違えたら怖いんですよね。"},
		{ID: "doc_no_count", Source: domain.SourceDataset, Content: "Dataset observation: something.", Metadata: map[string]string{"period": "2026-01"}},
		{ID: "doc_bad_count", Source: domain.SourceDataset, Content: "Dataset observation: x.", Metadata: map[string]string{"record_count": "many"}},
	}

	observations, notes := MaterializeDatasetObservations(docs, now)

	if len(observations) != 1 {
		t.Fatalf("observations = %d, want 1 (only the countable dataset document): %+v", len(observations), observations)
	}
	obs := observations[0]
	wantQuote := "Dataset observation: period=2026-01; location=サンプル都 サンプル市; event_type=ASSIGNED; record_count=2; source_provider=sample_registry; source_version=v4."
	if obs.Quote != wantQuote {
		t.Errorf("Quote = %q, want %q", obs.Quote, wantQuote)
	}
	if obs.DocumentID != "doc_a" || obs.StartOffset != 0 || obs.EndOffset != len([]rune(wantQuote)) {
		t.Errorf("observation not grounded at the start of the document: %+v", obs)
	}
	if obs.Behavior != "record_count=2 (ASSIGNED, 2026-01, サンプル都 サンプル市)" {
		t.Errorf("Behavior = %q", obs.Behavior)
	}
	if obs.Topic != "ASSIGNED" || !obs.CreatedAt.Equal(now) || !strings.HasPrefix(obs.ID, "obs_") {
		t.Errorf("unexpected observation fields: %+v", obs)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "doc_bad_count") {
		t.Errorf("a dataset document with an unparsable count must be reported, got %v", notes)
	}
}

func TestComputeDatasetComparisonsCalculatesDeltaRateBaselineAndShare(t *testing.T) {
	docs := []*domain.Document{
		datasetDoc("doc_mar", "2026-03", "サンプル市", "ASSIGNED", "3", nil),
		datasetDoc("doc_jan", "2026-01", "サンプル市", "ASSIGNED", "2", nil),
		datasetDoc("doc_feb", "2026-02", "サンプル市", "ASSIGNED", "5", nil),
		datasetDoc("doc_other_city", "2026-01", "武蔵野市", "ASSIGNED", "9", nil),
	}

	comparisons, notes := ComputeDatasetComparisons(docs)

	if len(notes) != 0 {
		t.Errorf("unexpected notes: %v", notes)
	}
	if len(comparisons) != 2 {
		t.Fatalf("comparisons = %d, want 2 consecutive steps for the single multi-period series: %+v", len(comparisons), comparisons)
	}
	first, second := comparisons[0], comparisons[1]
	if first.FromDocumentID != "doc_jan" || first.ToDocumentID != "doc_feb" || first.FromPeriod != "2026-01" || first.ToPeriod != "2026-02" {
		t.Errorf("first step should run 2026-01 -> 2026-02: %+v", first)
	}
	if first.FromValue != 2 || first.ToValue != 5 || first.Delta != 3 || first.RateOfChange == nil || *first.RateOfChange != 1.5 {
		t.Errorf("first step arithmetic wrong: %+v", first)
	}
	if first.BaselinePeriod != "2026-01" || first.BaselineValue != 2 || first.DeltaFromBaseline != 3 {
		t.Errorf("baseline should be the earliest period: %+v", first)
	}
	if first.ShareOfSeriesTotal != 0.5 { // 5 / (2+5+3)
		t.Errorf("ShareOfSeriesTotal = %f, want 0.5", first.ShareOfSeriesTotal)
	}
	if second.FromDocumentID != "doc_feb" || second.ToDocumentID != "doc_mar" || second.Delta != -2 || second.DeltaFromBaseline != 1 || *second.RateOfChange != -0.4 {
		t.Errorf("second step arithmetic wrong: %+v", second)
	}
	if first.Series != "ASSIGNED / サンプル都 サンプル市 / sample_registry v4" {
		t.Errorf("Series label = %q", first.Series)
	}
}

func TestComputeDatasetComparisonsLeavesRateUndefinedWhenStartingFromZero(t *testing.T) {
	docs := []*domain.Document{
		datasetDoc("doc_jan", "2026-01", "サンプル市", "CLOSED", "0", nil),
		datasetDoc("doc_feb", "2026-02", "サンプル市", "CLOSED", "4", nil),
	}

	comparisons, _ := ComputeDatasetComparisons(docs)

	if len(comparisons) != 1 || comparisons[0].RateOfChange != nil || comparisons[0].Delta != 4 {
		t.Fatalf("rate must be undefined (not infinite or zero) when the start value is 0: %+v", comparisons)
	}
}

func TestComputeDatasetComparisonsDoesNotCompareAcrossIncompatibleUnitsOrPopulations(t *testing.T) {
	enterprises := validManifest()
	enterprises.FileHash = "h1"
	establishments := validManifest()
	establishments.DatasetID = "other"
	establishments.Unit = "establishments"
	establishments.FileHash = "h2"
	docs := []*domain.Document{
		datasetDoc("doc_2021", "2021", "サンプル市", "ENTERPRISES", "100", enterprises.DocumentMetadata(nil)),
		datasetDoc("doc_2024", "2024", "サンプル市", "ENTERPRISES", "130", establishments.DocumentMetadata(nil)),
	}

	comparisons, notes := ComputeDatasetComparisons(docs)

	if len(comparisons) != 0 {
		t.Fatalf("datasets with different units must not be compared: %+v", comparisons)
	}
	if len(notes) != 0 {
		t.Errorf("single-point series are ordinary, not notable: %v", notes)
	}
}

func TestComputeDatasetComparisonsSkipsSeriesWithDuplicatePeriods(t *testing.T) {
	docs := []*domain.Document{
		datasetDoc("doc_a", "2026-01", "サンプル市", "ASSIGNED", "2", nil),
		datasetDoc("doc_a_again", "2026-01", "サンプル市", "ASSIGNED", "2", nil),
		datasetDoc("doc_b", "2026-02", "サンプル市", "ASSIGNED", "5", nil),
	}

	comparisons, notes := ComputeDatasetComparisons(docs)

	if len(comparisons) != 0 {
		t.Errorf("a series imported twice is ambiguous and must not be compared: %+v", comparisons)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "2026-01") {
		t.Errorf("the duplicate period should be reported: %v", notes)
	}
}

func TestBuildComparisonPatternsCitesBothObservationsAndStatesTheNumbers(t *testing.T) {
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	docs := []*domain.Document{
		datasetDoc("doc_jan", "2026-01", "サンプル市", "ASSIGNED", "2", nil),
		datasetDoc("doc_feb", "2026-02", "サンプル市", "ASSIGNED", "5", nil),
	}
	observations, _ := MaterializeDatasetObservations(docs, now)
	comparisons, _ := ComputeDatasetComparisons(docs)

	patterns := buildComparisonPatterns("proj_1", "ana_1", comparisons, observations, now)

	if len(patterns) != 1 {
		t.Fatalf("patterns = %d, want 1", len(patterns))
	}
	p := patterns[0]
	if p.Kind != domain.PatternRepetition || p.ProjectID != "proj_1" || p.AnalysisID != "ana_1" || len(p.ObservationIDs) != 2 {
		t.Errorf("pattern shape wrong: %+v", p)
	}
	if p.Title != "Deterministic comparison: ASSIGNED / サンプル都 サンプル市 / sample_registry v4, 2026-01 → 2026-02" {
		t.Errorf("Title = %q", p.Title)
	}
	for _, want := range []string{"record_count 2 → 5", "delta +3", "rate +150.0%", "baseline 2026-01 = 2", "delta from baseline +3", "share of series total 71.4%", datasetPreAnalysisRuleVersion, "not an estimate"} {
		if !strings.Contains(p.Description, want) {
			t.Errorf("Description lacks %q: %s", want, p.Description)
		}
	}
}

func TestRunDatasetPreAnalysisCollectsProvenanceAndCompatibilityWarnings(t *testing.T) {
	a := validManifest()
	a.FileHash = "hash_a"
	b := validManifest()
	b.DatasetID = "000032143615"
	b.Unit = "establishments"
	b.FileHash = "hash_b"
	docs := []*domain.Document{
		datasetDoc("doc_1", "2021", "サンプル市", "ENTERPRISES", "100", a.DocumentMetadata(nil)),
		datasetDoc("doc_2", "2024", "サンプル市", "ENTERPRISES", "130", b.DocumentMetadata(nil)),
		datasetDoc("doc_3", "2026-01", "サンプル市", "ASSIGNED", "2", map[string]string{MetadataDatasetHash: "hash_c"}),
		{ID: "doc_interview", Source: domain.SourceInterview, Content: "怖い。"},
	}

	pre := RunDatasetPreAnalysis(docs, time.Now().UTC())

	if len(pre.Observations) != 3 || len(pre.Comparisons) != 0 {
		t.Errorf("observations=%d comparisons=%d, want 3 and 0", len(pre.Observations), len(pre.Comparisons))
	}
	if strings.Join(pre.DatasetHashes, ",") != "hash_a,hash_b,hash_c" {
		t.Errorf("DatasetHashes = %v, want sorted unique hashes", pre.DatasetHashes)
	}
	if len(pre.Manifests) != 2 {
		t.Errorf("Manifests = %d, want 2", len(pre.Manifests))
	}
	if len(pre.CompatibilityWarnings) != 1 || pre.CompatibilityWarnings[0].Code != CompatibilityUnitMismatch {
		t.Errorf("CompatibilityWarnings = %+v, want one unit mismatch", pre.CompatibilityWarnings)
	}
	if !pre.Handled("doc_1") || pre.Handled("doc_interview") {
		t.Error("Handled must report exactly the documents materialized deterministically")
	}
	if pre.RuleVersion != datasetPreAnalysisRuleVersion {
		t.Errorf("RuleVersion = %q", pre.RuleVersion)
	}
}
