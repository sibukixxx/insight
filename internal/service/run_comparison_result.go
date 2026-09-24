package service

import (
	"encoding/json"
	"math"
	"sort"

	"insight-lab/internal/domain"
)

// MetricDelta is one numeric top-level metric. A nil side means the run did
// not record that metric; Delta is nil unless both sides exist.
type MetricDelta struct {
	Metric string   `json:"metric"`
	From   *float64 `json:"from"`
	To     *float64 `json:"to"`
	Delta  *float64 `json:"delta"`
}

// InsightMatch pairs an insight of the earlier run with one of the later run.
// Method and Score make the pairing auditable; raw IDs are never the basis.
type InsightMatch struct {
	FromInsightID string        `json:"fromInsightId"`
	ToInsightID   string        `json:"toInsightId"`
	Method        string        `json:"method"`
	Score         float64       `json:"score"`
	Changes       []FieldChange `json:"changes"`
}

type InsightResultDiff struct {
	Added   []string       `json:"added"`
	Removed []string       `json:"removed"`
	Matched []InsightMatch `json:"matched"`
}

// MetricRange is the observed spread of one metric within a repeat group.
type MetricRange struct {
	Metric string  `json:"metric"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}

// RepeatGroup collects runs with identical (input, execution) fingerprints.
type RepeatGroup struct {
	Key            string        `json:"key"`
	AnalysisIDs    []string      `json:"analysisIds"`
	Runs           int           `json:"runs"`
	VariationKnown bool          `json:"variationKnown"`
	Metrics        []MetricRange `json:"metrics"`
}

// Matching methods, strongest first.
const (
	MatchExactSpans    = "EXACT_EVIDENCE_SPANS"
	MatchSpanOverlap   = "EVIDENCE_SPAN_OVERLAP"
	MatchHypothesisKey = "HYPOTHESIS_COMPARISON_KEY"
	// spanOverlapThreshold is the minimum overlap ratio for a span match.
	spanOverlapThreshold = 0.3
)

func numericMetrics(raw string) map[string]float64 {
	out := map[string]float64{}
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil {
		return out
	}
	for k, v := range m {
		if f, ok := v.(float64); ok {
			out[k] = f
		}
	}
	return out
}

func compareMetrics(fromRaw, toRaw string) []MetricDelta {
	a, b := numericMetrics(fromRaw), numericMetrics(toRaw)
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	names := make([]string, 0, len(keys))
	for k := range keys {
		names = append(names, k)
	}
	sort.Strings(names)
	out := []MetricDelta{}
	for _, k := range names {
		d := MetricDelta{Metric: k}
		if v, ok := a[k]; ok {
			d.From = &v
		}
		if v, ok := b[k]; ok {
			d.To = &v
		}
		if d.From != nil && d.To != nil {
			delta := *d.To - *d.From
			d.Delta = &delta
		}
		out = append(out, d)
	}
	return out
}

type span struct {
	doc        string
	start, end int
}

func insightSpans(in *domain.Insight) []span {
	out := []span{}
	for _, e := range in.Evidence {
		if e.EndOffset > e.StartOffset {
			out = append(out, span{e.DocumentID, e.StartOffset, e.EndOffset})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].doc != out[j].doc {
			return out[i].doc < out[j].doc
		}
		return out[i].start < out[j].start
	})
	return out
}

// spanOverlap is |intersection| / |union| of evidence characters, per document.
func spanOverlap(a, b []span) float64 {
	cover := func(list []span) map[string]map[int]bool {
		m := map[string]map[int]bool{}
		for _, s := range list {
			if m[s.doc] == nil {
				m[s.doc] = map[int]bool{}
			}
			for i := s.start; i < s.end; i++ {
				m[s.doc][i] = true
			}
		}
		return m
	}
	ca, cb := cover(a), cover(b)
	inter, union := 0, 0
	for doc, chars := range ca {
		for i := range chars {
			union++
			if cb[doc][i] {
				inter++
			}
		}
	}
	for doc, chars := range cb {
		for i := range chars {
			if !ca[doc][i] {
				union++
			}
		}
	}
	if union == 0 {
		return 0
	}
	return math.Round(float64(inter)/float64(union)*1000) / 1000
}

func equalSpans(a, b []span) bool {
	if len(a) == 0 || len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func scoreInsightPair(a, b *domain.Insight) (string, float64) {
	sa, sb := insightSpans(a), insightSpans(b)
	if equalSpans(sa, sb) {
		return MatchExactSpans, 1
	}
	if s := spanOverlap(sa, sb); s >= spanOverlapThreshold {
		return MatchSpanOverlap, s
	}
	if k := hypothesisComparisonKey(a); k != "" && k == hypothesisComparisonKey(b) {
		// The comparison key is the normalized hypothesis title (fallback only).
		return MatchHypothesisKey, 0.5
	}
	return "", 0
}

var methodRank = map[string]int{MatchExactSpans: 4, MatchSpanOverlap: 3, MatchHypothesisKey: 2}

// matchInsights pairs insights greedily by method strength, then score, then
// stable ID order, so identical inputs always yield identical pairings.
func matchInsights(from, to RunComparisonInput) InsightResultDiff {
	type cand struct {
		a, b   *domain.Insight
		method string
		score  float64
	}
	var cands []cand
	for _, a := range from.Insights {
		for _, b := range to.Insights {
			if m, s := scoreInsightPair(a, b); m != "" {
				cands = append(cands, cand{a, b, m, s})
			}
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		ci, cj := cands[i], cands[j]
		if methodRank[ci.method] != methodRank[cj.method] {
			return methodRank[ci.method] > methodRank[cj.method]
		}
		if ci.score != cj.score {
			return ci.score > cj.score
		}
		if ci.a.ID != cj.a.ID {
			return ci.a.ID < cj.a.ID
		}
		return ci.b.ID < cj.b.ID
	})
	usedA, usedB := map[string]bool{}, map[string]bool{}
	out := InsightResultDiff{Added: []string{}, Removed: []string{}, Matched: []InsightMatch{}}
	for _, c := range cands {
		if usedA[c.a.ID] || usedB[c.b.ID] {
			continue
		}
		usedA[c.a.ID], usedB[c.b.ID] = true, true
		out.Matched = append(out.Matched, InsightMatch{FromInsightID: c.a.ID, ToInsightID: c.b.ID, Method: c.method, Score: c.score, Changes: insightChanges(c.a, c.b)})
	}
	for _, a := range from.Insights {
		if !usedA[a.ID] {
			out.Removed = append(out.Removed, a.ID)
		}
	}
	for _, b := range to.Insights {
		if !usedB[b.ID] {
			out.Added = append(out.Added, b.ID)
		}
	}
	sort.Strings(out.Added)
	sort.Strings(out.Removed)
	return out
}

func insightChanges(a, b *domain.Insight) []FieldChange {
	fields := func(in *domain.Insight) map[string]string {
		return map[string]string{
			"title": in.Title, "validationStatus": string(in.ValidationStatus),
			"identificationStatus": string(in.IdentificationStatus), "causalStatus": string(in.CausalStatus),
			"hypothesisRole": string(in.HypothesisRole),
		}
	}
	return diffFields(fields(a), fields(b))
}

func repeatKey(a *domain.Analysis) string {
	if a.InputFingerprint == "" || a.ExecutionFingerprint == "" {
		return ""
	}
	return a.InputFingerprint + "|" + a.ExecutionFingerprint
}

// repeatGroupsFor returns the repeat groups of the compared runs, built from
// the project history. Runs without both fingerprints form no group.
func repeatGroupsFor(history []*domain.Analysis, runs ...*domain.Analysis) []RepeatGroup {
	out := []RepeatGroup{}
	seen := map[string]bool{}
	for _, r := range runs {
		key := repeatKey(r)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		g := RepeatGroup{Key: key, AnalysisIDs: []string{}, Metrics: []MetricRange{}}
		ranges := map[string]*MetricRange{}
		for _, h := range history {
			if repeatKey(h) != key || h.Status != domain.AnalysisCompleted {
				continue
			}
			g.AnalysisIDs = append(g.AnalysisIDs, h.ID)
			for name, v := range numericMetrics(h.Metrics) {
				if rg, ok := ranges[name]; ok {
					rg.Min, rg.Max = math.Min(rg.Min, v), math.Max(rg.Max, v)
				} else {
					ranges[name] = &MetricRange{Metric: name, Min: v, Max: v}
				}
			}
		}
		sort.Strings(g.AnalysisIDs)
		g.Runs = len(g.AnalysisIDs)
		g.VariationKnown = g.Runs >= 2
		for _, rg := range ranges {
			g.Metrics = append(g.Metrics, *rg)
		}
		sort.Slice(g.Metrics, func(i, j int) bool { return g.Metrics[i].Metric < g.Metrics[j].Metric })
		out = append(out, g)
	}
	return out
}
