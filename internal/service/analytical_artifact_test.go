package service

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/analytical"
	"insight-lab/internal/domain"
)

func loadArtifactFixture(t *testing.T, name string) *analytical.Artifact {
	t.Helper()
	data, err := os.ReadFile("../../contracts/analytical-artifact/v1/fixtures/" + name)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := analytical.Import(data)
	if err != nil {
		t.Fatal(err)
	}
	return artifact
}

func TestAnalyticalArtifactDocumentCarriesEveryResultStatementAndItsIdentity(t *testing.T) {
	artifact := loadArtifactFixture(t, "trade-time-series.json")
	doc, err := AnalyticalArtifactDocument("proj_1", *artifact, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := analytical.ToCandidates(*artifact)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Source != domain.SourceDataset || doc.Metadata[MetadataAnalyticalArtifactID] != artifact.ID || doc.Metadata[MetadataAnalyticalReproducibilityKey] != artifact.ReproducibilityKey() {
		t.Fatalf("document identity = %+v", doc.Metadata)
	}
	for _, c := range candidates {
		if !strings.Contains(doc.Content, c.Observation.Quote) {
			t.Errorf("document content is missing result statement %q", c.Observation.Quote)
		}
	}
}

func TestPreAnalysisMaterializesAnalyticalArtifactResultsAsGroundedObservations(t *testing.T) {
	artifact := loadArtifactFixture(t, "trade-time-series.json")
	doc, err := AnalyticalArtifactDocument("proj_1", *artifact, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	doc.ID = "doc_artifact"
	pre := RunDatasetPreAnalysis([]*domain.Document{doc}, time.Unix(2, 0))

	if len(pre.Observations) != len(artifact.Results) {
		t.Fatalf("observations = %d, want one per result (%d); notes %v", len(pre.Observations), len(artifact.Results), pre.Notes)
	}
	for _, o := range pre.Observations {
		if o.DocumentID != "doc_artifact" || doc.Content[runeOffsetToByte(doc.Content, o.StartOffset):runeOffsetToByte(doc.Content, o.EndOffset)] != o.Quote {
			t.Fatalf("observation is not grounded in the artifact document: %+v", o)
		}
	}
	if !pre.Handled("doc_artifact") {
		t.Fatal("an artifact document must not be sent to the model for extraction")
	}
}

func TestDeterministicRunAnalyzesAnArtifactOnlyProjectWithoutAModel(t *testing.T) {
	artifact := loadArtifactFixture(t, "external-consumer.json")
	doc, err := AnalyticalArtifactDocument("proj_1", *artifact, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	pipeline, _, project := newTestPipelineWith(t, nil, []*domain.Document{doc})
	metrics, err := pipeline.Run(context.Background(), testAnalysisID, project.ID, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if metrics.Provenance.DeterministicObservations != len(artifact.Results) {
		t.Fatalf("deterministic observations = %d, want %d", metrics.Provenance.DeterministicObservations, len(artifact.Results))
	}
}

func runeOffsetToByte(s string, runeOffset int) int {
	n := 0
	for i := range s {
		if n == runeOffset {
			return i
		}
		n++
	}
	return len(s)
}
