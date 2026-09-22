package analytical

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"insight-lab/internal/domain"
)

func fixture(t *testing.T) Artifact {
	t.Helper()
	data, err := os.ReadFile("../../contracts/analytical-artifact/v1/fixtures/trade-time-series.json")
	if err != nil { t.Fatal(err) }
	artifact, err := Import(data)
	if err != nil { t.Fatal(err) }
	return *artifact
}

func TestImportFixtureAndUnknownFieldTolerance(t *testing.T) {
	a := fixture(t)
	if a.ArtifactSchema != Schema || len(a.Results) != 3 { t.Fatalf("unexpected artifact: %#v", a) }
	data, _ := json.Marshal(a)
	data = append(data[:len(data)-1], []byte(`,"futureOptionalField":true}`)...)
	if _, err := Import(data); err != nil { t.Fatalf("additive field should be tolerated: %v", err) }
}

func TestValidateRejectsMissingProvenance(t *testing.T) {
	a := fixture(t)
	a.Provenance = nil
	if err := a.Validate(); !errors.Is(err, ErrInvalidArtifact) { t.Fatalf("got %v", err) }
	a = fixture(t)
	a.Datasets[0].Hash = Hash{}
	if err := a.Validate(); !errors.Is(err, ErrInvalidArtifact) { t.Fatalf("got %v", err) }
}

func TestDuplicateSemantics(t *testing.T) {
	a := fixture(t)
	duplicate, err := CheckDuplicate(a, a)
	if err != nil || !duplicate { t.Fatalf("duplicate=%v err=%v", duplicate, err) }
	b := a
	b.ArtifactHash.Value = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if duplicate, err = CheckDuplicate(a, b); duplicate || !errors.Is(err, ErrIdentityConflict) {
		t.Fatalf("duplicate=%v err=%v", duplicate, err)
	}
}

func TestAdapterProducesOnlyNeutralCandidates(t *testing.T) {
	a := fixture(t)
	candidates, err := ToCandidates(a)
	if err != nil { t.Fatal(err) }
	if len(candidates) != len(a.Results) { t.Fatalf("got %d candidates", len(candidates)) }
	for _, candidate := range candidates {
		if candidate.Evidence.Type != domain.EvidenceNeutral { t.Fatalf("analytical result promoted to %q", candidate.Evidence.Type) }
		if candidate.Evidence.InsightID != "" { t.Fatal("candidate must not be attached to an Insight") }
		if candidate.Observation.Behavior == "" { t.Fatal("candidate boundary must remain explicit") }
	}
}

func TestResultCannotClaimStructuredExplanation(t *testing.T) {
	a := fixture(t)
	a.Results[0].Value = json.RawMessage(`{"cause":"exchange rate"}`)
	if err := a.Validate(); !errors.Is(err, ErrInvalidArtifact) { t.Fatalf("got %v", err) }
}

func TestReproducibilityKeyIgnoresDatasetOrder(t *testing.T) {
	a := fixture(t)
	b := a
	b.Datasets = append([]DatasetRef(nil), a.Datasets...)
	b.Datasets[0], b.Datasets[1] = b.Datasets[1], b.Datasets[0]
	if a.ReproducibilityKey() != b.ReproducibilityKey() { t.Fatal("key depends on dataset order") }
}
