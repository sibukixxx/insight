//go:build golden

package goldenset

// Discovery Benchmark v1 (#120). Each case feeds synthetic, domain-neutral
// inputs through the real deterministic Core (Temporal Analytics Pack and
// dataset pre-analysis) and asserts what Core actually surfaced: a planted
// non-obvious deviation (positive discovery), nothing (valid no-discovery),
// or an association that must stay guarded (false-association guard).
//
// Only deterministic invariants are judged here. Novelty, surprise and the
// other human rubric items stay PENDING in the case files; nothing in this
// test derives a pass for them. Measurements are compared with a checked-in
// baseline whose input and execution fingerprints are kept separate, so a
// changed result can be attributed to changed input or changed rules.
//
// Regenerate the baseline after an intended change with:
//
//	GOLDEN_UPDATE=1 go test -tags=golden ./internal/goldenset/ -run DiscoveryBenchmark

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"insight-lab/internal/analytical"
	"insight-lab/internal/buildinfo"
	"insight-lab/internal/domain"
	"insight-lab/internal/service"
)

const discoveryBenchmarkVersion = "discovery-benchmark/v1"

var discoveryDir = filepath.Join("..", "..", "testdata", "golden", "discovery")

type discoveryCase struct {
	CaseID           string                     `json:"caseId"`
	Version          string                     `json:"version"`
	Category         string                     `json:"category"`
	Mode             domain.AnalysisMode        `json:"mode"`
	ResearchQuestion string                     `json:"researchQuestion,omitempty"`
	Claim            *domain.ResearchClaim      `json:"claim,omitempty"`
	Input            discoveryInput             `json:"input"`
	Operations       []analytical.OperationSpec `json:"operations,omitempty"`
	Expected         struct {
		Discovery    bool                 `json:"discovery"`
		Assertions   []discoveryAssertion `json:"assertions"`
		KnownFailure *struct {
			Assertions []int  `json:"assertions"`
			Reason     string `json:"reason"`
		} `json:"knownFailure,omitempty"`
	} `json:"expected"`
}

type discoveryInput struct {
	Basis      string            `json:"basis,omitempty"`
	Population string            `json:"population,omitempty"`
	Geography  string            `json:"geography,omitempty"`
	Series     []discoverySeries `json:"series,omitempty"`
	Documents  []discoveryDoc    `json:"documents,omitempty"`
}

// discoverySeries is one metric series. A null value is a recorded missing
// window. Duplicates lists windows imported a second time with another value.
type discoverySeries struct {
	MetricID   string              `json:"metricId"`
	Name       string              `json:"name"`
	Unit       string              `json:"unit"`
	Dimensions map[string]string   `json:"dimensions,omitempty"`
	Values     map[string]*float64 `json:"values"`
	Duplicates map[string]float64  `json:"duplicates,omitempty"`
}

type discoveryDoc struct {
	ID                     string `json:"id"`
	DatasetID              string `json:"datasetId"`
	Period                 string `json:"period"`
	Location               string `json:"location"`
	EventType              string `json:"eventType"`
	RecordCount            string `json:"recordCount"`
	Unit                   string `json:"unit"`
	PopulationScope        string `json:"populationScope"`
	PopulationDefinitionID string `json:"populationDefinitionId"`
	SchemaID               string `json:"schemaId"`
	SchemaVersion          string `json:"schemaVersion"`
	FileHash               string `json:"fileHash"`
}

// discoveryAssertion checks either one operation output (op) or the dataset
// pre-analysis (preanalysis). Numeric bounds apply to every matched result.
type discoveryAssertion struct {
	Op          *int              `json:"op,omitempty"`
	Metric      string            `json:"metric,omitempty"`
	Period      string            `json:"period,omitempty"`
	Dimensions  map[string]string `json:"dimensions,omitempty"`
	Flag        string            `json:"flag,omitempty"`
	Count       *int              `json:"count,omitempty"`
	Min         *float64          `json:"min,omitempty"`
	Max         *float64          `json:"max,omitempty"`
	AbsMax      *float64          `json:"absMax,omitempty"`
	Missing     *bool             `json:"missing,omitempty"`
	Limitation  string            `json:"limitation,omitempty"`
	Error       string            `json:"error,omitempty"`
	Preanalysis string            `json:"preanalysis,omitempty"`
	Code        string            `json:"code,omitempty"`
	Contains    string            `json:"contains,omitempty"`
}

