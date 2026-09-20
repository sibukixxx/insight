package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"insight-lab/internal/domain"
)

// BuildResearchIteration projects persisted Insight results into the generic
// Research Loop. It only promotes gaps already present in Insight output; it
// does not invent evidence or upgrade causal identification. Identical
// missing-evidence needs shared by several hypotheses become one gap so that
// its discriminating value across hypotheses is visible.
func BuildResearchIteration(sequence int, question string, inputReferences []string, insights []*domain.Insight, now time.Time) domain.ResearchIteration {
	iteration := domain.ResearchIteration{
		ID: newID("rit"), Sequence: sequence, Stage: domain.StageExploratory, Question: strings.TrimSpace(question),
		InputReferences: append([]string(nil), inputReferences...), CreatedAt: now,
		Promotion: domain.PromotionAssessment{State: domain.PromotionDraft, AssessedAt: now},
	}

	seenSets := map[string]bool{}
	gapIndex := map[string]int{}
	for _, insight := range insights {
		if insight == nil {
			continue
		}
		if insight.ID != "" {
			iteration.InsightIDs = append(iteration.InsightIDs, insight.ID)
			iteration.HypothesisStates = append(iteration.HypothesisStates, domain.HypothesisState{
				HypothesisID: insight.ID, ComparisonKey: hypothesisComparisonKey(insight), ValidationStatus: insight.ValidationStatus,
				IdentificationStatus: insight.IdentificationStatus,
			})
		}
		if strings.TrimSpace(insight.SurprisingFact) != "" {
			iteration.SurpriseIDs = append(iteration.SurpriseIDs, insight.ID)
		}
		if insight.HypothesisSetID != "" && !seenSets[insight.HypothesisSetID] {
			seenSets[insight.HypothesisSetID] = true
			iteration.HypothesisSetIDs = append(iteration.HypothesisSetIDs, insight.HypothesisSetID)
		}
		for _, missing := range insight.MissingEvidence {
			missing = strings.TrimSpace(missing)
			if missing == "" {
				continue
			}
			key := gapNeedKey(missing)
			if idx, exists := gapIndex[key]; exists {
				iteration.ResearchGaps[idx].AffectedHypothesisIDs = appendUnique(iteration.ResearchGaps[idx].AffectedHypothesisIDs, insight.ID)
				continue
			}
			gap := ResearchGapFromMissingEvidence(
				newID("gap"), ClassifyResearchGap(missing), missing,
				"This evidence is missing from the current hypothesis evaluation.", []string{insight.ID},
			)
			gap.FirstSeenIterationID = iteration.ID
			gapIndex[key] = len(iteration.ResearchGaps)
			iteration.ResearchGaps = append(iteration.ResearchGaps, gap)
		}
		if insight.IdentificationStatus == domain.IdentificationNotIdentified {
			iteration.WhatWeCannotConclude = append(iteration.WhatWeCannotConclude,
				"Causality is not identified for hypothesis "+insight.ID+" from the current evidence.")
		}
		if exp, ok := expectationFromInsight(insight, iteration.ID, now); ok {
			iteration.Expectations = append(iteration.Expectations, exp)
		}
	}
	for _, gap := range iteration.ResearchGaps {
		if req, err := PlanDataRequirementForGap(gap); err == nil {
			iteration.DataRequirements = append(iteration.DataRequirements, req)
		}
	}

	sort.Strings(iteration.HypothesisSetIDs)
	return PrioritizeResearchIteration(iteration)
}

// expectationFromInsight projects an Insight's Expectation/ExpectationBasis
// pair into a first-class domain.Expectation. It never rewrites a post-hoc
// basis into a pre-observation one: ObservedDataAvailableAtCreation is set
// purely from the basis's own ObservationTiming, and the basis itself is
// carried verbatim. The result starts unfrozen; freezing it for validation is
// a separate, explicit decision.
func expectationFromInsight(insight *domain.Insight, iterationID string, now time.Time) (domain.Expectation, bool) {
	statement := strings.TrimSpace(insight.Expectation)
	if statement == "" {
		return domain.Expectation{}, false
	}
	return domain.Expectation{
		ID: newID("exp"), Statement: statement, Provenance: insight.ExpectationBasis,
		AuthorType: domain.AuthorModel, ResearchIterationID: iterationID,
		FalsificationCriteria:           append([]string(nil), insight.FalsificationCriteria...),
		ObservedDataAvailableAtCreation: insight.ExpectationBasis.ObservationTiming() == domain.TimingPostObservation,
		CreatedAt:                       now,
	}, true
}

