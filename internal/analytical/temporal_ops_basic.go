package analytical

import (
	"fmt"
	"math"
)

func computeOperation(src Artifact, spec OperationSpec) (*derived, error) {
	base, err := singleOrCohort(src, spec)
	if err != nil {
		return nil, err
	}
	d := &derived{op: spec.Operation, srcVersion: base.metric.Version}
	switch spec.Operation {
	case OpYoY, OpMoM, OpQoQ, OpPeriodOverPeriod:
		return d, periodOverPeriod(d, base, spec)
	case OpCAGR:
		return d, cagr(d, base)
	case OpRollingMean, OpRollingDelta:
		return d, rolling(d, base, spec)
	case OpIndexedBaseline:
		return d, indexed(d, base, spec)
	case OpShare, OpUnitPrice:
		return d, ratioOf(d, src, base, spec)
	case OpBeforeAfter:
		return d, beforeAfter(d, base, spec)
	case OpCohortComparison:
		return d, cohorts(d, src, spec)
	case OpLaggedComparison:
		return d, laggedComparison(d, src, base, spec)
	case OpControlComparison:
		return d, controlComparison(d, src, base, spec)
	case OpAnomalyCandidate:
		return d, anomalies(d, base, spec)
	case OpChangePointCandidate:
		return d, changePoint(d, base, spec)
	}
	return nil, fmt.Errorf("%w: unknown operation %q", ErrInvalidOperation, spec.Operation)
}

func singleOrCohort(src Artifact, spec OperationSpec) (series, error) {
	if spec.Operation == OpCohortComparison {
		all, err := extractSeries(src, spec.MetricID, spec.Series)
		if err != nil {
			return series{}, err
		}
		return all[0], nil
	}
	return singleSeries(src, spec.MetricID, spec.Series)
}

func referenceSeries(src Artifact, base series, spec OperationSpec) (series, error) {
	metric, filter := spec.ReferenceMetricID, spec.ReferenceSeries
	if metric == "" {
		metric = spec.MetricID
	}
	if filter == nil {
		filter = spec.Series
	}
	return singleSeries(src, metric, filter)
}

func lagFor(spec OperationSpec, basis string) (int, error) {
	switch {
	case spec.Operation == OpPeriodOverPeriod:
		return spec.Lag, nil
	case spec.Operation == OpYoY && basis == "calendar-year":
		return 1, nil
	case spec.Operation == OpYoY && basis == "calendar-month":
		return 12, nil
	case spec.Operation == OpMoM && basis == "calendar-month":
		return 1, nil
	case spec.Operation == OpQoQ && basis == "calendar-month":
		return 3, nil
	}
	return 0, fmt.Errorf("%w: %s is not defined for %s periods; use period_over_period with an explicit lag", ErrInvalidOperation, spec.Operation, basis)
}

func periodOverPeriod(d *derived, s series, spec OperationSpec) error {
	lag, err := lagFor(spec, s.basis)
	if err != nil {
		return err
	}
	id := d.metric("change_ratio", fmt.Sprintf("%s change ratio of %s (lag %d)", spec.Operation, s.metric.Name, lag), "ratio",
		"(current - earlier) / abs(earlier); a ratio, not a percentage")
	for _, cur := range s.points {
		prev, ok := s.at(cur.index - lag)
		if !ok {
			continue
		}
		var v *float64
		code := ""
		switch {
		case cur.value == nil || prev.value == nil:
			code = "MISSING_OR_GAP"
		case *prev.value == 0:
			code = "ZERO_BASELINE"
		default:
			x := (*cur.value - *prev.value) / math.Abs(*prev.value)
			v = &x
		}
		d.add(id, cur.period, s.dims, v, cur.temporal, "", code)
	}
	return nil
}

func periodsPerYear(basis string) (float64, error) {
	switch basis {
	case "calendar-year":
		return 1, nil
	case "calendar-month":
		return 12, nil
	}
	return 0, fmt.Errorf("%w: cagr needs calendar-year or calendar-month periods", ErrInvalidOperation)
}

