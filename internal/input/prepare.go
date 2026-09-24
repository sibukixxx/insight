package input

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

// PreparationKindCSVAggregate is the one declarative preparation Core ships:
// deterministic grouped aggregates over a CSV stream. Its output is an
// Analytical Artifact, the existing prepared-input boundary (#69).
const PreparationKindCSVAggregate = "csv-aggregate/v1"

// DefaultMaxGroups bounds memory: state grows with groups, never with rows.
const DefaultMaxGroups = 100_000

type PreparationMetric struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Column      string `json:"column,omitempty"`
	Aggregation string `json:"aggregation"`
	Unit        string `json:"unit"`
}

type PreparationPeriod struct {
	Start string `json:"start"`
	End   string `json:"end"`
	Basis string `json:"basis,omitempty"`
}

type PreparationPopulation struct {
	Description string `json:"description"`
	Unit        string `json:"unit,omitempty"`
}

// PreparationSpec declares how raw rows become deterministic results.
type PreparationSpec struct {
	Kind             string                `json:"kind"`
	Metrics          []PreparationMetric   `json:"metrics"`
	DimensionColumns []string              `json:"dimensionColumns,omitempty"`
	PeriodColumn     string                `json:"periodColumn,omitempty"`
	Period           *PreparationPeriod    `json:"period,omitempty"`
	Population       PreparationPopulation `json:"population"`
}

func (s PreparationSpec) Validate() error {
	fail := func(format string, args ...any) error {
		return fmt.Errorf("preparation: "+format, args...)
	}
	if s.Kind != PreparationKindCSVAggregate {
		return fail("unsupported kind %q (supported: %s)", s.Kind, PreparationKindCSVAggregate)
	}
	if len(s.Metrics) == 0 || strings.TrimSpace(s.Population.Description) == "" {
		return fail("metrics and population.description are required")
	}
	if s.PeriodColumn == "" && (s.Period == nil || s.Period.Start == "" || s.Period.End == "") {
		return fail("periodColumn or period is required; a period is never guessed")
	}
	seen := map[string]bool{}
	for _, m := range s.Metrics {
		if m.ID == "" || m.Name == "" || m.Unit == "" || seen[m.ID] {
			return fail("each metric needs a unique id, a name and a unit")
		}
		seen[m.ID] = true
		switch m.Aggregation {
		case "count":
		case "sum", "mean", "min", "max":
			if m.Column == "" {
				return fail("metric %q aggregation %s requires a column", m.ID, m.Aggregation)
			}
		default:
			return fail("metric %q has unsupported aggregation %q", m.ID, m.Aggregation)
		}
	}
	return nil
}

// Partial is a mergeable aggregate for one (group, metric). The sum is an
// exact rational (encoded as a string in JSON), so partition order never
// changes a result: floating-point addition is not associative.
type Partial struct {
	Rows    int64
	N       int64
	Min     float64
	Max     float64
	Invalid int64
	acc     *big.Rat
}

