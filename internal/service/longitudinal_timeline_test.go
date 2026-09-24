package service

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"insight-lab/internal/analytical"
	"insight-lab/internal/domain"
)

// japanEUObservations returns the first n yearly Japan→EU observations bound
// to analysisID, mimicking a re-observation that has seen n periods so far.
func japanEUObservations(t *testing.T, analysisID string, n int) []*domain.Observation {
	t.Helper()
	b, err := os.ReadFile("../../contracts/analytical-artifact/v1/fixtures/temporal-japan-eu.json")
	if err != nil {
		t.Fatal(err)
	}
	a, err := analytical.Import(b)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := analytical.ToCandidatesForAnalysis(*a, analysisID)
	if err != nil {
		t.Fatal(err)
	}
	out := []*domain.Observation{}
	for i := 0; i < n; i++ {
		o := cs[i].Observation
		out = append(out, &o)
	}
	return out
}

func window(year int) *domain.ObservationWindow {
	return &domain.ObservationWindow{AsOf: time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC), Start: "2022", End: time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC).Format("2006"), Basis: "calendar-year"}
}

func analysis(id, execFP, engineVersion string) *domain.Analysis {
	snap, _ := json.Marshal(map[string]any{"engineVersion": engineVersion, "executionFingerprint": execFP})
	return &domain.Analysis{ID: id, ExecutionFingerprint: execFP, ExecutionSnapshot: string(snap)}
}

func threePeriodRun() (domain.ResearchRun, TimelineSources, func(*testing.T) TimelineSources) {
	run := domain.ResearchRun{ID: "run-1", Question: "Is Japan→EU export value growing?", Iterations: []domain.ResearchIteration{
		{ID: "it-1", Sequence: 1, AnalysisID: "a1", ObservationWindow: window(2022), InputSnapshot: domain.InputSetSnapshot{ArtifactReferences: []string{"doc-2022"}},
			WhatWeCannotConclude: []string{"trend direction", "exchange-rate effect"}, InsightIDs: []string{"ins-1"}},
		{ID: "it-2", Sequence: 2, PreviousIterationID: "it-1", AnalysisID: "a2", ObservationWindow: window(2023),
			InputSnapshot:        domain.InputSetSnapshot{ArtifactReferences: []string{"doc-2022", "doc-2023"}},
			AddedEvidenceLinks:   []domain.AddedEvidenceLink{{Reference: "doc-2023", GapIDs: []string{"gap-trend"}}},
			HypothesisChanges:    []domain.HypothesisChange{{HypothesisID: "h-growth", Evolution: domain.HypothesisStrengthened, Reason: "second year higher"}},
			WhatWeCannotConclude: []string{"exchange-rate effect"}, InsightIDs: []string{"ins-2"},
			Delta: &domain.InsightDelta{Explanation: []string{"added 2023 observation"}}},
		{ID: "it-3", Sequence: 3, PreviousIterationID: "it-2", AnalysisID: "a3", ObservationWindow: window(2024),
			InputSnapshot:        domain.InputSetSnapshot{ArtifactReferences: []string{"doc-2023", "doc-2024"}},
			HypothesisChanges:    []domain.HypothesisChange{{HypothesisID: "h-growth", Evolution: domain.HypothesisWeakened, Reason: "growth slowed"}},
			WhatWeCannotConclude: []string{"exchange-rate effect"}},
	}}
	sources := func(t *testing.T) TimelineSources {
		return TimelineSources{
			Analyses: map[string]*domain.Analysis{"a1": analysis("a1", "fp-1", "v1"), "a2": analysis("a2", "fp-1", "v1"), "a3": analysis("a3", "fp-2", "v2")},
			Observations: map[string][]*domain.Observation{
				"a1": japanEUObservations(t, "a1", 1), "a2": japanEUObservations(t, "a2", 2), "a3": japanEUObservations(t, "a3", 3),
			},
		}
	}
	return run, TimelineSources{}, sources
}

