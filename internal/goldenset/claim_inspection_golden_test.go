//go:build golden

package goldenset

// Claim Inspection golden cases (#119). Each synthetic, domain-neutral case
// holds a research state (documents, observations, hypotheses with evidence)
// and external claims, and pins the status and exact flag set the real
// inspector must derive. Negative cases (invented source, causal wording,
// over-generalization, definition change) matter as much as supported ones.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/service"
)

type claimCase struct {
	CaseID  string `json:"caseId"`
	Version string `json:"version"`
	Domain  string `json:"domain"`
	Input   struct {
		Documents []struct {
			ID          string `json:"id"`
			ExternalRef string `json:"externalRef"`
			Source      string `json:"source"`
			Content     string `json:"content"`
			Manifest    *struct {
				DatasetID              string `json:"datasetId"`
				Unit                   string `json:"unit"`
				PopulationScope        string `json:"populationScope"`
				PopulationDefinitionID string `json:"populationDefinitionId"`
				Period                 string `json:"period"`
				SchemaID               string `json:"schemaId"`
				SchemaVersion          string `json:"schemaVersion"`
			} `json:"manifest"`
		} `json:"documents"`
		Observations []struct {
			ID         string `json:"id"`
			DocumentID string `json:"documentId"`
			Quote      string `json:"quote"`
		} `json:"observations"`
		Insights []struct {
			ID                   string   `json:"id"`
			Title                string   `json:"title"`
			HypothesisSetID      string   `json:"hypothesisSetId"`
			Role                 string   `json:"role"`
			CausalStatus         string   `json:"causalStatus"`
			ValidationStatus     string   `json:"validationStatus"`
			IdentificationStatus string   `json:"identificationStatus"`
			MissingEvidence      []string `json:"missingEvidence"`
			Evidence             []struct {
				ID            string `json:"id"`
				InsightID     string `json:"insightId"`
				DocumentID    string `json:"documentId"`
				ObservationID string `json:"observationId"`
				Quote         string `json:"quote"`
				Type          string `json:"type"`
			} `json:"evidence"`
		} `json:"insights"`
		ResearchGaps []domain.ResearchGap   `json:"researchGaps"`
		Claims       []domain.ResearchClaim `json:"claims"`
	} `json:"input"`
	Expected []struct {
		ClaimID string   `json:"claimId"`
		Status  string   `json:"status"`
		Flags   []string `json:"flags"`
	} `json:"expected"`
}

func (c claimCase) inspectionInput() service.ClaimInspectionInput {
	var in service.ClaimInspectionInput
	for _, d := range c.Input.Documents {
		meta := map[string]string{"public_external_ref": d.ExternalRef}
		if m := d.Manifest; m != nil {
			meta = service.AcquisitionManifest{
				SourceName: "synthetic golden case", DatasetID: m.DatasetID, RetrievalMethod: service.RetrievalDownload,
				RetrievedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Period: m.Period, Unit: m.Unit,
				PopulationScope: m.PopulationScope, PopulationDefinitionID: m.PopulationDefinitionID, SchemaID: m.SchemaID, SchemaVersion: m.SchemaVersion,
			}.DocumentMetadata(meta)
		}
		in.Documents = append(in.Documents, &domain.Document{ID: d.ID, Source: domain.SourceType(d.Source), Content: d.Content, Metadata: meta})
	}
	for _, o := range c.Input.Observations {
		in.Observations = append(in.Observations, &domain.Observation{ID: o.ID, DocumentID: o.DocumentID, Quote: o.Quote})
	}
	for _, h := range c.Input.Insights {
		insight := &domain.Insight{ID: h.ID, Title: h.Title, HypothesisSetID: h.HypothesisSetID, HypothesisRole: domain.HypothesisRole(h.Role),
			CausalStatus: domain.CausalStatus(h.CausalStatus), ValidationStatus: domain.ValidationStatus(h.ValidationStatus),
			IdentificationStatus: domain.IdentificationStatus(h.IdentificationStatus), MissingEvidence: h.MissingEvidence}
		for _, e := range h.Evidence {
			obs := e.ObservationID
			insight.Evidence = append(insight.Evidence, domain.Evidence{ID: e.ID, InsightID: e.InsightID, DocumentID: e.DocumentID, ObservationID: &obs, Quote: e.Quote, Type: domain.EvidenceType(e.Type)})
		}
		in.Insights = append(in.Insights, insight)
	}
	in.ResearchGaps = c.Input.ResearchGaps
	in.Claims = c.Input.Claims
	return in
}

func TestGoldenClaimInspectionDerivesStatusAndFlagsFromResearchState(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "golden", "claims", "cases", "*.json"))
	if err != nil || len(paths) < 3 {
		t.Fatalf("need at least 3 claim golden cases, found %d (%v)", len(paths), err)
	}
	domains := map[string]bool{}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var c claimCase
		if err := json.Unmarshal(raw, &c); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		domains[c.Domain] = true
		t.Run(c.CaseID, func(t *testing.T) {
			in := c.inspectionInput()
			got, err := service.InspectClaims(in)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(c.Expected) {
				t.Fatalf("inspections = %d, want %d", len(got), len(c.Expected))
			}
			evidenceIDs := map[string]bool{}
			for _, h := range in.Insights {
				for _, e := range h.Evidence {
					evidenceIDs[e.ID] = true
				}
			}
			for i, want := range c.Expected {
				ins := got[i]
				var flags []string
				for _, f := range ins.Flags {
					flags = append(flags, string(f.Code))
				}
				sort.Strings(flags)
				if ins.ClaimID != want.ClaimID || string(ins.Status) != want.Status || strings.Join(flags, ",") != strings.Join(want.Flags, ",") {
					t.Errorf("%s: got %s flags [%s], want %s flags [%s]", want.ClaimID, ins.Status, strings.Join(flags, ","), want.Status, strings.Join(want.Flags, ","))
				}
				// Only existing grounded evidence may appear; a claim never adds any.
				for _, list := range [][]domain.ClaimEvidence{ins.SupportingEvidence, ins.CounterEvidence, ins.NeutralEvidence} {
					for _, e := range list {
						if !evidenceIDs[e.EvidenceID] {
							t.Errorf("%s: inspection cites evidence %s that the research state does not hold", want.ClaimID, e.EvidenceID)
						}
					}
				}
				if ins.Status == domain.ClaimUnknown && len(ins.CannotConclude) == 0 {
					t.Errorf("%s: an UNKNOWN claim must say what cannot be concluded", want.ClaimID)
				}
			}
		})
	}
	if len(domains) < 3 {
		t.Fatalf("claim golden cases must cover at least 3 domains, got %v", domains)
	}
}
