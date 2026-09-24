package domain

import (
	"fmt"
	"strings"
	"time"
)

// ScaffoldInput is what a deterministic scenario scaffold needs. Baseline
// and horizon are required from the caller: the engine never guesses when
// "now" is or how far ahead a scenario looks.
type ScaffoldInput struct {
	ResearchRunID string
	Question      string
	IterationID   string
	Insights      []*Insight
	Baseline      ScenarioBaseline
	Horizon       ScenarioHorizon
	Now           time.Time
}

// ScaffoldScenarioSet drafts one scenario per hypothesis that has not been
// contradicted, and one per competing hypothesis it carries. Each
// falsification criterion becomes a frozen expectation to be tested only by
// observations after the baseline (DERIVED_FROM_PRIOR_RUN). Hypotheses
// without any falsification criterion cannot become testable scenarios and
// are returned in skipped instead of being given an invented expectation.
func ScaffoldScenarioSet(in ScaffoldInput) (ScenarioSet, []string, error) {
	set := ScenarioSet{ResearchRunID: in.ResearchRunID, Question: in.Question, Baseline: in.Baseline,
		NonExhaustive: true, Origin: ScenarioOriginScaffold, FromIterationID: in.IterationID, CreatedAt: in.Now}
	window := TimeWindow{Start: in.Baseline.AsOf, End: in.Horizon.End}
	var skipped []string
	add := func(title, mechanism, rationale, sourceRef, hypothesisID string, criteria, missing []string) {
		if len(criteria) == 0 {
			skipped = append(skipped, fmt.Sprintf("%s: no falsification criterion, so no testable scenario was drafted", title))
			return
		}
		n := len(set.Scenarios) + 1
		sc := Scenario{ID: fmt.Sprintf("S%d", n), Title: title, Horizon: in.Horizon, DerivedFromHypothesisID: hypothesisID,
			EvidenceRefs: append([]string(nil), in.Baseline.EvidenceRefs...)}
		if strings.TrimSpace(mechanism) != "" {
			sc.Mechanisms = []string{mechanism}
		}
		if strings.TrimSpace(rationale) != "" {
			sc.Assumptions = append(sc.Assumptions, Assumption{ID: fmt.Sprintf("S%d-A1", n), Statement: rationale,
				Kind: AssumptionModel, SourceRefs: []string{sourceRef}, AsOf: in.Baseline.AsOf})
		}
		for i, c := range criteria {
			sc.Expectations = append(sc.Expectations, ScenarioExpectation{
				ID: fmt.Sprintf("S%d-E%d", n, i+1), Indicator: "evidence bearing on: " + title,
				Statement:         "Observations after the baseline stay consistent with: " + title,
				ObservationWindow: window, FalsificationCondition: c,
				Provenance: ExpectationDerivedFromPriorRun, SourceRefs: []string{sourceRef},
			})
		}
		for _, m := range missing {
			sc.Limitations = append(sc.Limitations, "missing evidence: "+m)
		}
		sc.Limitations = append(sc.Limitations, "scaffolded from a hypothesis; indicators are placeholders until a human names concrete measures")
		set.Scenarios = append(set.Scenarios, sc)
	}
	for _, insight := range in.Insights {
		if insight == nil || insight.ValidationStatus == ValidationContradicted {
			continue
		}
		ref := "insight:" + insight.ID
		add(insight.Title, insight.LatentNeed, insight.Rationale, ref, insight.ID, insight.FalsificationCriteria, insight.MissingEvidence)
		for _, alt := range insight.CompetingHypotheses {
			add(alt.Title, alt.Explanation, alt.Rationale, ref, insight.ID, alt.FalsificationCriteria, alt.MissingEvidence)
		}
	}
	if len(set.Scenarios) == 0 {
		return ScenarioSet{}, skipped, scenarioErr("no hypothesis with a falsification criterion to scaffold from")
	}
	return set, skipped, nil
}
