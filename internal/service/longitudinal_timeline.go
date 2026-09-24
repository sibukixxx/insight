package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"insight-lab/internal/analytical"
	"insight-lab/internal/domain"
)

// TimelineSources are the recorded analyses and their observations for the
// iterations of one run, keyed by analysis ID.
type TimelineSources struct {
	Analyses     map[string]*domain.Analysis
	Observations map[string][]*domain.Observation
}

// BuildLongitudinalTimeline derives the #71 read model. It is pure: the same
// stored iterations and snapshots always yield the same timeline, and adding
// a later iteration never changes the entries of earlier ones.
func BuildLongitudinalTimeline(run domain.ResearchRun, src TimelineSources) domain.LongitudinalTimeline {
	its := append([]domain.ResearchIteration(nil), run.Iterations...)
	sort.SliceStable(its, func(i, j int) bool { return its[i].Sequence < its[j].Sequence })
	t := domain.LongitudinalTimeline{
		ResearchRunID: run.ID, Question: run.Question,
		Iterations: []domain.TimelineIteration{}, EvidenceEvents: []domain.EvidenceEvent{},
		ObservationDeltas: []domain.TimelineObservationDelta{}, HypothesisEvents: []domain.HypothesisEvent{},
		InsightVersions: []domain.InsightVersion{}, InstrumentChanges: []domain.InstrumentChange{},
		Limitations: []string{
			"A timeline records how the research interpretation changed; it does not establish why the world changed.",
			"Instrument changes (engine, model, prompt, rules) are shown separately and must not be read as evidence.",
		},
	}
	unwindowed := false
	for i, it := range its {
		var prev *domain.ResearchIteration
		if i > 0 {
			prev = &its[i-1]
		}
		if it.ObservationWindow == nil {
			unwindowed = true
		} else {
			asOf := it.ObservationWindow.AsOf
			t.AsOf = &asOf
		}
		t.Iterations = append(t.Iterations, timelineIteration(it, prev))

		events := evidenceEvents(it, prev, src)
		deltas, defEvents := observationDeltas(it, prev, src)
		events = append(events, defEvents...)
		t.EvidenceEvents = append(t.EvidenceEvents, events...)
		t.ObservationDeltas = append(t.ObservationDeltas, deltas...)

		instrument := instrumentChange(it, prev, src)
		if instrument != nil {
			t.InstrumentChanges = append(t.InstrumentChanges, *instrument)
		}
		attribution := attribute(i == 0 || len(events) > 0, instrument != nil && instrument.Execution == domain.FingerprintChanged)
		refs := eventRefs(events)
		for _, hc := range it.HypothesisChanges {
			t.HypothesisEvents = append(t.HypothesisEvents, domain.HypothesisEvent{
				IterationID: it.ID, Sequence: it.Sequence, HypothesisID: hc.HypothesisID, Evolution: hc.Evolution,
				Reason: hc.Reason, EvidenceRefs: refs, Attribution: attribution,
				InvalidatesPriorInterpretation: hc.Evolution == domain.HypothesisWeakened || hc.Evolution == domain.HypothesisContradicted,
			})
		}
		version := domain.InsightVersion{IterationID: it.ID, Sequence: it.Sequence, InsightIDs: it.InsightIDs, Attribution: attribution}
		if it.Delta != nil {
			version.Explanation = it.Delta.Explanation
		}
		t.InsightVersions = append(t.InsightVersions, version)
	}
	if unwindowed {
		t.Limitations = append(t.Limitations, "Some iterations have no observation window; their as-of boundary is unknown.")
	}
	return t
}

func timelineIteration(it domain.ResearchIteration, prev *domain.ResearchIteration) domain.TimelineIteration {
	out := domain.TimelineIteration{
		IterationID: it.ID, Sequence: it.Sequence, PreviousIterationID: it.PreviousIterationID, AnalysisID: it.AnalysisID,
		ObservationWindow: it.ObservationWindow, Question: it.Question, Readiness: it.EffectiveReadiness(),
		Stopped: it.Stop != nil, UnresolvedGapIDs: it.UnresolvedGapIDs(), WhatWeCannotConclude: it.WhatWeCannotConclude,
		RecordedAt: it.CreatedAt,
	}
	if prev != nil {
		if out.PreviousIterationID == "" {
			out.PreviousIterationID = prev.ID
		}
		current := setOf(it.WhatWeCannotConclude)
		for _, s := range prev.WhatWeCannotConclude {
			if current[s] {
				out.CarriedUncertainty = append(out.CarriedUncertainty, s)
			} else {
				out.ResolvedUncertainty = append(out.ResolvedUncertainty, s)
			}
		}
	}
	return out
}

