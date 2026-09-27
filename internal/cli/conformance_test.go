package cli

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// Deployment conformance (#139): the same subject, raw evidence and
// analysis request must give the same prepared artifact and research result
// in every supported runtime. The test drives the real headless CLI and the
// real app wiring; PROCESS workers are this test binary re-executed as
// `worker`, exactly as the server re-executes `insight-lab worker`.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "worker" {
		os.Exit(Main(context.Background(), os.Args[1:], os.Stdout, os.Stderr, nil))
	}
	os.Exit(m.Run())
}

// conformanceCSV spans several shards (quoted newlines and commas, missing
// values, a row without a period) at the default 1 MiB shard size.
func conformanceCSV() []byte {
	var b strings.Builder
	b.WriteString("year,region,amount\n")
	regions := []string{"east", "west", "\"no\nrth\"", "\"a,b\""}
	for i := 0; i < 120_000; i++ {
		year, amount := fmt.Sprint(2020+i%5), fmt.Sprint(i%1000)
		if i%97 == 0 {
			amount = ""
		}
		if i%1013 == 0 {
			year = ""
		}
		fmt.Fprintf(&b, "%s,%s,%s\n", year, regions[i%len(regions)], amount)
	}
	return []byte(b.String())
}

type conformanceRun struct {
	artifact    string // prepared Analytical Artifact document content
	runtimeMode string // recorded in the execution snapshot
	metrics     map[string]any
}

func runConformance(t *testing.T, runtime string, inputRoot, request string) conformanceRun {
	t.Helper()
	dir := t.TempDir()
	flags := []string{"-db", filepath.Join(dir, "insight.db"), "-input-root", inputRoot, "-heavy-dir", filepath.Join(dir, "heavy"), "-runtime", runtime}
	subject := run(t, append([]string{"subject", "create", "-namespace", "conformance", "-id", "s1"}, flags...)...)
	if subject.code != ExitOK {
		t.Fatalf("%s subject: %s", runtime, subject.stderr)
	}
	id, _ := subject.stdout["subjectId"].(string)
	if r := run(t, append([]string{"evidence", "add", "-subject", id, "-request", request}, flags...)...); r.code != ExitOK {
		t.Fatalf("%s evidence: %s", runtime, r.stderr)
	}
	analysis := run(t, append([]string{"analysis", "start", "-subject", id, "-execution-profile", "HEAVY"}, flags...)...)
	if analysis.code != ExitOK || analysis.stdout["status"] != "completed" {
		t.Fatalf("%s analysis: exit %d %v %s", runtime, analysis.code, analysis.stdout, analysis.stderr)
	}

	db, err := sql.Open("sqlite", filepath.Join(dir, "insight.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var out conformanceRun
	rows, err := db.Query(`SELECT metadata, content FROM documents`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var meta sql.NullString
		var content string
		if err := rows.Scan(&meta, &content); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(meta.String, "public_prepared_from") {
			out.artifact = content
		}
	}
	rows.Close()
	var snapshot, metrics string
	if err := db.QueryRow(`SELECT execution_snapshot, metrics FROM analyses`).Scan(&snapshot, &metrics); err != nil {
		t.Fatal(err)
	}
	var snap struct {
		RuntimeMode string `json:"runtimeMode"`
	}
	_ = json.Unmarshal([]byte(snapshot), &snap)
	out.runtimeMode = snap.RuntimeMode
	_ = json.Unmarshal([]byte(metrics), &out.metrics)
	return out
}

func TestDeploymentConformanceLocalAndProcessRuntimesProduceTheSameResearchInput(t *testing.T) {
	if testing.Short() {
		t.Skip("runs two HEAVY analyses")
	}
	t.Setenv("INSIGHT_LAB_API_KEY", "") // deterministic engine; no model is called
	inputRoot := t.TempDir()
	data := conformanceCSV()
	if err := os.WriteFile(filepath.Join(inputRoot, "raw.csv"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	request := filepath.Join(t.TempDir(), "evidence.json")
	body := fmt.Sprintf(`{"contractVersion":"1","idempotencyKey":"raw-1","inputSources":[{"externalRef":"raw-1","kind":"RAW_ARTIFACT","title":"raw","rawArtifact":{"uri":"file:raw.csv","mediaType":"text/csv","name":"raw.csv","sha256":"%s"},"preparation":{"kind":"csv-aggregate/v1","metrics":[{"id":"amount_sum","name":"Amount","column":"amount","aggregation":"sum","unit":"units"},{"id":"amount_mean","name":"Mean","column":"amount","aggregation":"mean","unit":"units"}],"dimensionColumns":["region"],"periodColumn":"year","population":{"description":"conformance rows"}}}]}`, hex.EncodeToString(sum[:]))
	if err := os.WriteFile(request, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	local := runConformance(t, "local", inputRoot, request)
	process := runConformance(t, "process", inputRoot, request)
	if local.artifact == "" || local.artifact != process.artifact {
		t.Fatalf("prepared artifacts differ between runtimes:\nLOCAL   %.300s\nPROCESS %.300s", local.artifact, process.artifact)
	}
	if local.runtimeMode != "LOCAL" || process.runtimeMode != "PROCESS" {
		t.Fatalf("runtime modes recorded as %q / %q", local.runtimeMode, process.runtimeMode)
	}
	for _, key := range []string{"groundedObservations", "patternCount", "traceCount", "finalInsightCount", "evidenceCoverage"} {
		if _, ok := local.metrics[key]; !ok {
			t.Errorf("metric %s was not recorded", key)
		}
		if fmt.Sprint(local.metrics[key]) != fmt.Sprint(process.metrics[key]) {
			t.Errorf("%s differs: LOCAL %v, PROCESS %v", key, local.metrics[key], process.metrics[key])
		}
	}
}

func TestDeploymentConformanceUnavailableCapabilitiesFailInsteadOfDowngrading(t *testing.T) {
	dir := t.TempDir()
	db := []string{"-db", filepath.Join(dir, "insight.db")}
	subject := run(t, append([]string{"subject", "create", "-namespace", "conformance", "-id", "s2"}, db...)...)
	id, _ := subject.stdout["subjectId"].(string)
	if r := run(t, append([]string{"evidence", "add", "-subject", id, "-document", writeTemp(t, "a note")}, db...)...); r.code != ExitOK {
		t.Fatalf("evidence: %s", r.stderr)
	}
	heavy := run(t, append([]string{"analysis", "start", "-subject", id, "-execution-profile", "HEAVY"}, db...)...)
	if heavy.code != ExitCapabilityUnavailable || errorCode(t, heavy) != "EXECUTION_PROFILE_UNAVAILABLE" {
		t.Fatalf("HEAVY without -heavy-dir: exit %d %s", heavy.code, heavy.stderr)
	}
	for _, args := range [][]string{
		{"-runtime", "process"},                     // no -heavy-dir / -input-root
		{"-runtime", "distributed"},                 // not implemented
		{"-runtime", "process", "-heavy-dir", dir},  // no -input-root
		{"-runtime", "process", "-input-root", dir}, // no -heavy-dir
	} {
		r := run(t, append(append([]string{"engine"}, db...), args...)...)
		if r.code == ExitOK {
			t.Errorf("engine %v started; an unavailable runtime must fail", args)
		}
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "doc.txt")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}