type partialJSON struct {
	Rows    int64   `json:"rows"`
	N       int64   `json:"n"`
	Sum     string  `json:"sum,omitempty"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Invalid int64   `json:"invalid"`
}

func (p Partial) MarshalJSON() ([]byte, error) {
	return json.Marshal(partialJSON{p.Rows, p.N, p.sum().RatString(), p.Min, p.Max, p.Invalid})
}

func (p *Partial) UnmarshalJSON(b []byte) error {
	var v partialJSON
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*p = Partial{Rows: v.Rows, N: v.N, Min: v.Min, Max: v.Max, Invalid: v.Invalid}
	if v.Sum != "" {
		r, ok := new(big.Rat).SetString(v.Sum)
		if !ok {
			return fmt.Errorf("partial: invalid sum %q", v.Sum)
		}
		p.acc = r
	}
	return nil
}

func (p Partial) sum() *big.Rat {
	if p.acc == nil {
		return new(big.Rat)
	}
	return p.acc
}

func (p *Partial) add(r *big.Rat) {
	if p.acc == nil {
		p.acc = new(big.Rat)
	}
	p.acc.Add(p.acc, r)
}

func (p *Partial) Merge(o Partial) {
	if o.N > 0 && (p.N == 0 || o.Min < p.Min) {
		p.Min = o.Min
	}
	if o.N > 0 && (p.N == 0 || o.Max > p.Max) {
		p.Max = o.Max
	}
	if o.acc != nil {
		p.add(o.acc)
	}
	p.Rows, p.N, p.Invalid = p.Rows+o.Rows, p.N+o.N, p.Invalid+o.Invalid
}

// Group is one period × dimension combination.
type Group struct {
	Period     string             `json:"period"`
	Dimensions map[string]string  `json:"dimensions,omitempty"`
	Metrics    map[string]Partial `json:"metrics"`
}

// Aggregate is mergeable partition output. Merging partials in any order
// yields the same totals (sum/min/max/count are commutative), which is what
// keeps STANDARD and partitioned HEAVY preparation identical.
type Aggregate struct {
	Groups          map[string]*Group `json:"groups"`
	RowsRead        int64             `json:"rowsRead"`
	RowsWithoutTime int64             `json:"rowsWithoutPeriod"`
}

func NewAggregate() *Aggregate { return &Aggregate{Groups: map[string]*Group{}} }

func (a *Aggregate) Merge(o *Aggregate) {
	a.RowsRead += o.RowsRead
	a.RowsWithoutTime += o.RowsWithoutTime
	for k, g := range o.Groups {
		cur, ok := a.Groups[k]
		if !ok {
			cur = &Group{Period: g.Period, Dimensions: g.Dimensions, Metrics: map[string]Partial{}}
			a.Groups[k] = cur
		}
		for id, p := range g.Metrics {
			m := cur.Metrics[id]
			m.Merge(p)
			cur.Metrics[id] = m
		}
	}
}

// Keep selects which rows a partition owns (row index → bool).
type Keep func(rowIndex int64) bool

// AggregateCSV streams r row by row. Only rows accepted by keep are
// aggregated; nil keeps every row.
func AggregateCSV(r io.Reader, spec PreparationSpec, keep Keep, maxGroups int) (*Aggregate, error) {
	if maxGroups <= 0 {
		maxGroups = DefaultMaxGroups
	}
	cr := csv.NewReader(r)
	cr.FieldsPerRecord, cr.ReuseRecord = -1, true
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("preparation: csv header: %w", err)
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.TrimSpace(strings.TrimPrefix(h, "\ufeff"))] = i
	}
	for _, c := range specColumns(spec) {
		if _, ok := idx[c]; !ok {
			return nil, fmt.Errorf("preparation: column %q is not in the data", c)
		}
	}
	cell := func(rec []string, col string) string {
		if i := idx[col]; i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}
	out := NewAggregate()
	for row := int64(0); ; row++ {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("preparation: csv row %d: %w", row+2, err)
		}
		if keep != nil && !keep(row) {
			continue
		}
		out.RowsRead++
		period := ""
		if spec.PeriodColumn != "" {
			if period = cell(rec, spec.PeriodColumn); period == "" {
				out.RowsWithoutTime++
				continue
			}
		}
		dims := map[string]string{}
		key := []string{period}
		for _, d := range spec.DimensionColumns {
			dims[d] = cell(rec, d)
			key = append(key, d+"="+dims[d])
		}
		k := strings.Join(key, "\x1f")
		g, ok := out.Groups[k]
		if !ok {
			if len(out.Groups) >= maxGroups {
				return nil, fmt.Errorf("preparation: more than %d groups; narrow the dimensions", maxGroups)
			}
			g = &Group{Period: period, Metrics: map[string]Partial{}}
			if len(dims) > 0 {
				g.Dimensions = dims
			}
			out.Groups[k] = g
		}
		for _, m := range spec.Metrics {
			p := g.Metrics[m.ID]
			p.Rows++
			if m.Column != "" {
				addValue(&p, cell(rec, m.Column))
			}
			g.Metrics[m.ID] = p
		}
	}
}

func addValue(p *Partial, v string) {
	if v == "" || strings.EqualFold(v, "NA") || strings.EqualFold(v, "null") {
		return
	}
	v = strings.ReplaceAll(v, ",", "")
	f, err := strconv.ParseFloat(v, 64)
	exact, ok := new(big.Rat).SetString(v)
	if err != nil || !ok || math.IsInf(f, 0) || math.IsNaN(f) {
		p.Invalid++
		return
	}
	if p.N == 0 || f < p.Min {
		p.Min = f
	}
	if p.N == 0 || f > p.Max {
		p.Max = f
	}
	p.N++
	p.add(exact)
}

func specColumns(s PreparationSpec) []string {
	set := map[string]bool{}
	for _, m := range s.Metrics {
		if m.Column != "" {
			set[m.Column] = true
		}
	}
	for _, d := range s.DimensionColumns {
		set[d] = true
	}
	if s.PeriodColumn != "" {
		set[s.PeriodColumn] = true
	}
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// SpecJSON is the canonical encoding used for hashing and storage.
func (s PreparationSpec) SpecJSON() []byte {
	b, _ := json.Marshal(s)
	return b
}
