package domain

import "fmt"

// buildScenarioDelta compares the new states with the previous evaluation
// (or UNTESTED for the first). It reports movement per branch and never
// orders or selects scenarios.
func buildScenarioDelta(prev *ScenarioEvaluation, eval ScenarioEvaluation, in NewObservationInput, allFired []FiredFalsification) ScenarioDelta {
	d := ScenarioDelta{ToEvaluationID: eval.ID}
	before := map[string]ScenarioStatus{}
	prevReqs := map[string]bool{}
	if prev != nil {
		d.FromEvaluationID = prev.ID
		for _, st := range prev.States {
			before[st.ScenarioID] = st.Status
		}
		for _, r := range prev.DataRequirements {
			prevReqs[r.GapID] = true
		}
	}
	for _, st := range eval.States {
		from, ok := before[st.ScenarioID]
		if !ok {
			from = ScenarioUntested
		}
		change := ScenarioStatusChange{ScenarioID: st.ScenarioID, From: from, To: st.Status}
		switch {
		case from == st.Status:
			d.Unchanged = append(d.Unchanged, st.ScenarioID)
		case st.Status == ScenarioContradicted:
			d.Contradicted = append(d.Contradicted, change)
		case st.Status == ScenarioWeakened:
			d.Weakened = append(d.Weakened, change)
		case st.Status == ScenarioConsistentSoFar:
			d.Strengthened = append(d.Strengthened, change)
		default:
			// e.g. UNTESTED -> INCONCLUSIVE: evidence arrived but decided nothing.
			d.Explanation = append(d.Explanation, fmt.Sprintf("scenario %s moved %s -> %s", st.ScenarioID, from, st.Status))
		}
	}
	newRefs := map[string]bool{}
	for _, o := range in.Observations {
		if o.EvidenceRef != "" {
			newRefs[o.ExpectationID+"\x00"+o.EvidenceRef] = true
		}
	}
	for _, f := range allFired {
		if newRefs[f.ExpectationID+"\x00"+f.EvidenceRef] {
			d.FalsificationsFired = append(d.FalsificationsFired, f)
			d.Explanation = append(d.Explanation, fmt.Sprintf("scenario %s: falsification condition %q fired on %s", f.ScenarioID, f.Condition, f.EvidenceRef))
		}
	}
	for _, c := range in.AssumptionChecks {
		if c.Result == AssumptionInvalidated {
			d.AssumptionsInvalidated = append(d.AssumptionsInvalidated, c.AssumptionID)
			d.Explanation = append(d.Explanation, fmt.Sprintf("assumption %s invalidated by %s", c.AssumptionID, c.EvidenceRef))
		}
	}
	for _, r := range eval.DataRequirements {
		if prev != nil && !prevReqs[r.GapID] {
			d.NewlyRequiredEvidence = append(d.NewlyRequiredEvidence, r.GapID)
		}
	}
	if len(d.Explanation) == 0 && len(d.Strengthened)+len(d.Weakened)+len(d.Contradicted) == 0 {
		d.Explanation = append(d.Explanation, "no scenario changed state")
	}
	return d
}
