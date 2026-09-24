package triage

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"insight-lab/internal/llm"
)

const sampleCSV = `year,region,exports,imports,firm_id,notes
2022,EU,100,40,f1,
2022,US,80,30,f2,late
2023,EU,,45,f3,
2023,US,90,NA,f4,
`

func mustProfile(t *testing.T) Profile {
	t.Helper()
	p, err := ProfileCSV([]byte(sampleCSV))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProfileCSVIsDeterministicAndBounded(t *testing.T) {
	a, b := mustProfile(t), mustProfile(t)
	if a.Fingerprint() != b.Fingerprint() {
		t.Fatal("same bytes produced different profiles")
	}
	got := map[string]ColumnType{}
	for _, c := range a.Columns {
		got[c.Name] = c.Type
		if len(c.SampleValues) > SampleLimit {
			t.Errorf("%s keeps %d samples", c.Name, len(c.SampleValues))
		}
	}
	want := map[string]ColumnType{"year": TypeInteger, "region": TypeString, "exports": TypeInteger, "imports": TypeInteger, "firm_id": TypeString, "notes": TypeString}
	if !reflect.DeepEqual(got, want) || a.RowCount != 4 || !strings.HasPrefix(a.ContentSHA256, "sha256:") {
		t.Fatalf("profile = %+v", a)
	}
	exports := a.Columns[2]
	if exports.NullCount != 1 || exports.Min != "80" || exports.Max != "100" {
		t.Fatalf("exports = %+v", exports)
	}
}

func TestProfileCSVRejectsDuplicateColumnNames(t *testing.T) {
	if _, err := ProfileCSV([]byte("a,a\n1,2\n")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestNormalizeRejectsInventedColumns(t *testing.T) {
	_, err := Normalize(mustProfile(t), []Decision{{Name: "made_up", Bucket: BucketInclude, Rationale: "x"}}, ProposerModel)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestNormalizeKeepsEveryColumnExactlyOnceInProfileOrder(t *testing.T) {
	p := mustProfile(t)
	out, err := Normalize(p, []Decision{
		{Name: "exports", Bucket: BucketInclude, Rationale: "outcome"},
		{Name: "exports", Bucket: BucketDefer, Rationale: "duplicate ignored"},
	}, ProposerModel)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(p.Columns) {
		t.Fatalf("got %d decisions for %d columns", len(out), len(p.Columns))
	}
	for i, d := range out {
		if d.Name != p.Columns[i].Name {
			t.Fatalf("decision %d is %s, want %s", i, d.Name, p.Columns[i].Name)
		}
	}
	if out[2].Bucket != BucketInclude || out[0].Bucket != BucketNeedsReview || out[0].Rationale != omittedRationale {
		t.Fatalf("decisions = %+v", out)
	}
}

func TestNormalizeDowngradesAutomatedExcludeToDefer(t *testing.T) {
	out, err := Normalize(mustProfile(t), []Decision{{Name: "notes", Bucket: BucketExclude, Rationale: "noise"}}, ProposerModel)
	if err != nil {
		t.Fatal(err)
	}
	if out[5].Bucket != BucketDefer {
		t.Fatalf("automated exclude kept: %+v", out[5])
	}
	human, _ := Normalize(mustProfile(t), []Decision{{Name: "notes", Bucket: BucketExclude, Rationale: "noise"}}, ProposerHuman)
	if human[5].Bucket != BucketExclude {
		t.Fatal("human exclude was changed")
	}
}

func TestDeterministicIncludesQuestionMatchesAndDefersTheRest(t *testing.T) {
	out, err := Normalize(mustProfile(t), Deterministic(Input{Question: "Why did exports to the EU rise?", Profile: mustProfile(t)}), ProposerDeterministic)
	if err != nil {
		t.Fatal(err)
	}
	buckets := map[string]Bucket{}
	for _, d := range out {
		buckets[d.Name] = d.Bucket
	}
	want := map[string]Bucket{"year": BucketInclude, "region": BucketInclude, "exports": BucketInclude, "imports": BucketDefer, "firm_id": BucketInclude, "notes": BucketInclude}
	if !reflect.DeepEqual(buckets, want) {
		t.Fatalf("buckets = %v", buckets)
	}
}

func TestDeterministicReproposesDeferredVariableReferencedByGap(t *testing.T) {
	in := Input{Question: "Why did exports rise?", Profile: mustProfile(t), Gaps: []GapRef{{GapID: "gap-1", Need: "import volumes over the same period"}}}
	out, _ := Normalize(in.Profile, Deterministic(in), ProposerDeterministic)
	if out[3].Name != "imports" || out[3].Bucket != BucketInclude || !reflect.DeepEqual(out[3].LinkedGapIDs, []string{"gap-1"}) {
		t.Fatalf("imports = %+v", out[3])
	}
}

func TestReviseRecordsFromBucketAndLeavesBaseUntouched(t *testing.T) {
	base, _ := Normalize(mustProfile(t), Deterministic(Input{Question: "exports", Profile: mustProfile(t)}), ProposerDeterministic)
	next, moves, err := Revise(base, []Move{{Name: "imports", ToBucket: BucketInclude, Rationale: "needed as comparison"}})
	if err != nil {
		t.Fatal(err)
	}
	if base[3].Bucket != BucketDefer || next[3].Bucket != BucketInclude || moves[0].FromBucket != BucketDefer || next[3].Role != RoleMetric {
		t.Fatalf("base=%+v next=%+v moves=%+v", base[3], next[3], moves)
	}
	if _, _, err := Revise(base, []Move{{Name: "imports", ToBucket: BucketInclude}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("move without rationale accepted: %v", err)
	}
}

type fakeModel struct{ content string }

func (f fakeModel) Generate(_ context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	if err := req.Schema.Validate(json.RawMessage(f.content)); err != nil {
		return nil, err
	}
	return &llm.GenerateResponse{Content: json.RawMessage(f.content)}, nil
}

func TestModelOutputPassesThroughNormalization(t *testing.T) {
	p := mustProfile(t)
	raw, err := Model(context.Background(), fakeModel{`{"decisions":[{"name":"exports","bucket":"INCLUDE","role":"METRIC","classifications":["POSSIBLE_OUTCOME"],"rationale":"asked about exports"}]}`}, Input{Question: "exports", Profile: p})
	if err != nil {
		t.Fatal(err)
	}
	out, err := Normalize(p, raw, ProposerModel)
	if err != nil {
		t.Fatal(err)
	}
	if out[2].Classifications[0] != "POSSIBLE_OUTCOME" || out[0].Bucket != BucketNeedsReview {
		t.Fatalf("decisions = %+v", out)
	}
}
