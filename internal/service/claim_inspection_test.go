package service

import (
	"encoding/json"
	"strings"
	"testing"

	"insight-lab/internal/domain"
)

func strptr(s string) *string { return &s }

// claimFixture: one primary hypothesis and one competitor, both drawing on
// the same grounded observation obs-1 of document doc-1.
func claimFixture(primary, competitor domain.ValidationStatus) ClaimInspectionInput {
	evidence := func(id, insight string, typ domain.EvidenceType) domain.Evidence {
		return domain.Evidence{ID: id, InsightID: insight, DocumentID: "doc-1", ObservationID: strptr("obs-1"), Quote: "Visits rose from 100 to 130.", Type: typ}
	}
	return ClaimInspectionInput{
		Insights: []*domain.Insight{
			{ID: "h-primary", Title: "Campaign timing", HypothesisSetID: "set-1", HypothesisRole: domain.HypothesisPrimary,
				CausalStatus: domain.CausalObservedAssociation, ValidationStatus: primary, IdentificationStatus: domain.IdentificationNotIdentified,
				MissingEvidence: []string{"visits in a comparable region"}, Evidence: []domain.Evidence{evidence("ev-1", "h-primary", domain.EvidenceSupport)}},
			{ID: "h-seasonal", Title: "Seasonal demand", HypothesisSetID: "set-1", HypothesisRole: domain.HypothesisCompeting,
				CausalStatus: domain.CausalHypothesis, ValidationStatus: competitor, IdentificationStatus: domain.IdentificationNotIdentified,
				Evidence: []domain.Evidence{evidence("ev-2", "h-seasonal", domain.EvidenceNeutral)}},
		},
		Observations: []*domain.Observation{{ID: "obs-1", DocumentID: "doc-1", Quote: "Visits rose from 100 to 130."}},
		Documents:    []*domain.Document{{ID: "doc-1", Source: domain.SourceDocument, Content: "Visits rose from 100 to 130.", Metadata: map[string]string{"public_external_ref": "visits-report"}}},
		ResearchGaps: []domain.ResearchGap{{ID: "gap-1", Need: "comparison region", AffectedHypothesisIDs: []string{"h-primary"}}},
	}
}

func inspectOne(t *testing.T, in ClaimInspectionInput, claim domain.ResearchClaim) domain.ClaimInspection {
	t.Helper()
	in.Claims = []domain.ResearchClaim{claim}
	out, err := InspectClaims(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("inspections = %d, want 1", len(out))
	}
	return out[0]
}

func hasClaimFlag(i domain.ClaimInspection, code domain.ClaimFlagCode) bool {
	for _, f := range i.Flags {
		if f.Code == code {
			return true
		}
	}
	return false
}

func TestInspectClaimsReportsSupportedWhenBasisIsSupportedAndCompetitorIsContradicted(t *testing.T) {
	got := inspectOne(t, claimFixture(domain.ValidationSupported, domain.ValidationContradicted), domain.ResearchClaim{
		ID: "c1", Statement: "Visits were higher during the campaign month.", EvidenceReferences: []string{"obs-1"}, HypothesisReferences: []string{"h-primary"},
		Assumptions: []string{"visit counting did not change"},
	})
	if got.Status != domain.ClaimSupported {
		t.Fatalf("status = %s, flags = %+v", got.Status, got.Flags)
	}
	if len(got.Basis) != 1 || got.Basis[0].InsightID != "h-primary" || got.Basis[0].ValidationStatus != domain.ValidationSupported {
		t.Fatalf("basis = %+v", got.Basis)
	}
	if len(got.CompetingHypotheses) != 1 || got.CompetingHypotheses[0].InsightID != "h-seasonal" {
		t.Fatalf("competing = %+v", got.CompetingHypotheses)
	}
	if len(got.SupportingEvidence) != 1 || got.SupportingEvidence[0].EvidenceID != "ev-1" {
		t.Fatalf("supporting evidence = %+v", got.SupportingEvidence)
	}
	if strings.Join(got.UnverifiedAssumptions, "|") != "visit counting did not change" {
		t.Fatalf("assumptions must pass through unverified: %v", got.UnverifiedAssumptions)
	}
	if strings.Join(got.RequiredEvidence, "|") != "visits in a comparable region" || strings.Join(got.ResearchGapIDs, "|") != "gap-1" {
		t.Fatalf("required evidence = %v gaps = %v", got.RequiredEvidence, got.ResearchGapIDs)
	}
	if got.RuleVersion != "claim-inspection/v1" || !strings.HasPrefix(got.ContentHash, "sha256:") {
		t.Fatalf("rule version %q / content hash %q", got.RuleVersion, got.ContentHash)
	}
}