// CarryForwardFrozenExpectations derives a validation target on the new
// iteration for every expectation frozen on the previous one. Unfrozen
// (still-exploratory) expectations are not carried: only a frozen statement
// is safe to test again, and DeriveForValidation records the lineage back to
// its exploratory source without ever mutating the historical iteration.
func CarryForwardFrozenExpectations(previous domain.ResearchIteration, current domain.ResearchIteration, now time.Time) domain.ResearchIteration {
	out := current
	for _, e := range previous.Expectations {
		if !e.FrozenForValidation {
			continue
		}
		out.Expectations = append(out.Expectations, e.DeriveForValidation(newID("exp"), current.ID, now))
	}
	return out
}

// FinalizeResearchIteration links a freshly built iteration to the run it
// extends: added evidence, gap carry-forward, hypothesis change history,
// requirement priority, readiness and a system stop decision. The caller
// still decides whether to persist it; nothing here acquires evidence.
func FinalizeResearchIteration(run domain.ResearchRun, iteration domain.ResearchIteration, addedEvidence []string, now time.Time) domain.ResearchIteration {
	return FinalizeResearchIterationWithLinks(run, iteration, addedEvidence, nil, now)
}

func FinalizeResearchIterationWithLinks(run domain.ResearchRun, iteration domain.ResearchIteration, addedEvidence []string, links []domain.AddedEvidenceLink, now time.Time) domain.ResearchIteration {
	iteration.AddedEvidence = append([]string(nil), addedEvidence...)
	iteration.AddedEvidenceLinks = append([]domain.AddedEvidenceLink(nil), links...)
	if previous, ok := run.LatestIteration(); ok {
		iteration = CarryForwardResearchGapsWithLinks(previous, iteration, addedEvidence, links)
		iteration = CarryForwardFrozenExpectations(previous, iteration, now)
		iteration.HypothesisChanges = CompareHypothesisStates(previous.HypothesisStates, iteration.HypothesisStates)
		// The run's stage only moves via an explicit transition (see
		// domain.ResearchStage.Transition); building a new iteration from the
		// latest insights must not silently reset it back to EXPLORATORY.
		iteration.Stage = previous.Stage
	}
	iteration = PrioritizeResearchIteration(iteration)
	candidate := run.AppendIteration(iteration)
	iteration.Readiness = AssessDecisionReadiness(candidate, now)
	iteration.Stop = DecideStop(candidate, iteration.Readiness, now)
	return iteration
}

// CarryForwardResearchGaps keeps gap identity stable across iterations and
// refuses to forget gaps. A gap that disappears from a re-analysis is marked
// addressed only when the iteration actually carried added evidence;
// otherwise it is carried forward unresolved, because a model simply not
// mentioning a gap is not evidence that the gap closed.
func CarryForwardResearchGaps(previous domain.ResearchIteration, current domain.ResearchIteration, addedEvidence []string) domain.ResearchIteration {
	return CarryForwardResearchGapsWithLinks(previous, current, addedEvidence, nil)
}