func TestBuildLongitudinalTimelineTracksThreePeriodsWithSeparateLanes(t *testing.T) {
	run, _, sources := threePeriodRun()
	tl := BuildLongitudinalTimeline(run, sources(t))

	if len(tl.Iterations) != 3 || tl.AsOf == nil || tl.AsOf.Year() != 2024 {
		t.Fatalf("iterations=%d asOf=%v", len(tl.Iterations), tl.AsOf)
	}
	if len(tl.ObservationDeltas) != 2 {
		t.Fatalf("want 2 observation deltas (2022→2023, 2023→2024), got %d", len(tl.ObservationDeltas))
	}
	kinds := map[domain.EvidenceChangeKind][]string{}
	for _, e := range tl.EvidenceEvents {
		kinds[e.Kind] = append(kinds[e.Kind], e.Reference)
	}
	if len(kinds[domain.EvidenceRemoved]) != 1 || kinds[domain.EvidenceRemoved][0] != "doc-2022" {
		t.Fatalf("removed evidence = %v", kinds[domain.EvidenceRemoved])
	}
	if len(tl.InstrumentChanges) != 1 || tl.InstrumentChanges[0].ToIterationID != "it-3" ||
		tl.InstrumentChanges[0].Execution != domain.FingerprintChanged || tl.InstrumentChanges[0].ChangedFields[0] != "engineVersion" {
		t.Fatalf("instrument lane = %+v", tl.InstrumentChanges)
	}
	for _, e := range tl.EvidenceEvents {
		if e.Reference == "engineVersion" {
			t.Fatal("instrument change leaked into the evidence lane")
		}
	}
}

func TestBuildLongitudinalTimelineTiesHypothesisChangesToEvidenceAndInstrument(t *testing.T) {
	run, _, sources := threePeriodRun()
	tl := BuildLongitudinalTimeline(run, sources(t))
	if len(tl.HypothesisEvents) != 2 {
		t.Fatalf("hypothesis events = %d", len(tl.HypothesisEvents))
	}
	strengthened, weakened := tl.HypothesisEvents[0], tl.HypothesisEvents[1]
	if strengthened.Attribution != domain.AttributedToEvidence || len(strengthened.EvidenceRefs) != 1 || strengthened.EvidenceRefs[0] != "doc-2023" {
		t.Fatalf("strengthened = %+v", strengthened)
	}
	if weakened.Attribution != domain.AttributedToBoth || !weakened.InvalidatesPriorInterpretation {
		t.Fatalf("weakened = %+v", weakened)
	}
	linked := false
	for _, e := range tl.EvidenceEvents {
		if e.Reference == "doc-2023" && e.Kind == domain.EvidenceAdded && len(e.GapIDs) == 1 && e.GapIDs[0] == "gap-trend" {
			linked = true
		}
	}
	if !linked {
		t.Fatalf("added evidence lost its gap linkage: %+v", tl.EvidenceEvents)
	}
}

func TestBuildLongitudinalTimelineCarriesUnresolvedUncertainty(t *testing.T) {
	run, _, sources := threePeriodRun()
	tl := BuildLongitudinalTimeline(run, sources(t))
	it2 := tl.Iterations[1]
	if len(it2.CarriedUncertainty) != 1 || it2.CarriedUncertainty[0] != "exchange-rate effect" ||
		len(it2.ResolvedUncertainty) != 1 || it2.ResolvedUncertainty[0] != "trend direction" {
		t.Fatalf("iteration 2 uncertainty = %+v", it2)
	}
	if tl.Iterations[2].CarriedUncertainty[0] != "exchange-rate effect" {
		t.Fatal("uncertainty did not survive to iteration 3")
	}
}

