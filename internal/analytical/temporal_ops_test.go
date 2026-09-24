package analytical

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"regexp"
	"strings"
	"testing"

	"insight-lab/internal/domain"
)

func tradeFX(t *testing.T) Artifact {
	t.Helper()
	b, err := os.ReadFile("../../contracts/analytical-artifact/v1/fixtures/trade-fx-temporal.json")
	if err != nil {
		t.Fatal(err)
	}
	a, err := Import(b)
	if err != nil {
		t.Fatal(err)
	}
	return *a
}

func spec(op, metric string) OperationSpec {
	return OperationSpec{OperationSchema: OperationSchema, SchemaVersion: OperationVersion, Operation: op, MetricID: metric}
}

var exportSeries = map[string]string{"origin": "JP", "destination": "EU"}

func apply(t *testing.T, s OperationSpec) Artifact {
	t.Helper()
	out, err := ApplyTemporalOperation(tradeFX(t), s)
	if err != nil {
		t.Fatalf("%s: %v", s.Operation, err)
	}
	return out
}

func resultAt(t *testing.T, a Artifact, metricSuffix, period string) Result {
	t.Helper()
	for _, r := range a.Results {
		if strings.HasSuffix(r.MetricID, metricSuffix) && r.Period.Start == period {
			return r
		}
	}
	t.Fatalf("no %s result for %s", metricSuffix, period)
	return Result{}
}

func value(t *testing.T, r Result) float64 {
	t.Helper()
	v, ok := number(r)
	if !ok {
		t.Fatalf("result %s %s is not numeric: missing=%v flags=%v", r.MetricID, r.Period.Start, r.Missing, r.Quality)
	}
	return v
}

func hasFlag(r Result, code string) bool {
	for _, q := range r.Quality {
		if q.Code == code {
			return true
		}
	}
	return false
}

func TestApplyTemporalOperationYoYOnMonthlySeriesUsesTwelveWindowLag(t *testing.T) {
	s := spec(OpYoY, "export_value")
	s.Series = exportSeries
	out := apply(t, s)
	// 2024-02: 100+2*13+20=146 vs 2023-02: 102.
	if got := value(t, resultAt(t, out, "change_ratio", "2024-02")); math.Abs(got-(146.0-102)/102) > 1e-12 {
		t.Fatalf("yoy 2024-02 = %v", got)
	}
	r := resultAt(t, out, "change_ratio", "2024-10")
	if !r.Missing || !hasFlag(r, "MISSING_OR_GAP") {
		t.Fatalf("2024-10 vs missing 2023-10 must be missing, got %+v", r)
	}
	if r := resultAt(t, out, "change_ratio", "2024-05"); !r.Missing {
		t.Fatalf("unreported window 2024-05 must be missing, got %+v", r)
	}
}

func TestApplyTemporalOperationIsReproducibleAndCarriesProvenance(t *testing.T) {
	s := spec(OpRollingMean, "export_volume")
	s.Series, s.Window = exportSeries, 3
	a, b := apply(t, s), apply(t, s)
	if a.ArtifactHash != b.ArtifactHash || a.ID != b.ID || a.ReproducibilityKey() != b.ReproducibilityKey() {
		t.Fatal("same source and spec must yield the identical derived artifact")
	}
	src := tradeFX(t)
	if a.Parameters["sourceArtifactId"] != src.ID || a.Spec.Kind != "temporal-operation" || a.Computation.Deterministic != true {
		t.Fatalf("provenance: params=%v spec=%+v", a.Parameters, a.Spec)
	}
	refs := strings.Join(a.Provenance[0].TransformationRefs, " ")
	if !strings.Contains(refs, "artifact:"+src.ID) || !strings.Contains(refs, "temporal-operation:rolling_mean@1") {
		t.Fatalf("transformation refs = %s", refs)
	}
	for _, r := range a.Results {
		if r.Temporal != nil && r.Temporal.Origin != "derived" {
			t.Fatal("derived results must be marked origin=derived")
		}
	}
	if r := resultAt(t, a, "rolling_mean", "2024-06"); !r.Missing || !hasFlag(r, "INCOMPLETE_WINDOW") {
		t.Fatalf("window containing the 2024-05 gap must be missing, got %+v", r)
	}
	s.Window = 4
	if c := apply(t, s); c.ReproducibilityKey() == a.ReproducibilityKey() {
		t.Fatal("different parameters must change the reproducibility key")
	}
}