func CarryForwardResearchGapsWithLinks(previous domain.ResearchIteration, current domain.ResearchIteration, addedEvidence []string, links []domain.AddedEvidenceLink) domain.ResearchIteration {
	out := current
	targeted := map[string]bool{}
	for _, link := range links {
		for _, gapID := range link.GapIDs {
			if strings.TrimSpace(gapID) != "" {
				targeted[gapID] = true
			}
		}
	}
	out.ResearchGaps = append([]domain.ResearchGap(nil), current.ResearchGaps...)
	out.DataRequirements = append([]domain.DataRequirement(nil), current.DataRequirements...)

	currentByKey := map[string]int{}
	for i, gap := range out.ResearchGaps {
		currentByKey[gapNeedKey(gap.Need)] = i
	}
	renamed := map[string]string{}
	for _, prior := range previous.ResearchGaps {
		idx, matched := currentByKey[gapNeedKey(prior.Need)]
		if matched {
			renamed[out.ResearchGaps[idx].ID] = prior.ID
			out.ResearchGaps[idx].ID = prior.ID
			if prior.FirstSeenIterationID != "" {
				out.ResearchGaps[idx].FirstSeenIterationID = prior.FirstSeenIterationID
			}
			continue
		}
		carried := prior
		carried.AffectedHypothesisIDs = append([]string(nil), prior.AffectedHypothesisIDs...)
		carried.DependsOnGapIDs = nil
		if !carried.Resolved {
			if len(links) > 0 {
				if targeted[carried.ID] {
					carried.Resolved = true
					carried.AddressedInIterationID = current.ID
				}
			} else if len(addedEvidence) > 0 {
				// Backward-compatible legacy behavior for callers that have
				// not adopted structured gap linkage yet.
				carried.Resolved = true
				carried.AddressedInIterationID = current.ID
			}
		}
		out.ResearchGaps = append(out.ResearchGaps, carried)
		if !carried.Resolved {
			if req, err := PlanDataRequirementForGap(carried); err == nil {
				out.DataRequirements = append(out.DataRequirements, req)
			}
		}
	}
	for i := range out.DataRequirements {
		if id, ok := renamed[out.DataRequirements[i].GapID]; ok {
			out.DataRequirements[i].GapID = id
		}
	}
	return out
}

// PrioritizeResearchIteration explains which additional evidence would best
// separate the competing hypotheses. Every judgement is qualitative and
// derived from gap category, affected hypotheses and their current status;
// no probability or certainty score is produced.
func PrioritizeResearchIteration(iteration domain.ResearchIteration) domain.ResearchIteration {
	out := iteration
	out.ResearchGaps = append([]domain.ResearchGap(nil), iteration.ResearchGaps...)
	out.DataRequirements = append([]domain.DataRequirement(nil), iteration.DataRequirements...)

	states := map[string]domain.HypothesisState{}
	for _, state := range iteration.HypothesisStates {
		states[state.HypothesisID] = state
	}
	gapsByID := map[string]*domain.ResearchGap{}
	for i := range out.ResearchGaps {
		gapsByID[out.ResearchGaps[i].ID] = &out.ResearchGaps[i]
	}
	for i := range out.ResearchGaps {
		gap := &out.ResearchGaps[i]
		gap.DependsOnGapIDs = gapDependencies(*gap, out.ResearchGaps)
		gap.AffectedHypothesisIDs = appendUnique(nil, gap.AffectedHypothesisIDs...)
	}

	for i := range out.DataRequirements {
		req := &out.DataRequirements[i]
		gap, known := gapsByID[req.GapID]
		if !known {
			continue
		}
		req.AffectedHypothesisIDs = append([]string(nil), gap.AffectedHypothesisIDs...)
		req.DependsOnGapIDs = append([]string(nil), gap.DependsOnGapIDs...)
		req.Priority = prioritizeRequirement(*gap, *req, states)
	}
	sort.SliceStable(out.DataRequirements, func(i, j int) bool {
		return requirementLess(out.DataRequirements[i], out.DataRequirements[j])
	})
	for i := range out.DataRequirements {
		out.DataRequirements[i].Priority.Rank = i + 1
	}
	return out
}

