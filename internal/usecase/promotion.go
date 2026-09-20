package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"insight-lab/internal/domain"
	"insight-lab/internal/service"
)

// SubmitPromotionReviewInput carries a human's promotion review submission
// for the latest iteration of a research run (issue #24). Contribution,
// HumanReviewCompleted, MakesStrongClaim, HasUnresolvedCriticalGap, and the
// three judgment-based checklist items are always supplied explicitly by a
// human; nothing here is inferred by the system on its own.
type SubmitPromotionReviewInput struct {
	RunID                               string
	IterationID                         string
	Contribution                        domain.ContributionType
	HumanReviewCompleted                bool
	MakesStrongClaim                    bool
	HasUnresolvedCriticalGap            bool
	CompetingHypothesisConsidered       bool
	IndependentValidationStatusAccurate bool
	DecisionReadinessHonestlyStated     bool
}

// SubmitPromotionReview assembles a domain.PromotionGateInput from the
// research artifact's mechanically-verifiable facts plus the human's
// explicit review, then advances the latest iteration's promotion state as
// far as the gate allows. It never applies a transition beyond
// PUBLICATION_READY: moving to PUBLISHED or REJECTED_FOR_PUBLICATION is a
// separate, explicit act, see TransitionPromotionState.
func (a *Application) SubmitPromotionReview(ctx context.Context, in SubmitPromotionReviewInput) (*domain.ResearchIteration, error) {
	run, err := a.repos.Research.GetResearchRun(ctx, in.RunID)
	if err != nil {
		return nil, err
	}
	latest, ok := run.LatestIteration()
	if !ok || latest.ID != in.IterationID {
		return nil, fmt.Errorf("promotion review can only be applied to the latest research iteration")
	}
	if run.CurrentPromotionState() == domain.PromotionPublished || run.CurrentPromotionState() == domain.PromotionRejectedForPublication {
		return nil, domain.ErrPromotionTransitionNotAllowed
	}
	facts, err := a.promotionArtifactFacts(ctx, *run)
	if err != nil {
		return nil, err
	}
	checklist := service.BuildPublicationChecklist(facts, domain.PublicationChecklist{
		CompetingHypothesisConsidered:       in.CompetingHypothesisConsidered,
		IndependentValidationStatusAccurate: in.IndependentValidationStatusAccurate,
		DecisionReadinessHonestlyStated:     in.DecisionReadinessHonestlyStated,
	})
	gateInput := domain.PromotionGateInput{
		Contribution: in.Contribution, Checklist: checklist, HumanReviewCompleted: in.HumanReviewCompleted,
		MakesStrongClaim: in.MakesStrongClaim, HasUnresolvedCriticalGap: in.HasUnresolvedCriticalGap,
	}
	updated := latest
	updated.PromotionGateInput = gateInput
	updated.Promotion = service.AssessPromotion(domain.PromotionDraft, gateInput, a.now())
	updated.ApprovedArtifact = nil
	if updated.Promotion.State == domain.PromotionPublicationReady {
		artifact, err := a.GetResearchArtifact(ctx, run.ID)
		if err != nil {
			return nil, err
		}
		artifact.Promotion = updated.Promotion
		artifact.PromotionGateInput = updated.PromotionGateInput
		data, err := json.Marshal(artifact)
		if err != nil {
			return nil, err
		}
		updated.ApprovedArtifact = &domain.ApprovedResearchArtifact{Reference: fmt.Sprintf("sha256:%x", sha256.Sum256(data)), Artifact: data, ApprovedAt: updated.Promotion.AssessedAt}
	}
	if err := a.repos.Research.UpdateResearchIteration(ctx, run.ID, updated); err != nil {
		return nil, fmt.Errorf("update research iteration: %w", err)
	}
	return &updated, nil
}

// TransitionPromotionStateInput carries an explicit human request to move a
// research run's promotion state, for moves SubmitPromotionReview never
// makes on its own: publishing (PUBLICATION_READY -> PUBLISHED) or rejecting
// (any non-terminal state -> REJECTED_FOR_PUBLICATION).
type TransitionPromotionStateInput struct {
	RunID       string
	IterationID string
	TargetState domain.PromotionState
}