func TestInspectClaimsCapsAtInsufficientWhenCompetingExplanationIsStillOpen(t *testing.T) {
	got := inspectOne(t, claimFixture(domain.ValidationSupported, domain.ValidationPlausible), domain.ResearchClaim{
		ID: "c1", Statement: "Visits were higher during the campaign month.", EvidenceReferences: []string{"obs-1"}, HypothesisReferences: []string{"h-primary"},
	})
	if got.Status != domain.ClaimInsufficient || !hasClaimFlag(got, domain.ClaimFlagCompetingExplanationOpen) {
		t.Fatalf("status = %s flags = %+v", got.Status, got.Flags)
	}
}

func TestInspectClaimsCapsCausalWordingAtInsufficientWhenNotIdentified(t *testing.T) {
	for _, statement := range []string{"The campaign caused the rise in visits.", "キャンペーンが原因で来店が増えた。"} {
		got := inspectOne(t, claimFixture(domain.ValidationSupported, domain.ValidationContradicted), domain.ResearchClaim{
			ID: "c1", Statement: statement, EvidenceReferences: []string{"obs-1"}, HypothesisReferences: []string{"h-primary"},
		})
		if got.Status != domain.ClaimInsufficient || !hasClaimFlag(got, domain.ClaimFlagCausalWithoutID) {
			t.Fatalf("%q: status = %s flags = %+v", statement, got.Status, got.Flags)
		}
		if len(got.CannotConclude) == 0 {
			t.Fatalf("%q: causal overreach must say what cannot be concluded", statement)
		}
	}
}

func TestInspectClaimsReportsContradictedWhenEveryBasisHypothesisIsContradicted(t *testing.T) {
	got := inspectOne(t, claimFixture(domain.ValidationContradicted, domain.ValidationContradicted), domain.ResearchClaim{
		ID: "c1", Statement: "Visits were higher during the campaign month.", EvidenceReferences: []string{"visits-report"},
	})
	if got.Status != domain.ClaimContradicted {
		t.Fatalf("status = %s", got.Status)
	}
	if len(got.Citations) != 1 || got.Citations[0].State != domain.ClaimCitationResolved || got.Citations[0].DocumentID != "doc-1" {
		t.Fatalf("external ref must resolve to its document: %+v", got.Citations)
	}
}

func TestInspectClaimsFlagsConflictWhenBasisMixesSupportedAndContradicted(t *testing.T) {
	got := inspectOne(t, claimFixture(domain.ValidationSupported, domain.ValidationContradicted), domain.ResearchClaim{
		ID: "c1", Statement: "Visits were higher during the campaign month.", EvidenceReferences: []string{"obs-1"},
	})
	if got.Status != domain.ClaimInsufficient || !hasClaimFlag(got, domain.ClaimFlagConflictingEvidence) || len(got.Basis) != 2 {
		t.Fatalf("status = %s flags = %+v basis = %+v", got.Status, got.Flags, got.Basis)
	}
}

func TestInspectClaimsReportsUnknownAndNeverPromotesAFabricatedCitation(t *testing.T) {
	in := claimFixture(domain.ValidationSupported, domain.ValidationContradicted)
	got := inspectOne(t, in, domain.ResearchClaim{ID: "c1", Statement: "A survey shows visits doubled.", EvidenceReferences: []string{"survey-2024"}})
	if got.Status != domain.ClaimUnknown || !hasClaimFlag(got, domain.ClaimFlagCitationNotFound) {
		t.Fatalf("status = %s flags = %+v", got.Status, got.Flags)
	}
	if len(got.Citations) != 1 || got.Citations[0].State != domain.ClaimCitationNotFound || got.Citations[0].ObservationID != "" {
		t.Fatalf("citations = %+v", got.Citations)
	}
	if len(got.SupportingEvidence)+len(got.CounterEvidence)+len(got.NeutralEvidence) != 0 || len(in.Observations) != 1 {
		t.Fatal("a claim without resolvable evidence must not gain evidence")
	}
}

func TestInspectClaimsFlagsMissingCitationAndUnknownHypothesis(t *testing.T) {
	got := inspectOne(t, claimFixture(domain.ValidationSupported, domain.ValidationContradicted), domain.ResearchClaim{
		ID: "c1", Statement: "Visits rose.", HypothesisReferences: []string{"h-missing"},
	})
	if got.Status != domain.ClaimUnknown || !hasClaimFlag(got, domain.ClaimFlagNoSourceCitation) || !hasClaimFlag(got, domain.ClaimFlagHypothesisNotFound) {
		t.Fatalf("status = %s flags = %+v", got.Status, got.Flags)
	}
}

