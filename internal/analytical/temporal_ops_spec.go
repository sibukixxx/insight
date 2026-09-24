package analytical

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Temporal Analytics Pack (#73). Operations are declarative and versioned.
// They turn temporal Analytical Artifact results into a derived Analytical
// Artifact whose results are neutral Observation candidates. They describe
// how values moved; they never explain why.
const (
	OperationSchema  = "insight-lab.temporal-operation"
	OperationVersion = "1"
	operationEngine  = "insight-lab.temporal-ops"
)

var ErrInvalidOperation = errors.New("invalid temporal operation")

// Operation names.
const (
	OpYoY                  = "yoy"
	OpMoM                  = "mom"
	OpQoQ                  = "qoq"
	OpPeriodOverPeriod     = "period_over_period"
	OpCAGR                 = "cagr"
	OpRollingMean          = "rolling_mean"
	OpRollingDelta         = "rolling_delta"
	OpShare                = "share"
	OpUnitPrice            = "unit_price"
	OpIndexedBaseline      = "indexed_baseline"
	OpLaggedComparison     = "lagged_comparison"
	OpBeforeAfter          = "before_after"
	OpCohortComparison     = "cohort_comparison"
	OpControlComparison    = "control_comparison"
	OpAnomalyCandidate     = "anomaly_candidate"
	OpChangePointCandidate = "change_point_candidate"
)

// OperationSpec is the declarative input. Its canonical JSON hash is the
// derived artifact's spec hash, so an external producer (for example a
// DuckDB job) that runs the same spec over the same source yields the same
// reproducibility key.
type OperationSpec struct {
	OperationSchema string `json:"operationSchema"`
	SchemaVersion   string `json:"schemaVersion"`
	Operation       string `json:"operation"`
	MetricID        string `json:"metricId"`
	// Series selects one series by dimension equality.
	Series map[string]string `json:"series,omitempty"`
	// ReferenceMetricID / ReferenceSeries select the second series for
	// share, unit_price, lagged_comparison and control_comparison.
	ReferenceMetricID string            `json:"referenceMetricId,omitempty"`
	ReferenceSeries   map[string]string `json:"referenceSeries,omitempty"`
	CohortDimension   string            `json:"cohortDimension,omitempty"`
	Lag               int               `json:"lag,omitempty"`
	MaxLag            int               `json:"maxLag,omitempty"`
	Window            int               `json:"window,omitempty"`
	BaselinePeriod    string            `json:"baselinePeriod,omitempty"`
	BreakPeriod       string            `json:"breakPeriod,omitempty"`
	Threshold         float64           `json:"threshold,omitempty"`
	MinSegment        int               `json:"minSegment,omitempty"`
}

func (s OperationSpec) Hash() Hash {
	b, _ := json.Marshal(s)
	sum := sha256.Sum256(b)
	return Hash{Algorithm: "sha256", Value: hex.EncodeToString(sum[:])}
}

// Validate checks structural requirements per operation.
func (s OperationSpec) Validate() error {
	fail := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrInvalidOperation, fmt.Sprintf(format, args...))
	}
	if s.OperationSchema != OperationSchema || s.SchemaVersion != OperationVersion {
		return fail("unsupported operation schema %q version %q", s.OperationSchema, s.SchemaVersion)
	}
	if strings.TrimSpace(s.MetricID) == "" {
		return fail("metricId is required")
	}
	needsRef := func() error {
		if s.ReferenceMetricID == "" && len(s.ReferenceSeries) == 0 {
			return fail("%s requires referenceMetricId or referenceSeries", s.Operation)
		}
		return nil
	}
	switch s.Operation {
	case OpYoY, OpMoM, OpQoQ, OpCAGR, OpAnomalyCandidate:
	case OpPeriodOverPeriod:
		if s.Lag < 1 {
			return fail("period_over_period requires lag >= 1")
		}
	case OpRollingMean, OpRollingDelta:
		if s.Window < 2 {
			return fail("%s requires window >= 2", s.Operation)
		}
	case OpShare, OpUnitPrice, OpControlComparison:
		if err := needsRef(); err != nil {
			return err
		}
		if s.Operation == OpControlComparison && s.BreakPeriod == "" {
			return fail("control_comparison requires breakPeriod")
		}
	case OpLaggedComparison:
		if err := needsRef(); err != nil {
			return err
		}
		if s.MaxLag < 0 {
			return fail("maxLag must not be negative")
		}
	case OpIndexedBaseline:
		if s.BaselinePeriod == "" {
			return fail("indexed_baseline requires baselinePeriod")
		}
	case OpBeforeAfter:
		if s.BreakPeriod == "" {
			return fail("before_after requires breakPeriod")
		}
	case OpCohortComparison:
		if s.CohortDimension == "" {
			return fail("cohort_comparison requires cohortDimension")
		}
	case OpChangePointCandidate:
		if s.MinSegment != 0 && s.MinSegment < 2 {
			return fail("minSegment must be >= 2")
		}
	default:
		return fail("unknown operation %q", s.Operation)
	}
	return nil
}
