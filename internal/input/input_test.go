package input

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/domain"
)

const sampleCSV = "month,region,value\n2026-01,east,1.1\n2026-01,west,2.2\n2026-02,east,\n2026-02,west,3.3\n"

var sampleSpec = PreparationSpec{
	Kind:             PreparationKindCSVAggregate,
	Metrics:          []PreparationMetric{{ID: "value_sum", Name: "Value", Column: "value", Aggregation: "sum", Unit: "units"}, {ID: "rows", Name: "Rows", Aggregation: "count", Unit: "rows"}},
	DimensionColumns: []string{"region"},
	PeriodColumn:     "month",
	Population:       PreparationPopulation{Description: "sample rows"},
}

func TestDocumentSourceYieldsDocumentsThenEOF(t *testing.T) {
	src := NewDocumentSource([]*domain.Document{{ID: "a"}, {ID: "b"}})
	var ids []string
	for {
		d, err := src.Next(context.Background())
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, d.ID)
	}
	if strings.Join(ids, ",") != "a,b" {
		t.Fatalf("ids = %v", ids)
	}
}

func TestMeasureShapeCountsRawReferencesSeparatelyFromInlineText(t *testing.T) {
	raw := &domain.Document{Content: "descriptor", Metadata: map[string]string{MetadataKind: string(KindRawArtifact), MetadataRawSize: "1000", MetadataPreparation: "{}"}}
	shape, err := MeasureShape(context.Background(), NewDocumentSource([]*domain.Document{{Content: "abc"}, raw}))
	if err != nil {
		t.Fatal(err)
	}
	want := Shape{Documents: 1, InlineBytes: 3, RawArtifacts: 1, RawBytes: 1000, RawToPrepare: 1, RawToPrepareByte: 1000}
	if shape != want {
		t.Fatalf("shape = %+v, want %+v", shape, want)
	}
}

func TestFileResolverRefusesPathsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok.csv"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := FileResolver{Root: root}
	for _, uri := range []string{"file:../etc/passwd", "file:///etc/passwd", "file://host/x", "http://x/y"} {
		if _, err := r.Open(context.Background(), uri); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s: want ErrUnavailable, got %v", uri, err)
		}
	}
	rc, err := r.Open(context.Background(), "file:ok.csv")
	if err != nil {
		t.Fatal(err)
	}
	rc.Close()
}

func TestVerifyFillsHashFromStreamAndRejectsFalseClaims(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "d.csv"), []byte(sampleCSV), 0o600); err != nil {
		t.Fatal(err)
	}
	rs := Resolvers{"file": FileResolver{Root: root}}
	sum := sha256.Sum256([]byte(sampleCSV))
	got, err := Verify(context.Background(), rs, RawArtifactRef{URI: "file:d.csv"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.SHA256 != hex.EncodeToString(sum[:]) || got.SizeBytes != int64(len(sampleCSV)) {
		t.Fatalf("verified ref = %+v", got)
	}
	if _, err := Verify(context.Background(), rs, RawArtifactRef{URI: "file:d.csv", SHA256: strings.Repeat("0", 64)}, 0); !errors.Is(err, ErrVerification) {
		t.Fatalf("false sha256 claim accepted: %v", err)
	}
	if _, err := Verify(context.Background(), rs, RawArtifactRef{URI: "file:d.csv"}, 10); !errors.Is(err, ErrVerification) {
		t.Fatalf("size limit ignored: %v", err)
	}
}

func TestPartitionedAggregationEqualsSinglePass(t *testing.T) {
	whole, err := AggregateCSV(strings.NewReader(sampleCSV), sampleSpec, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	merged := NewAggregate()
	for part := int64(0); part < 3; part++ {
		p := part
		a, err := AggregateCSV(strings.NewReader(sampleCSV), sampleSpec, func(row int64) bool { return row%3 == p }, 0)
		if err != nil {
			t.Fatal(err)
		}
		merged.Merge(a)
	}
	ref := RawArtifactRef{URI: "file:d.csv", SHA256: strings.Repeat("a", 64)}
	at := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	a1, err := BuildPreparedArtifact("d", ref, sampleSpec, whole, at)
	if err != nil {
		t.Fatal(err)
	}
	a2, err := BuildPreparedArtifact("d", ref, sampleSpec, merged, at)
	if err != nil {
		t.Fatal(err)
	}
	if a1.ArtifactHash != a2.ArtifactHash {
		t.Fatalf("partitioned preparation changed the artifact: %v vs %v", a1.ArtifactHash, a2.ArtifactHash)
	}
	var missing int
	for _, r := range a1.Results {
		if r.Missing {
			missing++
		}
	}
	if missing != 1 {
		t.Fatalf("empty value group must be missing, not zero; missing=%d", missing)
	}
}

func TestPartialSurvivesJSONRoundTrip(t *testing.T) {
	a, _ := AggregateCSV(strings.NewReader(sampleCSV), sampleSpec, nil, 0)
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	var back Aggregate
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	again, _ := json.Marshal(&back)
	if string(again) != string(b) {
		t.Fatalf("round trip changed aggregate:\n%s\n%s", b, again)
	}
}

func TestAggregateCSVRejectsTooManyGroups(t *testing.T) {
	if _, err := AggregateCSV(strings.NewReader(sampleCSV), sampleSpec, nil, 2); err == nil {
		t.Fatal("group limit ignored")
	}
}

// rowStream generates CSV rows lazily; the whole stream is never in memory.
type rowStream struct {
	rows, next int
	buf        []byte
}

func (s *rowStream) Read(p []byte) (int, error) {
	for len(s.buf) < len(p) && s.next <= s.rows {
		if s.next == 0 {
			s.buf = append(s.buf, "month,region,value\n"...)
		} else {
			s.buf = append(s.buf, fmt.Sprintf("2026-%02d,r%d,%d\n", s.next%12+1, s.next%4, s.next%1000)...)
		}
		s.next++
	}
	if len(s.buf) == 0 {
		return 0, io.EOF
	}
	n := copy(p, s.buf)
	s.buf = s.buf[n:]
	return n, nil
}

func TestAggregateCSVStreamsLargeInputWithBoundedMemory(t *testing.T) {
	const rows = 400_000 // ~7 MB of CSV, far above any retained state
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	agg, err := AggregateCSV(&rowStream{rows: rows}, sampleSpec, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	if agg.RowsRead != rows || len(agg.Groups) != 12 {
		t.Fatalf("rows=%d groups=%d", agg.RowsRead, len(agg.Groups))
	}
	if grew := int64(after.HeapAlloc) - int64(before.HeapAlloc); grew > 4<<20 {
		t.Fatalf("retained heap grew by %d bytes; aggregation must not keep rows", grew)
	}
}