func TestInspectClaimsFlagsOvergeneralizationInEnglishAndJapanese(t *testing.T) {
	for _, statement := range []string{"All stores always see more visits in campaigns.", "キャンペーン中は必ず来店が増える。"} {
		got := inspectOne(t, claimFixture(domain.ValidationSupported, domain.ValidationContradicted), domain.ResearchClaim{
			ID: "c1", Statement: statement, EvidenceReferences: []string{"obs-1"}, HypothesisReferences: []string{"h-primary"},
		})
		if !hasClaimFlag(got, domain.ClaimFlagOvergeneralization) {
			t.Fatalf("%q: flags = %+v", statement, got.Flags)
		}
	}
}

func TestInspectClaimsFlagsDefinitionMismatchWhenCitedSourcesAreIncompatible(t *testing.T) {
	a, b := validManifest(), validManifest()
	a.DatasetID, a.PopulationScope, a.PopulationDefinitionID = "reg-2022", "all registered entities", "all"
	b.DatasetID, b.PopulationScope, b.PopulationDefinitionID = "reg-2023", "corporations only", "corp"
	in := claimFixture(domain.ValidationSupported, domain.ValidationContradicted)
	in.Documents = append(in.Documents,
		&domain.Document{ID: "doc-2022", Source: domain.SourceDataset, Metadata: a.DocumentMetadata(nil)},
		&domain.Document{ID: "doc-2023", Source: domain.SourceDataset, Metadata: b.DocumentMetadata(nil)})
	got := inspectOne(t, in, domain.ResearchClaim{ID: "c1", Statement: "Registrations fell 36% in 2023.", EvidenceReferences: []string{"doc-2022", "doc-2023"}})
	if !hasClaimFlag(got, domain.ClaimFlagDefinitionMismatch) {
		t.Fatalf("flags = %+v", got.Flags)
	}
}

func TestInspectClaimsRejectsReusedIDWithDifferentContent(t *testing.T) {
	in := claimFixture(domain.ValidationSupported, domain.ValidationContradicted)
	in.Claims = []domain.ResearchClaim{{ID: "c1", Statement: "Visits rose."}, {ID: "c1", Statement: "Visits fell."}}
	if _, err := InspectClaims(in); err == nil {
		t.Fatal("one claim id with two contents is ambiguous and must be rejected")
	}
}

func TestInspectClaimsMarksIdenticalClaimAsCopyAndIsDeterministic(t *testing.T) {
	in := claimFixture(domain.ValidationSupported, domain.ValidationContradicted)
	claim := domain.ResearchClaim{ID: "c1", Statement: "Visits rose.", EvidenceReferences: []string{"obs-1"}}
	copyClaim := claim
	copyClaim.ID = "c1-again"
	in.Claims = []domain.ResearchClaim{claim, copyClaim}
	first, err := InspectClaims(in)
	if err != nil {
		t.Fatal(err)
	}
	if first[1].DuplicateOf != "c1" || first[1].ContentHash != first[0].ContentHash || first[1].Status != first[0].Status {
		t.Fatalf("copy = %+v", first[1])
	}
	second, _ := InspectClaims(in)
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatalf("inspection is not deterministic:\n%s\n%s", a, b)
	}
}

func TestInspectClaimsRecordsStatusChangeAgainstPreviousIteration(t *testing.T) {
	in := claimFixture(domain.ValidationContradicted, domain.ValidationContradicted)
	in.Previous = &domain.ResearchIteration{ID: "rit-1", ClaimInspections: []domain.ClaimInspection{{ClaimID: "c1", Status: domain.ClaimInsufficient}}}
	got := inspectOne(t, in, domain.ResearchClaim{ID: "c1", Statement: "Visits rose.", EvidenceReferences: []string{"obs-1"}})
	if got.Previous == nil || got.Previous.IterationID != "rit-1" || got.Previous.Status != domain.ClaimInsufficient || !got.Previous.Changed {
		t.Fatalf("previous = %+v, status = %s", got.Previous, got.Status)
	}
}

func TestInspectClaimsNeverReportsSupportedWithoutCitedEvidence(t *testing.T) {
	got := inspectOne(t, claimFixture(domain.ValidationSupported, domain.ValidationContradicted), domain.ResearchClaim{
		ID: "c1", Statement: "Visits were higher during the campaign month.", HypothesisReferences: []string{"h-primary"},
	})
	if got.Status != domain.ClaimInsufficient || !hasClaimFlag(got, domain.ClaimFlagNoSourceCitation) {
		t.Fatalf("status = %s flags = %+v", got.Status, got.Flags)
	}
}
