package input

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shardCSV has a BOM, CRLF line ends, quoted fields with embedded newlines,
// commas and escaped quotes, a row without a period and no final newline.
const shardCSV = "\ufeffmonth,region,value\r\n" +
	"2026-01,east,1.5\r\n" +
	"2026-01,\"north\r\nwest\",2\r\n" +
	"2026-02,\"say \"\"hi\"\",\nthere\",3\r\n" +
	",east,4\r\n" +
	"2026-02,east,NA\r\n" +
	"2026-03,\"a,b\",5\r\n" +
	"2026-03,east,6"

func aggregateJSON(t *testing.T, agg *Aggregate) string {
	t.Helper()
	b, err := json.Marshal(agg)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPlanCSVShardsMergedShardAggregatesEqualTheSinglePassForEveryShardSize(t *testing.T) {
	whole, err := AggregateCSV(strings.NewReader(shardCSV), sampleSpec, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := aggregateJSON(t, whole)
	fileSum := sha256.Sum256([]byte(shardCSV))
	for _, target := range []int64{1, 5, 17, 40, 1 << 20} {
		plan, err := PlanCSVShards(context.Background(), strings.NewReader(shardCSV), target)
		if err != nil {
			t.Fatal(err)
		}
		if plan.SizeBytes != int64(len(shardCSV)) || plan.SHA256 != hex.EncodeToString(fileSum[:]) {
			t.Fatalf("target %d: plan size/hash = %d/%s", target, plan.SizeBytes, plan.SHA256)
		}
		header := shardCSV[:plan.HeaderLength]
		next := plan.HeaderLength
		merged := NewAggregate()
		for _, s := range plan.Shards {
			if s.Offset != next {
				t.Fatalf("target %d: shard at %d leaves a gap or overlap after %d", target, s.Offset, next)
			}
			next = s.Offset + s.Length
			content := header + shardCSV[s.Offset:next]
			if sum := sha256.Sum256([]byte(content)); hex.EncodeToString(sum[:]) != s.ContentSHA256 {
				t.Fatalf("target %d: shard %s content hash mismatch", target, s.ID())
			}
			agg, err := AggregateCSV(strings.NewReader(content), sampleSpec, nil, 0)
			if err != nil {
				t.Fatalf("target %d: shard %s does not hold whole records: %v", target, s.ID(), err)
			}
			merged.Merge(agg)
		}
		if next != plan.SizeBytes {
			t.Fatalf("target %d: shards end at %d of %d bytes", target, next, plan.SizeBytes)
		}
		if got := aggregateJSON(t, merged); got != want {
			t.Fatalf("target %d (%d shards) changed the aggregate:\n%s\n%s", target, len(plan.Shards), got, want)
		}
	}
	if plan, _ := PlanCSVShards(context.Background(), strings.NewReader(shardCSV), 1); len(plan.Shards) != 7 {
		t.Fatalf("a 1-byte target must cut after every record: %d shards", len(plan.Shards))
	}
}

func TestPlanCSVShardsHasNoShardForAHeaderOnlyFile(t *testing.T) {
	for _, body := range []string{"month,region,value\n", "month,region,value"} {
		plan, err := PlanCSVShards(context.Background(), strings.NewReader(body), 10)
		if err != nil || len(plan.Shards) != 0 || plan.HeaderLength != int64(len(body)) {
			t.Fatalf("%q: plan = %+v, %v", body, plan, err)
		}
	}
}

func TestPlanCSVShardsRejectsAnOversizedHeader(t *testing.T) {
	body := strings.Repeat("h", MaxCSVHeaderBytes+1) + "\n1\n"
	if _, err := PlanCSVShards(context.Background(), strings.NewReader(body), 10); err == nil {
		t.Fatal("oversized header accepted")
	}
}

func TestOpenRangeReadsOnlyTheRangeInsideTheRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "d.csv"), []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	rs := Resolvers{"file": FileResolver{Root: root}}
	rc, err := OpenRange(context.Background(), rs, "file:d.csv", 3, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	buf := make([]byte, 16)
	n, _ := rc.Read(buf)
	if string(buf[:n]) != "3456" {
		t.Fatalf("range = %q", buf[:n])
	}
	if _, err := OpenRange(context.Background(), rs, "file:../d.csv", 0, 1); err == nil {
		t.Fatal("a range outside the root was opened")
	}
}
