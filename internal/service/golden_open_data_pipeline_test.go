//go:build golden

package service

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository/sqlite"
)

func TestGoldenRealOpenDataRunsThroughDeterministicPipeline(t *testing.T) {
	path := filepath.Join("..", "..", "reports", "japan-company-count-2021-2024", "normalized.csv")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open real Open Data fixture: %v", err)
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil || len(rows) < 2 {
		t.Fatalf("read normalized open data: rows=%d err=%v", len(rows), err)
	}
	header := map[string]int{}
	for i, name := range rows[0] {
		header[name] = i
	}

	type selectedRow struct {
		metricID, period, value, populationScope, populationDefinitionID, harmonizationMethod, sourceName, sourceURL string
	}
	var selected []selectedRow
	for _, row := range rows[1:] {
		metric := row[header["metric_id"]]
		if metric != "enterprise_equivalents" && metric != "enterprise_equivalents_harmonized" {
			continue
		}
		selected = append(selected, selectedRow{
			metricID: metric,
			period: row[header["period"]],
			value: row[header["value"]],
			populationScope: row[header["population_scope"]],
			populationDefinitionID: row[header["population_definition_id"]],
			harmonizationMethod: row[header["harmonization_method"]],
			sourceName: row[header["source_name"]],
			sourceURL: row[header["source_url"]],
		})
	}
	if len(selected) != 3 {
		t.Fatalf("expected 3 enterprise-equivalent rows from checked-in open data, got %d", len(selected))
	}

	docs := make([]*domain.Document, 0, len(selected))
	for i, row := range selected {
		manifest := AcquisitionManifest{
			SourceName: row.sourceName,
			SourceURL: row.sourceURL,
			DatasetID: row.metricID + "-" + row.period + "-" + row.populationDefinitionID,
			RetrievalMethod: RetrievalDownload,
			RetrievedAt: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
			Geography: "Japan",
			Period: row.period,
			Unit: "enterprises",
			PopulationScope: row.populationScope,
			PopulationDefinitionID: row.populationDefinitionID,
			KnownCaveats: []string{"golden fixture mirrors checked-in normalized public data"},
			SchemaID: "japan-economic-census-enterprise-equivalents",
			SchemaVersion: row.period,
			RecipeRef: "reports/japan-company-count-2021-2024/ACQUISITION.md",
			FileHash: "golden-open-data",
		}
		meta := manifest.DocumentMetadata(map[string]string{
			"period": row.period,
			"event_type": "enterprise_equivalents",
			"location": "Japan",
			"record_count": row.value,
			"source_provider": "statistics_bureau",
			"source_version": "economic-census",
		})
		first := "Dataset observation: period=" + row.period + "; location=Japan; event_type=enterprise_equivalents; record_count=" + row.value + "."
		docs = append(docs, &domain.Document{
			ID: "golden-open-data-" + string(rune('a'+i)),
			Source: domain.SourceDataset,
			Title: row.metricID + " " + row.period,
			Content: first + " Source: " + row.sourceName + ".",
			Metadata: meta,
			CreatedAt: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
		})
	}

	pipeline, db, project := newTestPipelineWith(t, nil, docs)
	metrics, err := pipeline.Run(context.Background(), testAnalysisID, project.ID, nil)
	if err != nil {
		t.Fatalf("deterministic pipeline over real Open Data: %v", err)
	}
	if metrics.Provenance.Mode != ExecutionModeDeterministic {
		t.Fatalf("real open-data golden must remain deterministic, got %+v", metrics.Provenance)
	}
	if metrics.Provenance.DeterministicObservations != 3 {
		t.Fatalf("expected 3 deterministic observations, got %+v", metrics.Provenance)
	}
	if metrics.Provenance.DeterministicComparisons != 1 {
		t.Fatalf("population-definition mismatch must prevent the naive comparison; got %+v", metrics.Provenance)
	}
	if len(metrics.Provenance.CompatibilityWarnings) == 0 {
		t.Fatal("population-definition mismatch must remain visible in provenance warnings")
	}

	patterns, err := sqlite.NewPatternRepository(db).ListByProject(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) != 1 {
		t.Fatalf("expected one harmonized comparison pattern, got %+v", patterns)
	}
	desc := patterns[0].Description
	if !strings.Contains(desc, "record_count 3684049 → 3352019") ||
		!strings.Contains(desc, "delta -332030") ||
		!strings.Contains(desc, "rate -9.0%") {
		t.Fatalf("harmonized open-data comparison changed unexpectedly: %s", desc)
	}
}