func TestApplyTemporalOperationDerivedArtifactsBecomeNeutralCandidates(t *testing.T) {
	s := spec(OpIndexedBaseline, "export_value")
	s.Series, s.BaselinePeriod = exportSeries, "2023-01"
	out := apply(t, s)
	if got := value(t, resultAt(t, out, "index", "2023-02")); math.Abs(got-102) > 1e-9 {
		t.Fatalf("index 2023-02 = %v", got)
	}
	cs, err := ToCandidatesForAnalysis(out, "analysis-derived")
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != len(out.Results) || cs[0].Evidence.Type != domain.EvidenceNeutral {
		t.Fatalf("candidate evidence type = %q", cs[0].Evidence.Type)
	}
}

func TestApplyTemporalOperationUnitPriceAndShare(t *testing.T) {
	s := spec(OpUnitPrice, "export_value")
	s.Series, s.ReferenceMetricID = exportSeries, "export_volume"
	out := apply(t, s)
	if got := value(t, resultAt(t, out, "unit_price", "2023-01")); got != 2 {
		t.Fatalf("unit price 2023-01 = %v", got)
	}
	if out.Metrics[0].Unit != "JPY bn/kt" {
		t.Fatalf("unit = %q", out.Metrics[0].Unit)
	}
}

func TestApplyTemporalOperationFindsBreakCandidateAndAnomaly(t *testing.T) {
	cp := spec(OpChangePointCandidate, "export_volume")
	cp.Series = exportSeries
	if _, err := ApplyTemporalOperation(tradeFX(t), cp); err != nil {
		t.Fatal(err)
	}
	an := spec(OpAnomalyCandidate, "export_value")
	an.Series = exportSeries
	out := apply(t, an)
	if r := resultAt(t, out, "robust_z", "2023-07"); !hasFlag(r, "ANOMALY_CANDIDATE") {
		t.Fatalf("2023-07 spike should be an anomaly candidate: %+v", r)
	}
	if r := resultAt(t, out, "robust_z", "2023-10"); !r.Missing {
		t.Fatal("missing input must stay missing")
	}
}

func TestApplyTemporalOperationLaggedAndControlComparison(t *testing.T) {
	lag := spec(OpLaggedComparison, "export_volume")
	lag.Series, lag.ReferenceMetricID, lag.ReferenceSeries, lag.MaxLag = exportSeries, "fx_eurjpy", map[string]string{"pair": "EURJPY"}, 2
	out := apply(t, lag)
	if len(out.Results) != 5 {
		t.Fatalf("want lags -2..2, got %d results", len(out.Results))
	}
	ctl := spec(OpControlComparison, "export_value")
	ctl.Series, ctl.ReferenceMetricID, ctl.ReferenceSeries, ctl.BreakPeriod = exportSeries, "fx_eurjpy", map[string]string{"pair": "EURJPY"}, "2024-01"
	out = apply(t, ctl)
	if len(out.Results) != 3 {
		t.Fatalf("control comparison results = %d", len(out.Results))
	}
	ba := spec(OpBeforeAfter, "export_volume")
	ba.Series, ba.BreakPeriod = exportSeries, "2024-01"
	apply(t, ba)
}

// twoDestinations adds a second export series (JP→US) so selections become
// ambiguous unless narrowed.
func twoDestinations(t *testing.T) Artifact {
	t.Helper()
	a := tradeFX(t)
	var extra []Result
	for _, r := range a.Results {
		if r.MetricID == "export_volume" {
			c := r
			c.Dimensions = map[string]string{"origin": "JP", "destination": "US"}
			v, _ := number(r)
			c.Value, _ = json.Marshal(v * 2)
			extra = append(extra, c)
		}
	}
	a.Results = append(a.Results, extra...)
	return a
}

func TestApplyTemporalOperationCohortComparisonIndexesEachCohort(t *testing.T) {
	s := spec(OpCohortComparison, "export_volume")
	s.CohortDimension = "destination"
	out, err := ApplyTemporalOperation(twoDestinations(t), s)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range out.Results {
		if r.Period.Start == "2023-01" && value(t, r) != 100 {
			t.Fatalf("cohort %v does not start at 100", r.Dimensions)
		}
	}
}