func prioritizeRequirement(gap domain.ResearchGap, req domain.DataRequirement, states map[string]domain.HypothesisState) domain.RequirementPriority {
	p := domain.RequirementPriority{}
	p.DiscriminatingPower, p.Rationale = discriminatingPower(gap)
	if len(gap.AffectedHypothesisIDs) == 0 {
		p.DiscriminatingPower = domain.DiscriminatingNone
		p.Rationale = append(p.Rationale, "no hypothesis is linked to this gap, so it cannot separate explanations")
	}
	p.CanDistinguishHypotheses = len(gap.AffectedHypothesisIDs) >= 2
	p.CanFalsify = len(gap.AffectedHypothesisIDs) >= 1 && gap.Category != domain.ResearchGapOther
	if p.CanDistinguishHypotheses {
		p.Rationale = append(p.Rationale, fmt.Sprintf("affects %d hypotheses (%s): the same evidence can distinguish between them", len(gap.AffectedHypothesisIDs), strings.Join(gap.AffectedHypothesisIDs, ", ")))
	} else if p.CanFalsify {
		p.Rationale = append(p.Rationale, "affects a single hypothesis: it can falsify but not distinguish")
	}

	p.Urgency = domain.UrgencyLow
	for _, id := range gap.AffectedHypothesisIDs {
		state, known := states[id]
		if !known {
			continue
		}
		if p.DiscriminatingPower == domain.DiscriminatingHigh && state.IdentificationStatus != domain.IdentificationIdentified {
			p.Urgency = domain.UrgencyHigh
			p.Rationale = append(p.Rationale, fmt.Sprintf("hypothesis %s is %s; identification-critical evidence is urgent", id, identificationLabel(state.IdentificationStatus)))
			break
		}
		if p.DiscriminatingPower == domain.DiscriminatingModerate || state.ValidationStatus == domain.ValidationUntested || state.ValidationStatus == domain.ValidationInsufficientEvidence {
			p.Urgency = domain.UrgencyMedium
		}
	}

	p.AcquisitionDifficulty = acquisitionDifficulty(gap, req)
	p.Rationale = append(p.Rationale, fmt.Sprintf("acquisition from %q is qualitatively %s; Insight does not acquire it", orDefault(req.SuggestedSourceCategory, "unspecified"), p.AcquisitionDifficulty))
	if len(gap.DependsOnGapIDs) > 0 {
		p.Rationale = append(p.Rationale, "depends on unresolved gap(s) "+strings.Join(gap.DependsOnGapIDs, ", ")+": collect those first")
	}
	return p
}

func discriminatingPower(gap domain.ResearchGap) (domain.DiscriminatingPower, []string) {
	switch gap.Category {
	case domain.ResearchGapComparison, domain.ResearchGapPrePeriod, domain.ResearchGapConfounder:
		return domain.DiscriminatingHigh, []string{string(gap.Category) + " evidence separates a causal explanation from its alternatives"}
	case domain.ResearchGapTiming, domain.ResearchGapMeasurement:
		return domain.DiscriminatingModerate, []string{string(gap.Category) + " evidence can rule out an explanation but rarely settles causality alone"}
	case domain.ResearchGapExternalContext, domain.ResearchGapSourceQuality:
		return domain.DiscriminatingLow, []string{string(gap.Category) + " evidence qualifies the reading of existing data rather than distinguishing hypotheses"}
	}
	return domain.DiscriminatingLow, []string{"unclassified evidence need: discriminating value cannot be asserted"}
}

func acquisitionDifficulty(gap domain.ResearchGap, req domain.DataRequirement) domain.AcquisitionDifficulty {
	if !gap.Resolvable {
		return domain.AcquisitionInfeasible
	}
	switch req.SuggestedSourceCategory {
	case "documented event chronology", "measurement documentation", "source documentation":
		return domain.AcquisitionLow
	case "historical records", "official or audited statistics", "external contextual data":
		return domain.AcquisitionModerate
	case "comparison dataset":
		return domain.AcquisitionHigh
	}
	return domain.AcquisitionUnknown
}

