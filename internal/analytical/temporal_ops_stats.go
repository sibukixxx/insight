package analytical

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

// segment accumulates recorded values on one side of a break.
type segment struct {
	sum      float64
	n        int
	first    *point
	last     *point
	excluded int
}

func (s *segment) addPoint(p *point) {
	if p.value == nil {
		s.excluded++
		return
	}
	if s.first == nil {
		s.first = p
	}
	s.last = p
	s.sum += *p.value
	s.n++
}

func (s segment) mean() *float64 {
	if s.n == 0 {
		return nil
	}
	m := s.sum / float64(s.n)
	return &m
}

func (s segment) period(basis string) (Period, *TemporalMetadata) {
	if s.first == nil {
		return Period{}, nil
	}
	return Period{Start: s.first.period.Start, End: s.last.period.End, Basis: basis}, s.last.temporal
}

func split(s series, breakPeriod string) (segment, segment, error) {
	idx, err := calendarIndex(Period{Start: breakPeriod, End: breakPeriod, Basis: s.basis})
	if err != nil {
		return segment{}, segment{}, err
	}
	var before, after segment
	for i := range s.points {
		if s.points[i].index < idx {
			before.addPoint(&s.points[i])
		} else {
			after.addPoint(&s.points[i])
		}
	}
	if before.n == 0 || after.n == 0 {
		return before, after, fmt.Errorf("%w: breakPeriod %s leaves no recorded values on one side", ErrInvalidOperation, breakPeriod)
	}
	return before, after, nil
}

func excludedFlag(n int) []QualityFlag {
	if n == 0 {
		return nil
	}
	return []QualityFlag{{Code: "EXCLUDED_MISSING", Message: strconv.Itoa(n) + " missing windows were excluded"}}
}

func beforeAfter(d *derived, s series, spec OperationSpec) error {
	before, after, err := split(s, spec.BreakPeriod)
	if err != nil {
		return err
	}
	bID := d.metric("before_mean", "Mean of "+s.metric.Name+" before "+spec.BreakPeriod, s.metric.Unit, "mean of recorded windows before breakPeriod")
	aID := d.metric("after_mean", "Mean of "+s.metric.Name+" from "+spec.BreakPeriod, s.metric.Unit, "mean of recorded windows from breakPeriod on")
	dID := d.metric("difference", "After mean minus before mean of "+s.metric.Name, s.metric.Unit, "after_mean - before_mean")
	bp, bt := before.period(s.basis)
	ap, at := after.period(s.basis)
	d.add(bID, bp, s.dims, before.mean(), bt, "", "", excludedFlag(before.excluded)...)
	d.add(aID, ap, s.dims, after.mean(), at, "", "", excludedFlag(after.excluded)...)
	diff := *after.mean() - *before.mean()
	d.add(dID, Period{Start: bp.Start, End: ap.End, Basis: s.basis}, s.dims, &diff, at, "", "")
	d.limitation("A before/after difference compares two periods; other things may also differ between them.")
	return nil
}

func cohorts(d *derived, src Artifact, spec OperationSpec) error {
	all, err := extractSeries(src, spec.MetricID, spec.Series)
	if err != nil {
		return err
	}
	if len(all) < 2 {
		return fmt.Errorf("%w: cohort_comparison needs at least two series", ErrInvalidOperation)
	}
	id := d.metric("cohort_index", all[0].metric.Name+" indexed to each cohort's first recorded window = 100", "index (cohort start = 100)",
		"value / first recorded value of the same cohort * 100")
	for _, s := range all {
		if _, ok := s.dims[spec.CohortDimension]; !ok {
			return fmt.Errorf("%w: a series has no %q dimension", ErrInvalidOperation, spec.CohortDimension)
		}
		var base *float64
		for _, p := range s.points {
			if p.value != nil && *p.value != 0 {
				base = p.value
				break
			}
		}
		for _, p := range s.points {
			var v *float64
			if base != nil && p.value != nil {
				x := *p.value / *base * 100
				v = &x
			}
			d.add(id, p.period, s.dims, v, p.temporal, "", "MISSING_OR_GAP")
		}
	}
	d.limitation("Cohorts are indexed to their own start; differences in cohort size or composition are not adjusted.")
	return nil
}