func attribute(evidence, instrument bool) domain.ChangeAttribution {
	switch {
	case evidence && instrument:
		return domain.AttributedToBoth
	case evidence:
		return domain.AttributedToEvidence
	case instrument:
		return domain.AttributedToInstrument
	}
	return domain.Unattributed
}

func eventRefs(events []domain.EvidenceEvent) []string {
	var out []string
	seen := map[string]bool{}
	for _, e := range events {
		if e.Kind != domain.EvidenceRemoved && !seen[e.Reference] {
			seen[e.Reference] = true
			out = append(out, e.Reference)
		}
	}
	return out
}

func setOf(list []string) map[string]bool {
	out := make(map[string]bool, len(list))
	for _, s := range list {
		out[s] = true
	}
	return out
}

func iterationRefs(it domain.ResearchIteration) []string {
	var out []string
	seen := map[string]bool{}
	add := func(list ...string) {
		for _, r := range list {
			if r = strings.TrimSpace(r); r != "" && !seen[r] {
				seen[r] = true
				out = append(out, r)
			}
		}
	}
	add(it.InputSnapshot.ArtifactReferences...)
	add(it.InputSnapshot.EvidenceReferences...)
	add(it.AddedEvidence...)
	for _, l := range it.AddedEvidenceLinks {
		add(l.Reference)
	}
	return out
}

func gapIDsFor(it domain.ResearchIteration, ref string) []string {
	var out []string
	for _, l := range it.AddedEvidenceLinks {
		if l.Reference == ref {
			out = append(out, l.GapIDs...)
		}
	}
	return out
}

func evidenceEvents(it domain.ResearchIteration, prev *domain.ResearchIteration, src TimelineSources) []domain.EvidenceEvent {
	current := iterationRefs(it)
	var events []domain.EvidenceEvent
	add := func(kind domain.EvidenceChangeKind, ref, detail string) {
		events = append(events, domain.EvidenceEvent{IterationID: it.ID, Sequence: it.Sequence, Kind: kind, Reference: ref, GapIDs: gapIDsFor(it, ref), Detail: detail})
	}
	if prev == nil {
		for _, r := range current {
			add(domain.EvidenceAdded, r, "")
		}
		return events
	}
	before := setOf(iterationRefs(*prev))
	after := setOf(current)
	for _, r := range current {
		if !before[r] {
			add(domain.EvidenceAdded, r, "")
		}
	}
	for _, r := range iterationRefs(*prev) {
		if !after[r] {
			add(domain.EvidenceRemoved, r, "")
		}
	}
	if it.AnalysisID != "" && prev.AnalysisID != "" && it.AnalysisID != prev.AnalysisID {
		pd, cd := inputDocuments(src.Analyses[prev.AnalysisID]), inputDocuments(src.Analyses[it.AnalysisID])
		ids := make([]string, 0, len(cd))
		for id := range cd {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			p, ok := pd[id]
			if !ok {
				continue
			}
			c := cd[id]
			if p.ContentHash != c.ContentHash {
				add(domain.EvidenceContentChanged, id, "document content hash changed between analyses")
			} else if p.MetadataHash != c.MetadataHash {
				add(domain.EvidenceDefinitionChanged, id, "document metadata/definition hash changed between analyses")
			}
		}
	}
	return events
}

func inputDocuments(a *domain.Analysis) map[string]InputDocument {
	out := map[string]InputDocument{}
	if a == nil || a.InputSnapshot == "" {
		return out
	}
	var snap InputSnapshot
	if json.Unmarshal([]byte(a.InputSnapshot), &snap) != nil {
		return out
	}
	for _, d := range snap.Documents {
		out[d.ID] = d
	}
	return out
}

var instrumentFields = []string{"engineVersion", "gitCommit", "gitDirty", "executionMode", "ruleVersions", "promptVersion", "promptFingerprint", "llm"}