func cagr(d *derived, s series) error {
	per, err := periodsPerYear(s.basis)
	if err != nil {
		return err
	}
	var first, last *point
	gaps := 0
	for i := range s.points {
		p := &s.points[i]
		if p.value == nil {
			gaps++
			continue
		}
		if first == nil {
			first = p
		}
		last = p
	}
	id := d.metric("cagr", "Compound annual growth rate of "+s.metric.Name, "ratio per year", "(last / first)^(1 / years) - 1")
	if first == nil || first == last {
		return fmt.Errorf("%w: cagr needs at least two recorded values", ErrInvalidOperation)
	}
	period := Period{Start: first.period.Start, End: last.period.End, Basis: s.basis}
	var flags []QualityFlag
	if gaps > 0 {
		flags = append(flags, QualityFlag{Code: "INTERIOR_GAPS", Message: fmt.Sprintf("%d missing windows inside the span were not used", gaps)})
	}
	if *first.value <= 0 || *last.value <= 0 {
		d.add(id, period, s.dims, nil, last.temporal, "", "NON_POSITIVE_ENDPOINT", flags...)
		return nil
	}
	years := float64(last.index-first.index) / per
	x := math.Pow(*last.value / *first.value, 1/years) - 1
	d.add(id, period, s.dims, &x, last.temporal, "", "", flags...)
	return nil
}

func rolling(d *derived, s series, spec OperationSpec) error {
	w := spec.Window
	var id string
	if spec.Operation == OpRollingMean {
		id = d.metric("rolling_mean", fmt.Sprintf("%d-window rolling mean of %s", w, s.metric.Name), s.metric.Unit, "mean of the last window values")
	} else {
		id = d.metric("rolling_delta", fmt.Sprintf("%d-window delta of %s", w, s.metric.Name), s.metric.Unit, "current minus the value window steps earlier")
	}
	for i, cur := range s.points {
		if i < w-1 && spec.Operation == OpRollingMean || i < w && spec.Operation == OpRollingDelta {
			continue
		}
		var v *float64
		if spec.Operation == OpRollingMean {
			sum, ok := 0.0, true
			for _, p := range s.points[i-w+1 : i+1] {
				if p.value == nil {
					ok = false
					break
				}
				sum += *p.value
			}
			if ok {
				x := sum / float64(w)
				v = &x
			}
		} else if prev := s.points[i-w]; cur.value != nil && prev.value != nil {
			x := *cur.value - *prev.value
			v = &x
		}
		d.add(id, cur.period, s.dims, v, cur.temporal, "", "INCOMPLETE_WINDOW")
	}
	return nil
}

func indexed(d *derived, s series, spec OperationSpec) error {
	idx, err := calendarIndex(Period{Start: spec.BaselinePeriod, End: spec.BaselinePeriod, Basis: s.basis})
	if err != nil {
		return err
	}
	base, ok := s.at(idx)
	if !ok || base.value == nil || *base.value == 0 {
		return fmt.Errorf("%w: baseline period %s has no non-zero recorded value", ErrInvalidOperation, spec.BaselinePeriod)
	}
	id := d.metric("index", s.metric.Name+" indexed to "+spec.BaselinePeriod+" = 100", "index (baseline = 100)", "value / baseline value * 100")
	for _, p := range s.points {
		var v *float64
		if p.value != nil {
			x := *p.value / *base.value * 100
			v = &x
		}
		d.add(id, p.period, s.dims, v, p.temporal, "", "MISSING_OR_GAP")
	}
	return nil
}

func ratioOf(d *derived, src Artifact, num series, spec OperationSpec) error {
	den, err := referenceSeries(src, num, spec)
	if err != nil {
		return err
	}
	if den.basis != num.basis {
		return fmt.Errorf("%w: numerator and denominator use different period bases", ErrInvalidOperation)
	}
	unit, basis, name := "ratio", "not_applicable", "Share of "+num.metric.Name+" in "+den.metric.Name
	if spec.Operation == OpUnitPrice {
		unit, basis, name = num.metric.Unit+"/"+den.metric.Unit, "", "Unit value of "+num.metric.Name+" per "+den.metric.Name
	}
	id := d.metric(spec.Operation, name, unit, "numerator / denominator for the same window")
	for _, p := range num.points {
		q, ok := den.at(p.index)
		var v *float64
		code := "MISSING_OR_GAP"
		if ok && p.value != nil && q.value != nil {
			if *q.value == 0 {
				code = "ZERO_DENOMINATOR"
			} else {
				x := *p.value / *q.value
				v = &x
			}
		}
		d.add(id, p.period, num.dims, v, p.temporal, basis, code)
	}
	return nil
}