func pearson(xs, ys []float64) (float64, bool) {
	n := float64(len(xs))
	var mx, my float64
	for i := range xs {
		mx += xs[i]
		my += ys[i]
	}
	mx, my = mx/n, my/n
	var sxy, sxx, syy float64
	for i := range xs {
		sxy += (xs[i] - mx) * (ys[i] - my)
		sxx += (xs[i] - mx) * (xs[i] - mx)
		syy += (ys[i] - my) * (ys[i] - my)
	}
	if sxx == 0 || syy == 0 {
		return 0, false
	}
	return math.Round(sxy/math.Sqrt(sxx*syy)*1e12) / 1e12, true
}

func laggedComparison(d *derived, src Artifact, a series, spec OperationSpec) error {
	b, err := referenceSeries(src, a, spec)
	if err != nil {
		return err
	}
	if a.basis != b.basis {
		return fmt.Errorf("%w: series use different period bases", ErrInvalidOperation)
	}
	id := d.metric("lag_correlation", "Correlation of "+a.metric.Name+" with "+b.metric.Name+" shifted by lag windows", "correlation coefficient",
		"Pearson coefficient of a[t] and b[t-lag] over windows where both are recorded")
	period := Period{Start: a.points[0].period.Start, End: a.points[len(a.points)-1].period.End, Basis: a.basis}
	var tmpl *TemporalMetadata
	for _, p := range a.points {
		if p.temporal != nil {
			tmpl = p.temporal
		}
	}
	for lag := -spec.MaxLag; lag <= spec.MaxLag; lag++ {
		var xs, ys []float64
		for _, p := range a.points {
			q, ok := b.at(p.index - lag)
			if ok && p.value != nil && q.value != nil {
				xs, ys = append(xs, *p.value), append(ys, *q.value)
			}
		}
		dims := map[string]string{"lag": strconv.Itoa(lag)}
		if len(xs) < 3 {
			d.add(id, period, dims, nil, tmpl, "not_applicable", "INSUFFICIENT_PAIRS")
			continue
		}
		r, ok := pearson(xs, ys)
		if !ok {
			d.add(id, period, dims, nil, tmpl, "not_applicable", "ZERO_VARIANCE")
			continue
		}
		d.add(id, period, dims, &r, tmpl, "not_applicable", "", QualityFlag{Code: "PAIRS", Message: strconv.Itoa(len(xs))})
	}
	d.limitation("Association only: a coefficient at some lag says nothing about why the series move together.")
	return nil
}

func controlComparison(d *derived, src Artifact, treated series, spec OperationSpec) error {
	control, err := referenceSeries(src, treated, spec)
	if err != nil {
		return err
	}
	tb, ta, err := split(treated, spec.BreakPeriod)
	if err != nil {
		return err
	}
	cb, ca, err := split(control, spec.BreakPeriod)
	if err != nil {
		return err
	}
	tChange, cChange := *ta.mean()-*tb.mean(), *ca.mean()-*cb.mean()
	gap := tChange - cChange
	bp, _ := tb.period(treated.basis)
	_, at := ta.period(treated.basis)
	period := Period{Start: bp.Start, End: ta.last.period.End, Basis: treated.basis}
	unit := treated.metric.Unit
	d.add(d.metric("selected_change", "Change of the selected series around "+spec.BreakPeriod, unit, "after mean - before mean"), period, treated.dims, &tChange, at, "", "", excludedFlag(tb.excluded+ta.excluded)...)
	d.add(d.metric("reference_change", "Change of the reference series around "+spec.BreakPeriod, unit, "after mean - before mean"), period, control.dims, &cChange, at, "", "", excludedFlag(cb.excluded+ca.excluded)...)
	d.add(d.metric("difference_of_changes", "Selected change minus reference change", unit, "selected_change - reference_change"), period, treated.dims, &gap, at, "", "")
	d.limitation("The reference series is assumed comparable; that assumption is not verified here.")
	return nil
}