// gapDependencies makes measurement and source-quality gaps prerequisites of
// gaps that compare or adjust the same hypotheses: a comparison over an
// undefined measure cannot discriminate anything.
func gapDependencies(gap domain.ResearchGap, all []domain.ResearchGap) []string {
	switch gap.Category {
	case domain.ResearchGapComparison, domain.ResearchGapPrePeriod, domain.ResearchGapConfounder, domain.ResearchGapExternalContext:
	default:
		return nil
	}
	var deps []string
	for _, other := range all {
		if other.ID == gap.ID || other.Resolved {
			continue
		}
		if other.Category != domain.ResearchGapMeasurement && other.Category != domain.ResearchGapSourceQuality {
			continue
		}
		if sharesHypothesis(gap.AffectedHypothesisIDs, other.AffectedHypothesisIDs) {
			deps = append(deps, other.ID)
		}
	}
	sort.Strings(deps)
	return deps
}

var powerRank = map[domain.DiscriminatingPower]int{domain.DiscriminatingNone: 0, domain.DiscriminatingLow: 1, domain.DiscriminatingModerate: 2, domain.DiscriminatingHigh: 3}
var urgencyRank = map[domain.GapUrgency]int{domain.UrgencyLow: 0, domain.UrgencyMedium: 1, domain.UrgencyHigh: 2}
var difficultyRank = map[domain.AcquisitionDifficulty]int{domain.AcquisitionLow: 0, domain.AcquisitionModerate: 1, domain.AcquisitionUnknown: 2, domain.AcquisitionHigh: 3, domain.AcquisitionInfeasible: 4}

func requirementLess(a, b domain.DataRequirement) bool {
	if powerRank[a.Priority.DiscriminatingPower] != powerRank[b.Priority.DiscriminatingPower] {
		return powerRank[a.Priority.DiscriminatingPower] > powerRank[b.Priority.DiscriminatingPower]
	}
	if urgencyRank[a.Priority.Urgency] != urgencyRank[b.Priority.Urgency] {
		return urgencyRank[a.Priority.Urgency] > urgencyRank[b.Priority.Urgency]
	}
	if len(a.DependsOnGapIDs) != len(b.DependsOnGapIDs) {
		return len(a.DependsOnGapIDs) < len(b.DependsOnGapIDs)
	}
	if difficultyRank[a.Priority.AcquisitionDifficulty] != difficultyRank[b.Priority.AcquisitionDifficulty] {
		return difficultyRank[a.Priority.AcquisitionDifficulty] < difficultyRank[b.Priority.AcquisitionDifficulty]
	}
	return a.Need < b.Need
}