func TestApplyTemporalOperationRejectsAmbiguousOrUndefinedRequests(t *testing.T) {
	cases := map[string]OperationSpec{
		"series not narrowed":       spec(OpRollingMean, "export_volume"),
		"mom on yearly unsupported": {OperationSchema: OperationSchema, SchemaVersion: OperationVersion, Operation: OpMoM, MetricID: "export_value"},
		"unknown operation":         spec("forecast", "export_value"),
		"wrong schema version":      {OperationSchema: OperationSchema, SchemaVersion: "2", Operation: OpYoY, MetricID: "export_value"},
	}
	cases["series not narrowed"] = func() OperationSpec { s := cases["series not narrowed"]; s.Window = 3; return s }()
	for name, s := range cases {
		t.Run(name, func(t *testing.T) {
			src := twoDestinations(t)
			if name == "mom on yearly unsupported" {
				src = temporalFixture(t)
			}
			if _, err := ApplyTemporalOperation(src, s); !errors.Is(err, ErrInvalidOperation) {
				t.Fatalf("want ErrInvalidOperation, got %v", err)
			}
		})
	}
}

// causal matches wording that asserts a cause. The analytical layer may say
// that it does not explain movement, but must never attribute one.
var causal = regexp.MustCompile(`(?i)\b(caused|causes|because of|due to|drove|driven by|led to|resulted in|impact of|effect of)\b`)

func TestTemporalOperationsNeverEmitCausalWording(t *testing.T) {
	ops := []OperationSpec{}
	add := func(op string, mut func(*OperationSpec)) {
		s := spec(op, "export_value")
		s.Series = exportSeries
		mut(&s)
		ops = append(ops, s)
	}
	none := func(*OperationSpec) {}
	add(OpYoY, none)
	add(OpCAGR, none)
	add(OpRollingDelta, func(s *OperationSpec) { s.Window = 2 })
	add(OpBeforeAfter, func(s *OperationSpec) { s.BreakPeriod = "2024-01" })
	add(OpAnomalyCandidate, none)
	add(OpChangePointCandidate, none)
	add(OpShare, func(s *OperationSpec) { s.ReferenceMetricID = "export_volume" })
	add(OpControlComparison, func(s *OperationSpec) {
		s.ReferenceMetricID, s.ReferenceSeries, s.BreakPeriod = "fx_eurjpy", map[string]string{"pair": "EURJPY"}, "2024-01"
	})
	add(OpLaggedComparison, func(s *OperationSpec) {
		s.ReferenceMetricID, s.ReferenceSeries, s.MaxLag = "fx_eurjpy", map[string]string{"pair": "EURJPY"}, 1
	})
	add(OpCohortComparison, func(s *OperationSpec) { s.Series, s.MetricID, s.CohortDimension = nil, "export_volume", "origin" })
	for _, s := range ops {
		out, err := ApplyTemporalOperation(tradeFX(t), s)
		if s.Operation == OpCohortComparison {
			if !errors.Is(err, ErrInvalidOperation) {
				t.Fatalf("single-cohort source must be rejected, got %v", err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", s.Operation, err)
		}
		b, _ := json.Marshal(out)
		if m := causal.FindString(string(b)); m != "" {
			t.Fatalf("%s emitted causal wording %q", s.Operation, m)
		}
	}
}

func TestOperationSpecMatchesTemporalOperationSchema(t *testing.T) {
	b, err := os.ReadFile("../../contracts/analytical-artifact/v1/temporal-operation.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Properties map[string]struct {
			Enum []string `json:"enum"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	all := OperationSpec{Series: map[string]string{"a": "b"}, ReferenceSeries: map[string]string{"a": "b"}, ReferenceMetricID: "r",
		CohortDimension: "c", Lag: 1, MaxLag: 1, Window: 2, BaselinePeriod: "p", BreakPeriod: "p", Threshold: 1, MinSegment: 2}
	raw, _ := json.Marshal(all)
	_ = json.Unmarshal(raw, &fields)
	if len(fields) != len(doc.Properties) {
		t.Fatalf("Go spec has %d fields, schema %d properties", len(fields), len(doc.Properties))
	}
	for k := range fields {
		if _, ok := doc.Properties[k]; !ok {
			t.Errorf("schema lacks %s", k)
		}
	}
	for _, op := range doc.Properties["operation"].Enum {
		s := spec(op, "m")
		s.Lag, s.Window, s.ReferenceMetricID, s.BaselinePeriod, s.BreakPeriod, s.CohortDimension = 1, 2, "r", "p", "p", "c"
		if err := s.Validate(); err != nil {
			t.Errorf("schema operation %s rejected by Go: %v", op, err)
		}
	}
}