type discoveryBaseline struct {
	BenchmarkVersion string                 `json:"benchmarkVersion"`
	Cases            []discoveryMeasurement `json:"cases"`
	Summary          map[string]int         `json:"summary"`
}

type discoveryMeasurement struct {
	CaseID               string              `json:"caseId"`
	Version              string              `json:"version"`
	Category             string              `json:"category"`
	Mode                 domain.AnalysisMode `json:"mode"`
	InputFingerprint     string              `json:"inputFingerprint"`
	ExecutionFingerprint string              `json:"executionFingerprint"`
	RuleVersions         map[string]string   `json:"ruleVersions"`
	Operations           []operationRecord   `json:"operations,omitempty"`
	Assertions           []assertionRecord   `json:"assertions"`
	Outcome              string              `json:"outcome"`
	KnownFailureReason   string              `json:"knownFailureReason,omitempty"`
}

// operationRecord omits the derived artifact hash on purpose: it covers raw
// float results whose last bits may differ across architectures, while the
// assertions record the measured values at a stable precision.
type operationRecord struct {
	Operation  string `json:"operation"`
	SpecHash   string `json:"specHash"`
	Candidates int    `json:"candidates"`
	Error      string `json:"error,omitempty"`
}

type assertionRecord struct {
	Index    int    `json:"index"`
	Passed   bool   `json:"passed"`
	Observed string `json:"observed"`
}

func TestDiscoveryBenchmarkMatchesBaselineWhenCoreRunsEveryCase(t *testing.T) {
	cases := loadDiscoveryCases(t)
	perCategory := map[string]int{}
	for _, c := range cases {
		perCategory[c.Category]++
	}
	for _, category := range []string{"POSITIVE_DISCOVERY", "VALID_NO_DISCOVERY", "FALSE_ASSOCIATION_GUARD"} {
		if perCategory[category] < 3 {
			t.Fatalf("benchmark needs at least 3 %s cases, has %d", category, perCategory[category])
		}
	}

	baseline := discoveryBaseline{BenchmarkVersion: discoveryBenchmarkVersion, Summary: map[string]int{}}
	for _, c := range cases {
		m := runDiscoveryCase(t, c)
		baseline.Cases = append(baseline.Cases, m)
		baseline.Summary[m.Outcome]++
		if m.Outcome == "FAIL" {
			t.Errorf("%s: deterministic invariant failed: %+v", c.CaseID, m.Assertions)
		}
	}

	got, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join(discoveryDir, "baseline.json")
	if os.Getenv("GOLDEN_UPDATE") == "1" {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read baseline (regenerate with GOLDEN_UPDATE=1): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("discovery benchmark drifted from %s; review the change, bump case versions if expected changed, then regenerate with GOLDEN_UPDATE=1.\n--- got ---\n%s", path, got)
	}
}

// Re-importing the same file under a new document ID must not add a second
// dataset identity: the input snapshot still names one source.
func TestDiscoveryBenchmarkDuplicateSourceAddsNoDatasetIdentityWhenReimported(t *testing.T) {
	var dup discoveryCase
	for _, c := range loadDiscoveryCases(t) {
		if c.CaseID == "DB-F03" {
			dup = c
		}
	}
	if dup.CaseID == "" {
		t.Fatal("DB-F03 duplicate-source case missing")
	}
	var once []discoveryDoc
	for _, d := range dup.Input.Documents {
		if d.ID != "visits-2023-reimport" {
			once = append(once, d)
		}
	}
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	unique := service.BuildInputSnapshot(discoveryDocuments(once), at)
	reimported := service.BuildInputSnapshot(discoveryDocuments(dup.Input.Documents), at)
	if fmt.Sprint(unique.DatasetHashes) != fmt.Sprint(reimported.DatasetHashes) {
		t.Fatalf("re-importing the same file changed dataset hashes: %v -> %v", unique.DatasetHashes, reimported.DatasetHashes)
	}
	if len(unique.Datasets) != len(reimported.Datasets) {
		t.Fatalf("re-importing the same dataset added a dataset provenance: %d -> %d", len(unique.Datasets), len(reimported.Datasets))
	}
}

func loadDiscoveryCases(t *testing.T) []discoveryCase {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(discoveryDir, "cases", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no discovery cases: %v", err)
	}
	sort.Strings(paths)
	out := make([]discoveryCase, 0, len(paths))
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var c discoveryCase
		if err := json.Unmarshal(raw, &c); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		out = append(out, c)
	}
	return out
}