// AssessDecisionReadiness states how prepared the research process is for a
// responsible human decision. It never expresses truth probability, causal
// certainty or a commercial recommendation. A single pass can never reach
// EVIDENCE_CONVERGING or DECISION_READY_WITH_LIMITATIONS.
func AssessDecisionReadiness(run domain.ResearchRun, now time.Time) domain.ReadinessAssessment {
	latest, ok := run.LatestIteration()
	assessment := domain.ReadinessAssessment{IterationCount: len(run.Iterations), AssessedAt: now}
	if !ok {
		assessment.State = domain.ReadinessEvidenceInsufficient
		assessment.Reasons = []string{"no research iteration exists"}
		return assessment
	}
	assessment.UnresolvedGapIDs = latest.UnresolvedGapIDs()
	summary := summarizeHypotheses(latest)
	evidenceEver := false
	for _, iteration := range run.Iterations {
		if len(iteration.AddedEvidence) > 0 {
			evidenceEver = true
		}
	}
	highOpen := highDiscriminatingOpenGaps(latest)

	switch {
	case len(latest.HypothesisStates) == 0:
		assessment.State = domain.ReadinessEvidenceInsufficient
		assessment.Reasons = []string{"no hypothesis has been formed"}
	case len(assessment.UnresolvedGapIDs) > 0 && !anyResolvable(latest):
		assessment.State = domain.ReadinessBlocked
		assessment.Reasons = []string{fmt.Sprintf("%d unresolved gap(s) and none is considered resolvable", len(assessment.UnresolvedGapIDs))}
	case len(summary.supported) == 0:
		assessment.State = domain.ReadinessEvidenceInsufficient
		assessment.Reasons = []string{"no surviving hypothesis is supported or partially supported by evidence"}
	case len(run.Iterations) == 1 || !evidenceEver:
		assessment.State = domain.ReadinessExploratoryOnly
		assessment.Reasons = []string{"only a single analysis pass exists and no evidence has been added; a one-pass summary cannot be decision-ready"}
		if len(run.Iterations) > 1 {
			assessment.Reasons = []string{fmt.Sprintf("%d iterations exist but no iteration carried added evidence; re-analysis alone is still exploratory", len(run.Iterations))}
		}
	case len(highOpen) > 0:
		names := make([]string, 0, len(highOpen))
		for _, gap := range highOpen {
			names = append(names, gap.Need)
		}
		if len(summary.supported) == 1 && len(summary.strong) == 1 {
			assessment.State = domain.ReadinessEvidenceConverging
			assessment.Reasons = []string{fmt.Sprintf("one surviving hypothesis (%s) is supported while %d alternative(s) fell away", summary.strong[0].HypothesisID, len(summary.contradicted))}
		} else {
			assessment.State = domain.ReadinessValidationRequired
			assessment.Reasons = []string{fmt.Sprintf("%d surviving hypotheses still carry evidence support", len(summary.supported))}
		}
		assessment.Reasons = append(assessment.Reasons, "high-discriminating evidence is still missing: "+strings.Join(names, "; "))
	case len(summary.supported) == 1 && len(summary.strong) == 1:
		assessment.State = domain.ReadinessDecisionReadyWithLimitation
		assessment.Reasons = []string{fmt.Sprintf("a single supported hypothesis (%s) survives and no high-discriminating gap remains open", summary.strong[0].HypothesisID)}
		assessment.Reasons = append(assessment.Reasons, limitationReasons(latest, summary)...)
	case len(summary.supported) == 1:
		assessment.State = domain.ReadinessEvidenceConverging
		assessment.Reasons = []string{fmt.Sprintf("hypothesis %s is only partially supported; evidence is converging but not settled", summary.supported[0].HypothesisID)}
	case !summary.moved:
		assessment.State = domain.ReadinessInconclusive
		assessment.Reasons = []string{fmt.Sprintf("%d competing hypotheses remain supported and the added evidence did not change any validation status", len(summary.supported))}
	default:
		assessment.State = domain.ReadinessEvidenceConverging
		assessment.Reasons = []string{fmt.Sprintf("%d competing hypotheses remain supported but validation moved in this iteration", len(summary.supported))}
	}
	if len(assessment.UnresolvedGapIDs) > 0 && assessment.State != domain.ReadinessBlocked {
		assessment.Reasons = append(assessment.Reasons, fmt.Sprintf("%d research gap(s) remain unresolved", len(assessment.UnresolvedGapIDs)))
	}
	return assessment
}

// DecideStop returns a system stop decision when the research process itself
// has a reason to stop, or nil when another iteration is warranted. Human
// stops go through ResearchIteration.ApplyHumanOverride instead.
func DecideStop(run domain.ResearchRun, assessment domain.ReadinessAssessment, now time.Time) *domain.StopDecision {
	latest, ok := run.LatestIteration()
	if !ok {
		return nil
	}
	stop := &domain.StopDecision{Source: domain.StopSourceSystem, ReadinessAtStop: assessment.State, UnresolvedGapIDs: latest.UnresolvedGapIDs(), DecidedAt: now}
	switch assessment.State {
	case domain.ReadinessDecisionReadyWithLimitation:
		stop.Reason = domain.StopHypothesesDistinguished
		stop.Note = "critical hypotheses are sufficiently distinguished; limitations are listed in the readiness assessment"
	case domain.ReadinessBlocked:
		stop.Reason = domain.StopNoFeasibleDataSource
		stop.Note = "remaining gaps are not considered resolvable with any feasible data source"
	case domain.ReadinessInconclusive:
		summary := summarizeHypotheses(latest)
		stop.Reason = domain.StopConflictingEvidenceRemains
		stop.Note = "competing hypotheses remain supported after added evidence"
		if len(stop.UnresolvedGapIDs) == 0 && summary.allUnidentified {
			stop.Reason = domain.StopIdentificationUnresolved
			stop.Note = "competing hypotheses are not causally identified and no further evidence need is recorded"
		}
	default:
		return nil
	}
	return stop
}