func median(xs []float64) float64 {
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

func anomalies(d *derived, s series, spec OperationSpec) error {
	threshold := spec.Threshold
	if threshold <= 0 {
		threshold = 3.5
	}
	var xs []float64
	for _, p := range s.points {
		if p.value != nil {
			xs = append(xs, *p.value)
		}
	}
	id := d.metric("robust_z", "Robust z-score of "+s.metric.Name, "robust z-score", fmt.Sprintf("0.6745 * (x - median) / MAD; |z| > %g is flagged", threshold))
	code := ""
	var med, mad float64
	if len(xs) < 5 {
		code = "INSUFFICIENT_POINTS"
	} else {
		med = median(xs)
		devs := make([]float64, len(xs))
		for i, x := range xs {
			devs[i] = math.Abs(x - med)
		}
		if mad = median(devs); mad == 0 {
			code = "ZERO_DISPERSION"
		}
	}
	for _, p := range s.points {
		if code != "" || p.value == nil {
			c := code
			if c == "" {
				c = "MISSING_OR_GAP"
			}
			d.add(id, p.period, s.dims, nil, p.temporal, "not_applicable", c)
			continue
		}
		z := math.Round(0.6745*(*p.value-med)/mad*1e9) / 1e9
		var flags []QualityFlag
		if math.Abs(z) > threshold {
			flags = append(flags, QualityFlag{Code: "ANOMALY_CANDIDATE", Message: "statistical outlier candidate; not a confirmed event"})
		}
		d.add(id, p.period, s.dims, &z, p.temporal, "not_applicable", "", flags...)
	}
	return nil
}

func sse(xs []float64) float64 {
	var m float64
	for _, x := range xs {
		m += x
	}
	m /= float64(len(xs))
	var s float64
	for _, x := range xs {
		s += (x - m) * (x - m)
	}
	return s
}

func changePoint(d *derived, s series, spec OperationSpec) error {
	minSeg := spec.MinSegment
	if minSeg == 0 {
		minSeg = 3
	}
	var pts []*point
	missing := 0
	for i := range s.points {
		if s.points[i].value == nil {
			missing++
			continue
		}
		pts = append(pts, &s.points[i])
	}
	if len(pts) < 2*minSeg {
		return fmt.Errorf("%w: change_point_candidate needs at least %d recorded windows, got %d", ErrInvalidOperation, 2*minSeg, len(pts))
	}
	xs := make([]float64, len(pts))
	for i, p := range pts {
		xs[i] = *p.value
	}
	total := sse(xs)
	best, bestSSE := -1, math.Inf(1)
	for k := minSeg; k <= len(xs)-minSeg; k++ {
		if v := sse(xs[:k]) + sse(xs[k:]); v < bestSSE {
			best, bestSSE = k, v
		}
	}
	period := Period{Start: pts[0].period.Start, End: pts[len(pts)-1].period.End, Basis: s.basis}
	last := pts[len(pts)-1].temporal
	dims := map[string]string{"candidatePeriod": pts[best].period.Start}
	for k, v := range s.dims {
		dims[k] = v
	}
	scoreID := d.metric("shift_score", "Mean-shift break candidate score of "+s.metric.Name, "ratio", "1 - SSE(two segments) / SSE(one segment) at the best split")
	var score *float64
	if total > 0 {
		x := math.Round((1-bestSSE/total)*1e9) / 1e9
		score = &x
	}
	d.add(scoreID, period, dims, score, last, "not_applicable", "ZERO_VARIANCE", excludedFlag(missing)...)
	var bm, am float64
	for _, x := range xs[:best] {
		bm += x
	}
	for _, x := range xs[best:] {
		am += x
	}
	bm, am = bm/float64(best), am/float64(len(xs)-best)
	d.add(d.metric("before_mean", "Mean before the break candidate", s.metric.Unit, "mean of windows before candidatePeriod"),
		Period{Start: pts[0].period.Start, End: pts[best-1].period.End, Basis: s.basis}, dims, &bm, pts[best-1].temporal, "", "")
	d.add(d.metric("after_mean", "Mean from the break candidate", s.metric.Unit, "mean of windows from candidatePeriod"),
		Period{Start: pts[best].period.Start, End: pts[len(pts)-1].period.End, Basis: s.basis}, dims, &am, last, "", "")
	d.limitation("Single mean-shift search; a break candidate is not a confirmed structural change.")
	return nil
}