// TransitionPromotionState checks TargetState against the promotion state
// machine using the gate evidence from the latest recorded review, then
// persists it. Like SubmitPromotionReview, only the latest iteration of a
// run may transition, because the run's current promotion state is always
// read from its latest iteration.
func (a *Application) TransitionPromotionState(ctx context.Context, in TransitionPromotionStateInput) (*domain.ResearchIteration, error) {
	run, err := a.repos.Research.GetResearchRun(ctx, in.RunID)
	if err != nil {
		return nil, err
	}
	latest, ok := run.LatestIteration()
	if !ok || latest.ID != in.IterationID {
		return nil, fmt.Errorf("promotion transition can only be applied to the latest research iteration")
	}
	current := run.CurrentPromotionState()
	if err := current.Transition(in.TargetState, latest.PromotionGateInput); err != nil {
		return nil, err
	}
	if in.TargetState == domain.PromotionPublished && latest.ApprovedArtifact == nil {
		return nil, fmt.Errorf("approved artifact missing; submit a new review")
	}
	updated := latest
	updated.Promotion = domain.PromotionAssessment{State: in.TargetState, AssessedAt: a.now()}
	if err := a.repos.Research.UpdateResearchIteration(ctx, run.ID, updated); err != nil {
		return nil, fmt.Errorf("update research iteration: %w", err)
	}
	return &updated, nil
}

// promotionArtifactFacts gathers the structural facts about run's referenced
// insights and latest analysis metrics that service.BuildPublicationChecklist
// can verify mechanically, without any human judgment call.
func (a *Application) promotionArtifactFacts(ctx context.Context, run domain.ResearchRun) (service.PromotionArtifactFacts, error) {
	facts := service.PromotionArtifactFacts{Stage: run.CurrentStage()}
	if latest, ok := run.LatestIteration(); ok {
		facts.ResearchGapsDisclosed = len(latest.ResearchGaps) > 0
		facts.UnresolvableConclusionsDisclosed = len(latest.WhatWeCannotConclude) > 0
	}
	if iteration, ok := run.LatestIteration(); ok {
		if metrics, ok := a.iterationMetrics(ctx, run.ProjectID, iteration); ok {
			facts.ProvenanceMode = string(metrics.Provenance.Mode)
			facts.DatasetHashesPresent = len(metrics.Provenance.DatasetHashes) > 0
			facts.GroundedObservations = metrics.GroundedObservations
			facts.TotalObservationCandidates = metrics.TotalObservationCandidates
			facts.CompatibilityWarningsPresent = len(metrics.Provenance.CompatibilityWarnings) > 0
			facts.CounterEvidenceSearched = metrics.CounterEvidenceCoverage == 1
		}
	}

	referenced := map[string]bool{}
	if iteration, ok := run.LatestIteration(); ok {
		for _, id := range iteration.InsightIDs {
			referenced[id] = true
		}
	}
	if len(referenced) == 0 {
		return facts, nil
	}
	facts.AllInsightsHaveExpectationBasis, facts.AllInsightsHaveEvidence = true, true
	for id := range referenced {
		insight, err := a.repos.Insights.Get(ctx, id)
		if err != nil {
			return service.PromotionArtifactFacts{}, fmt.Errorf("promotion facts: get insight %s: %w", id, err)
		}
		if insight.ExpectationBasis == "" {
			facts.AllInsightsHaveExpectationBasis = false
		}
		if len(insight.MissingEvidence) > 0 {
			facts.AnyLimitationsDisclosed = true
		}
		evidence, err := a.repos.Evidence.ListByInsight(ctx, id)
		if err != nil {
			return service.PromotionArtifactFacts{}, fmt.Errorf("promotion facts: list evidence %s: %w", id, err)
		}
		if len(evidence) == 0 {
			facts.AllInsightsHaveEvidence = false
		}
		for _, e := range evidence {
			if e.Type == domain.EvidenceCounter {
				facts.CounterEvidenceSearched = true
			}
		}
	}
	return facts, nil
}