func TestBuildLongitudinalTimelineNeverChangesEarlierEntriesWhenIterationIsAppended(t *testing.T) {
	run, _, sources := threePeriodRun()
	src := sources(t)
	short := run
	short.Iterations = run.Iterations[:2]
	before := BuildLongitudinalTimeline(short, src)
	after := BuildLongitudinalTimeline(run, src)
	for i := range before.Iterations {
		b, _ := json.Marshal(before.Iterations[i])
		a, _ := json.Marshal(after.Iterations[i])
		if string(a) != string(b) {
			t.Fatalf("iteration %d changed after append:\n%s\n%s", i+1, b, a)
		}
	}
	b, _ := json.Marshal(before.ObservationDeltas)
	a, _ := json.Marshal(after.ObservationDeltas[:len(before.ObservationDeltas)])
	if string(a) != string(b) {
		t.Fatal("historical observation deltas changed after append")
	}
}

func TestBuildLongitudinalTimelineReportsDefinitionChangeInsteadOfRealChange(t *testing.T) {
	run, _, sources := threePeriodRun()
	src := sources(t)
	latest := src.Observations["a3"][2]
	changed := *latest
	te := *latest.Temporal
	te.Metric.Version = "customs-value-v2"
	changed.Temporal = &te
	src.Observations["a3"][2] = &changed

	tl := BuildLongitudinalTimeline(run, src)
	found := false
	for _, e := range tl.EvidenceEvents {
		if e.Kind == domain.EvidenceDefinitionChanged && e.IterationID == "it-3" {
			found = true
		}
	}
	var last analytical.ObservationDelta
	_ = json.Unmarshal(tl.ObservationDeltas[len(tl.ObservationDeltas)-1].Delta, &last)
	if !found || last.Valid {
		t.Fatalf("definition change must be an event and suppress arithmetic: found=%v valid=%v", found, last.Valid)
	}
}

func TestBuildLongitudinalTimelineMarksMissingFingerprintAsUnknownInstrument(t *testing.T) {
	run, _, sources := threePeriodRun()
	src := sources(t)
	src.Analyses["a2"] = analysis("a2", "", "v1")
	tl := BuildLongitudinalTimeline(run, src)
	if len(tl.InstrumentChanges) != 2 || tl.InstrumentChanges[0].Execution != domain.FingerprintUnknown {
		t.Fatalf("instrument lane = %+v", tl.InstrumentChanges)
	}
	if tl.HypothesisEvents[0].Attribution != domain.AttributedToEvidence {
		t.Fatal("UNKNOWN instrument state must not be claimed as an instrument change")
	}
}

func TestValidateNextWindowRejectsRetroactiveAsOf(t *testing.T) {
	run, _, _ := threePeriodRun()
	if err := domain.ValidateNextWindow(run, window(2023)); err == nil {
		t.Fatal("earlier as-of accepted")
	}
	if err := domain.ValidateNextWindow(run, window(2025)); err != nil {
		t.Fatal(err)
	}
}

func TestBuildLongitudinalTimelineDoesNotTreatIncrementalEvidenceAsRemoval(t *testing.T) {
	run := domain.ResearchRun{ID: "r", Iterations: []domain.ResearchIteration{
		{ID: "i1", Sequence: 1},
		{ID: "i2", Sequence: 2, AddedEvidence: []string{"e-2023"}},
		{ID: "i3", Sequence: 3, AddedEvidence: []string{"e-2024"}},
	}}
	tl := BuildLongitudinalTimeline(run, TimelineSources{})
	if len(tl.EvidenceEvents) != 2 {
		t.Fatalf("events = %+v", tl.EvidenceEvents)
	}
	for _, e := range tl.EvidenceEvents {
		if e.Kind != domain.EvidenceAdded {
			t.Fatalf("incremental evidence produced %s for %s", e.Kind, e.Reference)
		}
	}
	if tl.InsightVersions[0].Attribution != domain.AttributedToEvidence || tl.InsightVersions[1].Attribution != domain.AttributedToEvidence {
		t.Fatalf("attribution = %+v", tl.InsightVersions)
	}
}
