package usecase

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/service"
)

func metricsJSON(t *testing.T, prov service.RunProvenance) string {
	t.Helper()
	encoded, err := json.Marshal(service.Metrics{Provenance: prov})
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestGetResearchArtifactCarriesProvenanceAndLatestIterationLosslessly(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	first := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)
	app.now = func() time.Time { return first }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "policy", CreatedAt: first}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p1", "a1", first, []*domain.Insight{
		{ID: "h1", Title: "Policy effect", SurprisingFact: "designations rose", ValidationStatus: domain.ValidationPlausible, IdentificationStatus: domain.IdentificationNotIdentified, MissingEvidence: []string{"comparison trend before treatment"}},
	})
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "did the policy cause it?", InputReferences: []string{"policy.csv"}})
	if err != nil {
		t.Fatal(err)
	}

	app.now = func() time.Time { return second }
	prov := service.RunProvenance{
		Mode: service.AnalysisModeModelBacked, Model: "gpt-5", PromptFingerprint: "sha256:abc", RuleVersion: "dataset-preanalysis/v1",
		DatasetHashes: []string{"hash-1"},
		Datasets:      []service.DatasetProvenance{{SourceName: "e-Stat", DatasetID: "0001", RetrievalMethod: service.RetrievalAPI, SchemaID: "estat-v1"}},
	}
	if err := app.repos.Analyses.Create(ctx, &domain.Analysis{ID: "a2", ProjectID: "p1", Status: domain.AnalysisCompleted, CreatedAt: second, Metrics: metricsJSON(t, prov)}); err != nil {
		t.Fatal(err)
	}
	analysisID := "a2"
	if err := app.repos.Insights.Create(ctx, &domain.Insight{
		ID: "h1b", ProjectID: "p1", AnalysisID: &analysisID, Title: "Policy effect", SurprisingFact: "designations rose",
		ValidationStatus: domain.ValidationSupported, IdentificationStatus: domain.IdentificationNotIdentified, CreatedAt: second,
	}); err != nil {
		t.Fatal(err)
	}

	got, err := app.AppendResearchIteration(ctx, AppendResearchIterationInput{RunID: run.ID, AddedEvidence: []string{"comparison series"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Stop == nil {
		t.Fatalf("expected the loop to stop on this iteration: %+v", got)
	}

	artifact, err := app.GetResearchArtifact(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}

	if artifact.ArtifactSchema != ResearchArtifactSchema || artifact.SchemaVersion != ResearchArtifactVersion {
		t.Fatalf("artifact must declare its schema and version: %+v", artifact)
	}
	if artifact.ProjectID != "p1" || artifact.ResearchRunID != run.ID || artifact.IterationID != got.ID || artifact.ResearchQuestion != "did the policy cause it?" {
		t.Fatalf("artifact must identify the run and iteration it snapshots: %+v", artifact)
	}
	if artifact.AnalysisMode != service.AnalysisModeModelBacked || artifact.ModelVersion != "gpt-5" || artifact.PromptFingerprint != "sha256:abc" || artifact.RuleVersion != "dataset-preanalysis/v1" {
		t.Fatalf("artifact must carry model/rule provenance so a no-model run cannot look model-backed: %+v", artifact)
	}
	if len(artifact.DatasetHashes) != 1 || artifact.DatasetHashes[0] != "hash-1" {
		t.Fatalf("artifact must carry dataset hashes: %+v", artifact.DatasetHashes)
	}
	if len(artifact.AcquisitionManifests) != 1 || artifact.AcquisitionManifests[0].DatasetID != "0001" {
		t.Fatalf("artifact must carry acquisition manifest references: %+v", artifact.AcquisitionManifests)
	}
	if len(artifact.Insights) != 1 || artifact.Insights[0].ID != "h1b" || artifact.Insights[0].ValidationStatus != domain.ValidationSupported {
		t.Fatalf("artifact must resolve the latest iteration's insights, not every insight ever created: %+v", artifact.Insights)
	}
	if len(got.ResearchGaps) == 0 || len(artifact.ResearchGaps) != len(got.ResearchGaps) || artifact.ResearchGaps[0].ID != got.ResearchGaps[0].ID {
		t.Fatalf("artifact must not drop unresolved/resolved research gaps: iteration=%+v artifact=%+v", got.ResearchGaps, artifact.ResearchGaps)
	}
	if artifact.Readiness.State != got.Readiness.State || artifact.EffectiveReadiness != got.EffectiveReadiness() {
		t.Fatalf("artifact must carry the exact readiness assessment: %+v", artifact)
	}
	if artifact.StopDecision == nil || artifact.StopDecision.Reason != got.Stop.Reason {
		t.Fatalf("artifact must carry the stop decision: %+v", artifact.StopDecision)
	}
	if artifact.CreatedAt != got.CreatedAt || artifact.ExportedAt != second {
		t.Fatalf("artifact must record when the iteration happened and when it was exported: %+v", artifact)
	}
	if artifact.ResearchStage != got.Stage {
		t.Fatalf("artifact must carry the latest iteration's research stage: artifact=%q iteration=%q", artifact.ResearchStage, got.Stage)
	}

	encoded, err := json.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip map[string]any
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"artifactSchema", "schemaVersion", "researchRunId", "analysisMode", "decisionReadiness", "insights", "researchStage"} {
		if _, ok := roundTrip[field]; !ok {
			t.Errorf("exported JSON is missing field %q for downstream consumption: %s", field, encoded)
		}
	}
}

// TestResearchArtifactStageAgreesWithReportStage guards against the artifact
// and the markdown report reading the run's stage through two different
// paths that could silently diverge.
func TestResearchArtifactStageAgreesWithReportStage(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return now }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "policy", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	seedAnalysis(t, app, ctx, "p1", "a1", now, []*domain.Insight{
		{ID: "h1", Title: "Policy effect", SurprisingFact: "designations rose"},
	})
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", Question: "did the policy cause it?"})
	if err != nil {
		t.Fatal(err)
	}

	artifact, err := app.GetResearchArtifact(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}

	persistedRun, err := app.repos.Research.GetResearchRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantStage := reportStage(ProjectReport{ResearchRun: persistedRun})
	if artifact.ResearchStage != wantStage {
		t.Fatalf("artifact research stage %q disagrees with report stage %q", artifact.ResearchStage, wantStage)
	}
}

func TestGetResearchArtifactReturnsNotFoundForUnknownRun(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	if _, err := app.GetResearchArtifact(ctx, "missing-run"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
