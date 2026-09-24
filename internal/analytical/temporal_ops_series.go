package analytical

import (
	"fmt"
	"sort"
	"time"
)

// point is one calendar window of a series. Value is nil when the source
// result was missing or when the window is absent from the source (a gap);
// unknown is never treated as zero.
type point struct {
	index    int
	period   Period
	value    *float64
	gap      bool
	temporal *TemporalMetadata
}

type series struct {
	metric MetricDefinition
	basis  string
	dims   map[string]string
	points []point // contiguous calendar windows from first to last
}

func calendarIndex(p Period) (int, error) {
	if p.Start != p.End {
		return 0, fmt.Errorf("%w: temporal operations need single-window periods, got %s..%s", ErrInvalidOperation, p.Start, p.End)
	}
	s, _, _, err := periodBounds(p)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidOperation, err)
	}
	switch p.Basis {
	case "calendar-year":
		return s.Year(), nil
	case "calendar-month":
		return s.Year()*12 + int(s.Month()) - 1, nil
	}
	return int(s.Sub(time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)).Hours() / 24), nil
}

func periodAt(basis string, index int) Period {
	var label string
	switch basis {
	case "calendar-year":
		label = fmt.Sprintf("%04d", index)
	case "calendar-month":
		label = fmt.Sprintf("%04d-%02d", index/12, index%12+1)
	default:
		label = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, index).Format("2006-01-02")
	}
	return Period{Start: label, End: label, Basis: basis}
}

func matches(dims, filter map[string]string) bool {
	for k, v := range filter {
		if dims[k] != v {
			return false
		}
	}
	return true
}

// extractSeries selects temporal results of one metric matching filter,
// grouped by their full dimension set. Gaps between the first and last
// window become explicit gap points.
func extractSeries(a Artifact, metricID string, filter map[string]string) ([]series, error) {
	var metric *MetricDefinition
	for i := range a.Metrics {
		if a.Metrics[i].ID == metricID {
			metric = &a.Metrics[i]
		}
	}
	if metric == nil {
		return nil, fmt.Errorf("%w: metric %q not in source artifact", ErrInvalidOperation, metricID)
	}
	groups := map[string]*series{}
	var order []string
	for _, r := range a.Results {
		if r.MetricID != metricID || !matches(r.Dimensions, filter) {
			continue
		}
		if r.Temporal == nil {
			return nil, fmt.Errorf("%w: result of %s for %s has no temporal metadata", ErrInvalidOperation, metricID, r.Period.Start)
		}
		p := r.Period
		if p.Basis == "" {
			p.Basis = a.Period.Basis
		}
		idx, err := calendarIndex(p)
		if err != nil {
			return nil, err
		}
		key := fmt.Sprint(sortedDims(r.Dimensions))
		g, ok := groups[key]
		if !ok {
			g = &series{metric: *metric, basis: p.Basis, dims: r.Dimensions}
			groups[key] = g
			order = append(order, key)
		}
		if g.basis != p.Basis {
			return nil, fmt.Errorf("%w: series mixes period bases %s and %s", ErrInvalidOperation, g.basis, p.Basis)
		}
		pt := point{index: idx, period: p, temporal: r.Temporal}
		if v, ok := number(r); ok {
			pt.value = &v
		}
		g.points = append(g.points, pt)
	}
	out := make([]series, 0, len(order))
	for _, key := range order {
		g := groups[key]
		sort.Slice(g.points, func(i, j int) bool { return g.points[i].index < g.points[j].index })
		var filled []point
		for i, pt := range g.points {
			if i > 0 && pt.index == g.points[i-1].index {
				return nil, fmt.Errorf("%w: duplicate window %s in one series", ErrInvalidOperation, pt.period.Start)
			}
			if i > 0 {
				for gap := g.points[i-1].index + 1; gap < pt.index; gap++ {
					filled = append(filled, point{index: gap, period: periodAt(g.basis, gap), gap: true})
				}
			}
			filled = append(filled, pt)
		}
		g.points = filled
		out = append(out, *g)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: no temporal results of %s match the series selection", ErrInvalidOperation, metricID)
	}
	return out, nil
}

// singleSeries requires the selection to identify exactly one series.
func singleSeries(a Artifact, metricID string, filter map[string]string) (series, error) {
	all, err := extractSeries(a, metricID, filter)
	if err != nil {
		return series{}, err
	}
	if len(all) != 1 {
		return series{}, fmt.Errorf("%w: selection matches %d series of %s; narrow it with series/referenceSeries", ErrInvalidOperation, len(all), metricID)
	}
	return all[0], nil
}

func sortedDims(d map[string]string) []string {
	out := make([]string, 0, len(d))
	for k, v := range d {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}

func (s series) at(index int) (point, bool) {
	for _, p := range s.points {
		if p.index == index {
			return p, true
		}
	}
	return point{}, false
}