// BuildHumanHandoff assembles the final shape handed to a human. It reports
// what survived, what was contradicted, what remains open, why the loop
// stopped, and which evidence to collect next. Stopping never removes gaps.
func BuildHumanHandoff(run domain.ResearchRun) domain.HumanHandoff {
	handoff := domain.HumanHandoff{ResearchRunID: run.ID, Question: run.Question, IterationCount: len(run.Iterations)}
	latest, ok := run.LatestIteration()
	if !ok {
		handoff.Readiness = domain.ReadinessEvidenceInsufficient
		return handoff
	}
	handoff.Readiness = latest.EffectiveReadiness()
	if handoff.Readiness == "" {
		handoff.Readiness = AssessDecisionReadiness(run, latest.CreatedAt).State
	}
	handoff.ReadinessReasons = append([]string(nil), latest.Readiness.Reasons...)
	handoff.Stop = latest.Stop
	handoff.HumanOverrides = append([]domain.HumanOverride(nil), latest.HumanOverrides...)
	handoff.WhatWeCannotConclude = append([]string(nil), latest.WhatWeCannotConclude...)

	summary := summarizeHypotheses(latest)
	handoff.StrongestSurvivingHypotheses = summary.strongest
	handoff.UnresolvedAlternatives = summary.alternatives
	handoff.ContradictedHypotheses = summary.contradicted
	for _, change := range latest.HypothesisChanges {
		if change.Evolution == domain.HypothesisWeakened {
			handoff.WeakenedHypotheses = append(handoff.WeakenedHypotheses, change)
		}
	}
	for _, gap := range latest.ResearchGaps {
		if !gap.Resolved {
			handoff.RemainingUncertainty = append(handoff.RemainingUncertainty, gap)
		}
	}
	for _, iteration := range run.Iterations {
		for _, evidence := range iteration.AddedEvidence {
			handoff.EvidenceAddedAcrossIterations = append(handoff.EvidenceAddedAcrossIterations, fmt.Sprintf("iteration %d: %s", iteration.Sequence, evidence))
		}
	}
	prioritized := PrioritizeResearchIteration(latest)
	for _, req := range prioritized.DataRequirements {
		handoff.SuggestedNextResearch = append(handoff.SuggestedNextResearch, req)
	}
	return handoff
}

type hypothesisSummary struct {
	surviving, supported, strong, contradicted []domain.HypothesisState
	strongest, alternatives                    []domain.HypothesisState
	moved, allUnidentified                     bool
}

var validationRank = map[domain.ValidationStatus]int{
	domain.ValidationContradicted: -1, domain.ValidationUntested: 0, domain.ValidationInsufficientEvidence: 1,
	domain.ValidationPlausible: 2, domain.ValidationPartiallySupported: 3, domain.ValidationSupported: 4,
}

func summarizeHypotheses(iteration domain.ResearchIteration) hypothesisSummary {
	s := hypothesisSummary{allUnidentified: len(iteration.HypothesisStates) > 0}
	best := -1
	for _, state := range iteration.HypothesisStates {
		if state.ValidationStatus == domain.ValidationContradicted {
			s.contradicted = append(s.contradicted, state)
			continue
		}
		s.surviving = append(s.surviving, state)
		if state.IdentificationStatus == domain.IdentificationIdentified {
			s.allUnidentified = false
		}
		switch state.ValidationStatus {
		case domain.ValidationSupported:
			s.strong = append(s.strong, state)
			s.supported = append(s.supported, state)
		case domain.ValidationPartiallySupported:
			s.supported = append(s.supported, state)
		}
		if validationRank[state.ValidationStatus] > best {
			best = validationRank[state.ValidationStatus]
		}
	}
	for _, state := range s.surviving {
		if validationRank[state.ValidationStatus] == best {
			s.strongest = append(s.strongest, state)
		} else {
			s.alternatives = append(s.alternatives, state)
		}
	}
	for _, change := range iteration.HypothesisChanges {
		if change.Evolution != domain.HypothesisUnchanged && change.Evolution != domain.HypothesisCreated {
			s.moved = true
		}
	}
	return s
}