func instrumentChange(it domain.ResearchIteration, prev *domain.ResearchIteration, src TimelineSources) *domain.InstrumentChange {
	if prev == nil || it.AnalysisID == prev.AnalysisID {
		return nil
	}
	pa, ca := src.Analyses[prev.AnalysisID], src.Analyses[it.AnalysisID]
	var pf, cf string
	if pa != nil {
		pf = pa.ExecutionFingerprint
	}
	if ca != nil {
		cf = ca.ExecutionFingerprint
	}
	state := domain.CompareFingerprints(pf, cf)
	if state == domain.FingerprintSame {
		return nil
	}
	change := &domain.InstrumentChange{FromIterationID: prev.ID, ToIterationID: it.ID, Execution: state}
	if pa != nil && ca != nil {
		var ps, cs map[string]json.RawMessage
		if json.Unmarshal([]byte(pa.ExecutionSnapshot), &ps) == nil && json.Unmarshal([]byte(ca.ExecutionSnapshot), &cs) == nil {
			for _, f := range instrumentFields {
				if string(ps[f]) != string(cs[f]) {
					change.ChangedFields = append(change.ChangedFields, f)
				}
			}
		}
	}
	return change
}

// seriesKey identifies "the same quantity" across re-observations while
// deliberately ignoring metric version, so a definition change is detected
// by the delta instead of silently splitting the series.
func seriesKey(o *domain.Observation) string {
	te := o.Temporal
	dims := make([]string, 0, len(te.Result.Dimensions))
	for k, v := range te.Result.Dimensions {
		dims = append(dims, k+"="+v)
	}
	sort.Strings(dims)
	geo := ""
	if te.Result.Temporal != nil {
		geo = te.Result.Temporal.Geography
	}
	return strings.Join([]string{te.Metric.ID, geo, strings.Join(dims, ",")}, "|")
}

func latestBySeries(list []*domain.Observation) map[string]*domain.Observation {
	out := map[string]*domain.Observation{}
	for _, o := range list {
		if o == nil || o.Temporal == nil {
			continue
		}
		k := seriesKey(o)
		if cur, ok := out[k]; !ok || o.Temporal.Result.Period.End > cur.Temporal.Result.Period.End ||
			(o.Temporal.Result.Period.End == cur.Temporal.Result.Period.End && o.ID > cur.ID) {
			out[k] = o
		}
	}
	return out
}

func observationDeltas(it domain.ResearchIteration, prev *domain.ResearchIteration, src TimelineSources) ([]domain.TimelineObservationDelta, []domain.EvidenceEvent) {
	if prev == nil || it.AnalysisID == "" || prev.AnalysisID == "" || it.AnalysisID == prev.AnalysisID {
		return nil, nil
	}
	before, after := latestBySeries(src.Observations[prev.AnalysisID]), latestBySeries(src.Observations[it.AnalysisID])
	keys := make([]string, 0, len(after))
	for k := range after {
		if _, ok := before[k]; ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var deltas []domain.TimelineObservationDelta
	var events []domain.EvidenceEvent
	for _, k := range keys {
		p, c := before[k], after[k]
		if p.Temporal.Result.Period == c.Temporal.Result.Period {
			if string(p.Temporal.Result.Value) != string(c.Temporal.Result.Value) || p.Temporal.Result.Missing != c.Temporal.Result.Missing {
				events = append(events, domain.EvidenceEvent{IterationID: it.ID, Sequence: it.Sequence, Kind: domain.EvidenceContentChanged, Reference: k,
					Detail: fmt.Sprintf("period %s..%s was restated", c.Temporal.Result.Period.Start, c.Temporal.Result.Period.End)})
			}
			continue
		}
		delta := analytical.CompareObservations(*p, *c)
		if len(delta.DataDefinitionChanges) > 0 {
			events = append(events, domain.EvidenceEvent{IterationID: it.ID, Sequence: it.Sequence, Kind: domain.EvidenceDefinitionChanged, Reference: k,
				Detail: "changed: " + strings.Join(delta.DataDefinitionChanges, ", ")})
		}
		raw, err := json.Marshal(delta)
		if err != nil {
			continue
		}
		deltas = append(deltas, domain.TimelineObservationDelta{FromIterationID: prev.ID, ToIterationID: it.ID, SeriesKey: k, Delta: raw})
	}
	return deltas, events
}