func runDiscoveryCase(t *testing.T, c discoveryCase) discoveryMeasurement {
	t.Helper()
	if c.Mode == domain.AnalysisModeResearchReview {
		if c.Claim == nil {
			t.Fatalf("%s: RESEARCH_REVIEW case needs a claim", c.CaseID)
		}
		artifacts := []domain.InputArtifact{{Reference: c.Claim.SourceReference, Kind: domain.ArtifactResearchReport}}
		if err := domain.ValidateAnalysisModeInput(c.Mode, artifacts, []domain.ResearchClaim{*c.Claim}); err != nil {
			t.Fatalf("%s: %v", c.CaseID, err)
		}
	}
	exec, err := service.BuildExecutionSnapshot(service.Settings{}, c.Mode, buildinfo.Info{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	rules := map[string]string{"temporalOperation": analytical.OperationSchema + "/v" + analytical.OperationVersion}
	for k, v := range exec.RuleVersions {
		rules[k] = v
	}
	execFP, err := service.Fingerprint(struct {
		Execution service.ExecutionConfig `json:"execution"`
		Rules     map[string]string       `json:"rules"`
	}{exec.ExecutionConfig, rules})
	if err != nil {
		t.Fatal(err)
	}
	inputFP, err := service.Fingerprint(struct {
		ResearchQuestion string                     `json:"researchQuestion,omitempty"`
		Claim            *domain.ResearchClaim      `json:"claim,omitempty"`
		Input            discoveryInput             `json:"input"`
		Operations       []analytical.OperationSpec `json:"operations,omitempty"`
	}{c.ResearchQuestion, c.Claim, c.Input, c.Operations})
	if err != nil {
		t.Fatal(err)
	}
	m := discoveryMeasurement{
		CaseID: c.CaseID, Version: c.Version, Category: c.Category, Mode: c.Mode,
		InputFingerprint: inputFP, ExecutionFingerprint: execFP, RuleVersions: rules,
	}

	var outputs []analytical.Artifact
	var opErrors []error
	if len(c.Input.Series) > 0 {
		source := discoveryArtifact(t, c)
		for _, spec := range c.Operations {
			spec.OperationSchema, spec.SchemaVersion = analytical.OperationSchema, analytical.OperationVersion
			rec := operationRecord{Operation: spec.Operation, SpecHash: "sha256:" + spec.Hash().Value}
			out, err := analytical.ApplyTemporalOperation(source, spec)
			if err != nil {
				rec.Error = err.Error()
			} else {
				candidates, cerr := analytical.ToCandidatesForAnalysis(out, "benchmark-"+c.CaseID)
				if cerr != nil {
					t.Fatalf("%s: derived artifact is not candidate-ready: %v", c.CaseID, cerr)
				}
				rec.Candidates = len(candidates)
			}
			outputs, opErrors = append(outputs, out), append(opErrors, err)
			m.Operations = append(m.Operations, rec)
		}
	}
	var pre service.DatasetPreAnalysis
	if len(c.Input.Documents) > 0 {
		pre = service.RunDatasetPreAnalysis(discoveryDocuments(c.Input.Documents), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	}

	known := map[int]bool{}
	if c.Expected.KnownFailure != nil {
		for _, i := range c.Expected.KnownFailure.Assertions {
			known[i] = true
		}
		m.KnownFailureReason = c.Expected.KnownFailure.Reason
	}
	failed, knownFailed := false, false
	for i, a := range c.Expected.Assertions {
		var passed bool
		var observed string
		if a.Preanalysis != "" {
			passed, observed = checkPreanalysis(a, pre)
		} else {
			if a.Op == nil || *a.Op >= len(outputs) {
				t.Fatalf("%s assertion %d: op index out of range", c.CaseID, i)
			}
			passed, observed = checkOperation(a, outputs[*a.Op], opErrors[*a.Op])
		}
		m.Assertions = append(m.Assertions, assertionRecord{Index: i, Passed: passed, Observed: observed})
		switch {
		case known[i] && passed:
			// A fixed known failure must be noticed and recorded, not
			// silently turned into a pass.
			t.Errorf("%s assertion %d is declared a known failure but now passes (%s); remove it from knownFailure and bump the case version", c.CaseID, i, observed)
		case known[i]:
			knownFailed = true
		case !passed:
			failed = true
		}
	}
	switch {
	case failed:
		m.Outcome = "FAIL"
	case knownFailed:
		m.Outcome = "KNOWN_FAILURE"
	default:
		m.Outcome = "PASS"
	}
	return m
}

// discoveryArtifact builds a validated Analytical Artifact from the case
// series. Its hash is the hash of the case input, so it is reproducible.
func discoveryArtifact(t *testing.T, c discoveryCase) analytical.Artifact {
	t.Helper()
	sum := sha256.Sum256(mustJSON(t, c.Input))
	hash := analytical.Hash{Algorithm: "sha256", Value: hex.EncodeToString(sum[:])}
	geography := c.Input.Geography
	if geography == "" {
		geography = "synthetic"
	}
	a := analytical.Artifact{
		ArtifactSchema: analytical.Schema, SchemaVersion: analytical.Version,
		ID: "benchmark:" + c.CaseID, ArtifactHash: hash,
		Producer: "discovery-benchmark", ProducerVersion: "1",
		GeneratedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Datasets:    []analytical.DatasetRef{{ID: "synthetic-" + c.CaseID, Version: c.Version, Hash: hash}},
		Spec:        analytical.SpecRef{Kind: "synthetic", Reference: "testdata/golden/discovery/cases/" + c.CaseID, Hash: hash},
		Population:  analytical.Population{Description: c.Input.Population},
		Computation: analytical.Computation{Engine: "discovery-benchmark", EngineVersion: "1", Deterministic: true, Timezone: "UTC"},
		Provenance:  []analytical.SourceProvenance{{DatasetID: "synthetic-" + c.CaseID, Source: "synthetic benchmark fixture", RetrievedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}},
	}
	var first, last string
	dims := map[string]bool{}
	seenMetric := map[string]bool{}
	for _, s := range c.Input.Series {
		if !seenMetric[s.MetricID] {
			seenMetric[s.MetricID] = true
			a.Metrics = append(a.Metrics, analytical.MetricDefinition{ID: s.MetricID, Name: s.Name, Unit: s.Unit, Aggregation: "sum", Version: s.MetricID + "-v1"})
		}
		periods := make([]string, 0, len(s.Values))
		for p := range s.Values {
			periods = append(periods, p)
		}
		sort.Strings(periods)
		add := func(period string, v *float64) {
			r := analytical.Result{
				MetricID: s.MetricID, Dimensions: s.Dimensions,
				Period:   analytical.Period{Start: period, End: period, Basis: c.Input.Basis},
				Temporal: &analytical.TemporalMetadata{ObservedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Origin: "observed", Geography: geography, ValueBasis: "not_applicable"},
			}
			if v == nil {
				r.Missing = true
			} else {
				r.Value = mustJSON(t, *v)
			}
			a.Results = append(a.Results, r)
		}
		for _, p := range periods {
			add(p, s.Values[p])
			if first == "" || p < first {
				first = p
			}
			if p > last {
				last = p
			}
		}
		for p, v := range s.Duplicates {
			v := v
			add(p, &v)
		}
		for k := range s.Dimensions {
			dims[k] = true
		}
	}
	for k := range dims {
		a.Dimensions = append(a.Dimensions, k)
	}
	sort.Strings(a.Dimensions)
	a.Period = analytical.Period{Start: first, End: last, Basis: c.Input.Basis}
	if err := a.Validate(); err != nil {
		t.Fatalf("%s: synthetic artifact invalid: %v", c.CaseID, err)
	}
	return a
}

func discoveryDocuments(in []discoveryDoc) []*domain.Document {
	docs := make([]*domain.Document, 0, len(in))
	for _, d := range in {
		manifest := service.AcquisitionManifest{
			SourceName: "synthetic benchmark fixture", SourceURL: "https://example.invalid/" + d.DatasetID,
			DatasetID: d.DatasetID, RetrievalMethod: service.RetrievalDownload,
			RetrievedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Geography:   d.Location, Period: d.Period, Unit: d.Unit,
			PopulationScope: d.PopulationScope, PopulationDefinitionID: d.PopulationDefinitionID,
			SchemaID: d.SchemaID, SchemaVersion: d.SchemaVersion, FileHash: d.FileHash,
		}
		meta := manifest.DocumentMetadata(map[string]string{
			"period": d.Period, "event_type": d.EventType, "location": d.Location, "record_count": d.RecordCount,
		})
		docs = append(docs, &domain.Document{
			ID: d.ID, Source: domain.SourceDataset, Title: d.EventType + " " + d.Period,
			Content:   fmt.Sprintf("Dataset observation: period=%s; location=%s; event_type=%s; record_count=%s. Synthetic benchmark fixture.", d.Period, d.Location, d.EventType, d.RecordCount),
			Metadata:  meta,
			CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		})
	}
	return docs
}

func checkOperation(a discoveryAssertion, out analytical.Artifact, opErr error) (bool, string) {
	if a.Error != "" {
		if opErr == nil {
			return false, "operation succeeded"
		}
		return strings.Contains(opErr.Error(), a.Error), "error: " + opErr.Error()
	}
	if opErr != nil {
		return false, "error: " + opErr.Error()
	}
	if a.Limitation != "" {
		for _, q := range out.Quality {
			if q.Code == "LIMITATION" && strings.Contains(q.Message, a.Limitation) {
				return true, "limitation: " + q.Message
			}
		}
		return false, "limitation absent"
	}
	var matched []analytical.Result
	for _, r := range out.Results {
		if a.Metric != "" && !strings.HasSuffix(r.MetricID, ":"+a.Metric) {
			continue
		}
		if a.Period != "" && r.Period.Start != a.Period {
			continue
		}
		if !dimsMatch(r.Dimensions, a.Dimensions) {
			continue
		}
		matched = append(matched, r)
	}
	if a.Count != nil {
		n := 0
		var where []string
		for _, r := range matched {
			if a.Flag == "" || resultHasFlag(r, a.Flag) {
				n++
				where = append(where, r.Period.Start)
			}
		}
		return n == *a.Count, fmt.Sprintf("count=%d %v", n, where)
	}
	if len(matched) == 0 {
		return false, "no matching result"
	}
	passed := true
	var observed []string
	for _, r := range matched {
		v, ok := discoveryNumber(r)
		desc := "missing"
		if ok {
			desc = fmt.Sprintf("%.6g", v)
		}
		if len(r.Quality) > 0 {
			var codes []string
			for _, q := range r.Quality {
				codes = append(codes, q.Code)
			}
			desc += " " + strings.Join(codes, ",")
		}
		observed = append(observed, r.Period.Start+"="+desc)
		if a.Flag != "" && !resultHasFlag(r, a.Flag) {
			passed = false
		}
		if a.Missing != nil && r.Missing != *a.Missing {
			passed = false
		}
		if a.Min != nil || a.Max != nil || a.AbsMax != nil {
			if !ok || (a.Min != nil && v < *a.Min) || (a.Max != nil && v > *a.Max) || (a.AbsMax != nil && math.Abs(v) > *a.AbsMax) {
				passed = false
			}
		}
	}
	return passed, strings.Join(observed, "; ")
}

func checkPreanalysis(a discoveryAssertion, pre service.DatasetPreAnalysis) (bool, string) {
	count := func(n int) (bool, string) {
		if a.Count == nil {
			return n > 0, fmt.Sprintf("count=%d", n)
		}
		return n == *a.Count, fmt.Sprintf("count=%d", n)
	}
	switch a.Preanalysis {
	case "observations":
		return count(len(pre.Observations))
	case "comparisons":
		var steps []string
		for _, c := range pre.Comparisons {
			steps = append(steps, c.FromPeriod+"->"+c.ToPeriod)
		}
		ok, obs := count(len(pre.Comparisons))
		return ok, obs + " " + strings.Join(steps, ",")
	case "datasetHashes":
		return count(len(pre.DatasetHashes))
	case "manifests":
		return count(len(pre.Manifests))
	case "compatibilityWarnings":
		n := 0
		for _, w := range pre.CompatibilityWarnings {
			if a.Code == "" || string(w.Code) == a.Code {
				n++
			}
		}
		return count(n)
	case "notes":
		for _, n := range pre.Notes {
			if strings.Contains(n, a.Contains) {
				return true, n
			}
		}
		return false, fmt.Sprintf("no note containing %q in %v", a.Contains, pre.Notes)
	}
	return false, "unknown preanalysis target " + a.Preanalysis
}

func dimsMatch(have, want map[string]string) bool {
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}

func resultHasFlag(r analytical.Result, code string) bool {
	for _, q := range r.Quality {
		if q.Code == code {
			return true
		}
	}
	return false
}

func discoveryNumber(r analytical.Result) (float64, bool) {
	if r.Missing || len(r.Value) == 0 {
		return 0, false
	}
	var v float64
	if err := json.Unmarshal(r.Value, &v); err != nil {
		return 0, false
	}
	return v, true
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