func limitationReasons(iteration domain.ResearchIteration, summary hypothesisSummary) []string {
	var reasons []string
	for _, state := range summary.strong {
		if state.IdentificationStatus != domain.IdentificationIdentified {
			reasons = append(reasons, fmt.Sprintf("limitation: hypothesis %s remains %s; support is evidential, not causal identification", state.HypothesisID, identificationLabel(state.IdentificationStatus)))
		}
	}
	for _, statement := range iteration.WhatWeCannotConclude {
		reasons = append(reasons, "limitation: "+statement)
	}
	return reasons
}

func highDiscriminatingOpenGaps(iteration domain.ResearchIteration) []domain.ResearchGap {
	power := map[string]domain.DiscriminatingPower{}
	for _, req := range iteration.DataRequirements {
		power[req.GapID] = req.Priority.DiscriminatingPower
	}
	var open []domain.ResearchGap
	for _, gap := range iteration.ResearchGaps {
		if gap.Resolved {
			continue
		}
		p, known := power[gap.ID]
		if !known {
			p, _ = discriminatingPower(gap)
		}
		if p == domain.DiscriminatingHigh {
			open = append(open, gap)
		}
	}
	return open
}

func anyResolvable(iteration domain.ResearchIteration) bool {
	for _, gap := range iteration.ResearchGaps {
		if !gap.Resolved && gap.Resolvable {
			return true
		}
	}
	return false
}

func identificationLabel(status domain.IdentificationStatus) string {
	if status == "" {
		return string(domain.IdentificationUnknown)
	}
	return string(status)
}

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func gapNeedKey(need string) string {
	return strings.ToLower(strings.Join(strings.Fields(need), " "))
}

func sharesHypothesis(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}

func appendUnique(list []string, values ...string) []string {
	seen := make(map[string]bool, len(list))
	for _, v := range list {
		seen[v] = true
	}
	for _, v := range values {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		list = append(list, v)
	}
	return list
}

// CompareHypothesisStates describes evidence/validation history. It never
// changes causal or identification status and does not express probability.
func CompareHypothesisStates(previous []domain.HypothesisState, current []domain.HypothesisState) []domain.HypothesisChange {
	old := make(map[string]domain.HypothesisState, len(previous))
	for _, state := range previous {
		old[stateKey(state)] = state
	}
	changes := make([]domain.HypothesisChange, 0, len(current))
	for _, state := range current {
		prior, exists := old[stateKey(state)]
		evolution := domain.HypothesisCreated
		reason := "hypothesis first appears in this iteration"
		if exists {
			evolution, reason = validationEvolution(prior.ValidationStatus, state.ValidationStatus)
		}
		changes = append(changes, domain.HypothesisChange{HypothesisID: state.HypothesisID, Evolution: evolution, Reason: reason})
	}
	return changes
}

func hypothesisComparisonKey(insight *domain.Insight) string {
	return strings.ToLower(strings.Join(strings.Fields(insight.Title), " "))
}

func stateKey(state domain.HypothesisState) string {
	if state.ComparisonKey != "" {
		return state.ComparisonKey
	}
	return state.HypothesisID
}

func validationEvolution(before, after domain.ValidationStatus) (domain.HypothesisEvolution, string) {
	if before == after {
		return domain.HypothesisUnchanged, "validation status is unchanged"
	}
	if after == domain.ValidationContradicted {
		return domain.HypothesisContradicted, "new evaluation is contradicted by evidence"
	}
	rank := map[domain.ValidationStatus]int{
		domain.ValidationUntested: 0, domain.ValidationInsufficientEvidence: 1,
		domain.ValidationPlausible: 2, domain.ValidationPartiallySupported: 3, domain.ValidationSupported: 4,
	}
	if rank[after] > rank[before] {
		return domain.HypothesisStrengthened, "validation status gained evidence support"
	}
	return domain.HypothesisWeakened, "validation status lost evidence support"
}
