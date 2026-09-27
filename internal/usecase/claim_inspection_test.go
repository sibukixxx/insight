package usecase

import (
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
)

// seedClaimResearch: one document and observation cited by a primary and a
// competing hypothesis of analysis a1.
func seedClaimResearch(t *testing.T, app *Application, fixed time.Time, validation domain.ValidationStatus, analysisID string, createdAt time.Time) {
	t.Helper()
	ctx := t.Context()
	obs := "obs-" + analysisID
	seedAnalysis(t, app, ctx, "p1", analysisID, createdAt, []*domain.Insight{
		{ID: "h-primary-" + analysisID, Title: "Campaign timing", HypothesisSetID: "set", HypothesisRole: domain.HypothesisPrimary, CausalStatus: domain.CausalObservedAssociation, ValidationStatus: validation, IdentificationStatus: domain.IdentificationNotIdentified},
	})
	if err := app.repos.Observations.CreateBatch(ctx, []*domain.Observation{{ID: obs, AnalysisID: analysisID, DocumentID: "doc-1", Quote: "Visits rose from 100 to 130.", CreatedAt: createdAt}}); err != nil {
		t.Fatal(err)
	}
	if err := app.repos.Evidence.CreateBatch(ctx, []*domain.Evidence{{ID: "ev-" + analysisID, InsightID: "h-primary-" + analysisID, DocumentID: "doc-1", ObservationID: &obs, Quote: "Visits rose from 100 to 130.", Type: domain.EvidenceSupport}}); err != nil {
		t.Fatal(err)
	}
}

func TestResearchRunInspectsClaimsAndReinspectsThemOnTheNextIteration(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	fixed := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return fixed }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "claims", CreatedAt: fixed}); err != nil {
		t.Fatal(err)
	}
	if err := app.repos.Documents.Create(ctx, &domain.Document{ID: "doc-1", ProjectID: "p1", Source: domain.SourceDocument, Content: "Visits rose from 100 to 130.", Metadata: map[string]string{"public_external_ref": "visits-report"}, CreatedAt: fixed}); err != nil {
		t.Fatal(err)
	}
	seedClaimResearch(t, app, fixed, domain.ValidationSupported, "a1", fixed)
	before, _ := app.repos.Observations.ListByProject(ctx, "p1")

	claims := []domain.ResearchClaim{
		{ID: "c-supported", Statement: "Visits were higher in the campaign month.", EvidenceReferences: []string{"visits-report"}},
		{ID: "c-fabricated", Statement: "A survey shows visits doubled.", EvidenceReferences: []string{"survey-2024"}},
		{ID: "c-causal", Statement: "The campaign caused the rise.", EvidenceReferences: []string{"visits-report"}},
	}
	run, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", AnalysisID: "a1", Question: "Did visits change?", Claims: claims})
	if err != nil {
		t.Fatal(err)
	}
	status := func(it domain.ResearchIteration) string {
		var parts []string
		for _, ci := range it.ClaimInspections {
			parts = append(parts, ci.ClaimID+"="+string(ci.Status))
		}
		return strings.Join(parts, ",")
	}
	if got := status(run.Iterations[0]); got != "c-supported=SUPPORTED,c-fabricated=UNKNOWN,c-causal=INSUFFICIENT" {
		t.Fatalf("first inspection = %s", got)
	}
	after, _ := app.repos.Observations.ListByProject(ctx, "p1")
	if len(after) != len(before) {
		t.Fatalf("claims created observations: %d -> %d", len(before), len(after))
	}
	artifact, err := app.GetResearchArtifact(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifact.ClaimInspections) != 3 {
		t.Fatalf("artifact must export the inspections: %+v", artifact.ClaimInspections)
	}

	report, err := app.ExportResearchMarkdown(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Claim inspection", "`c-fabricated` `UNKNOWN`; flags=[CITATION_NOT_FOUND]", "cannot conclude:"} {
		if !strings.Contains(string(report), want) {
			t.Fatalf("report missing %q", want)
		}
	}

	// New evidence contradicts the hypothesis; the next iteration inherits
	// the claims and records how each status moved.
	later := fixed.Add(time.Hour)
	seedClaimResearch(t, app, fixed, domain.ValidationContradicted, "a2", later)
	it, err := app.AppendResearchIteration(ctx, AppendResearchIterationInput{RunID: run.ID, AnalysisID: "a2", AddedEvidence: []string{"comparison region visits"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := status(*it); got != "c-supported=CONTRADICTED,c-fabricated=UNKNOWN,c-causal=CONTRADICTED" {
		t.Fatalf("re-inspection = %s", got)
	}
	first := it.ClaimInspections[0]
	if first.Previous == nil || first.Previous.IterationID != run.Iterations[0].ID || first.Previous.Status != domain.ClaimSupported || !first.Previous.Changed {
		t.Fatalf("status change not recorded: %+v", first.Previous)
	}
	persisted, _ := app.repos.Research.GetResearchRun(ctx, run.ID)
	if status(persisted.Iterations[0]) != "c-supported=SUPPORTED,c-fabricated=UNKNOWN,c-causal=INSUFFICIENT" {
		t.Fatal("the earlier iteration's inspections must stay as recorded")
	}
}

func TestCreateResearchRunRejectsClaimIDReusedForDifferentContent(t *testing.T) {
	app, ctx := newResearchTestApp(t)
	fixed := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	app.now = func() time.Time { return fixed }
	if err := app.repos.Projects.Create(ctx, &domain.Project{ID: "p1", Name: "claims", CreatedAt: fixed}); err != nil {
		t.Fatal(err)
	}
	if err := app.repos.Documents.Create(ctx, &domain.Document{ID: "doc-1", ProjectID: "p1", Source: domain.SourceDocument, Content: "Visits rose from 100 to 130.", CreatedAt: fixed}); err != nil {
		t.Fatal(err)
	}
	seedClaimResearch(t, app, fixed, domain.ValidationSupported, "a1", fixed)
	_, err := app.CreateResearchRun(ctx, CreateResearchRunInput{ProjectID: "p1", AnalysisID: "a1", Question: "Did visits change?", Claims: []domain.ResearchClaim{{ID: "c1", Statement: "rose"}, {ID: "c1", Statement: "fell"}}})
	if err == nil || !strings.Contains(err.Error(), "c1") {
		t.Fatalf("err = %v", err)
	}
}
