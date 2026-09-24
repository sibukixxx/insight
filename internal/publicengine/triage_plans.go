package publicengine

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"insight-lab/internal/repository"
	"insight-lab/internal/triage"
)

// maxTriageGaps bounds gap and hypothesis references per triage request.
const maxTriageGaps = 100

// Triage proposes a new Selection Plan version for a profile. A re-triage
// (for example after new ResearchGaps) becomes the next version, parented on
// the latest plan, with every changed placement recorded as a move.
func (e *Engine) Triage(ctx context.Context, subjectID, profileID string, req TriageRequest) (int, []byte, error) {
	deps, err := e.triageEnabled()
	if err != nil {
		return 0, nil, err
	}
	return e.idempotent(ctx, req.ContractVersion, req.IdempotencyKey, "triage:"+subjectID+":"+profileID, req, func() (int, any, error) {
		profile, err := e.GetDatasetProfile(ctx, subjectID, profileID)
		if err != nil {
			return 0, nil, err
		}
		if strings.TrimSpace(req.Question) == "" || len(req.Question) > maxQuestionLength {
			return 0, nil, newError(CodeInvalidRequest, "question must be 1-%d characters", maxQuestionLength)
		}
		if len(req.Gaps) > maxTriageGaps || len(req.HypothesisIDs) > maxTriageGaps {
			return 0, nil, newError(CodeInvalidRequest, "at most %d gaps and %d hypothesisIds", maxTriageGaps, maxTriageGaps)
		}
		for i, g := range req.Gaps {
			if strings.TrimSpace(g.GapID) == "" || len(g.GapID) > maxIdentifierLength || len(g.Need) > maxQuestionLength {
				return 0, nil, newError(CodeInvalidRequest, "gaps[%d]: gapId must be 1-%d characters and need at most %d", i, maxIdentifierLength, maxQuestionLength)
			}
		}
		gaps := append([]TriageGapRef(nil), req.Gaps...)
		if req.ResearchRunID != "" {
			fromRun, err := e.gapsFromRun(ctx, subjectID, req.ResearchRunID)
			if err != nil {
				return 0, nil, err
			}
			gaps = mergeGaps(gaps, fromRun)
		}
		in := triage.Input{Question: req.Question, HypothesisIDs: req.HypothesisIDs, Gaps: gaps, Profile: profileOf(profile)}
		proposer := PlanProposer{Kind: triage.ProposerDeterministic}
		var proposed []VariableDecision
		switch req.Triager {
		case "", string(triage.ProposerDeterministic):
			proposed = triage.Deterministic(in)
		case string(triage.ProposerModel):
			if deps.model == nil {
				return 0, nil, newError(CodeInvalidRequest, "model-backed triage requires a configured model; use triager DETERMINISTIC")
			}
			client, model, ok := deps.model()
			if !ok {
				return 0, nil, newError(CodeInvalidRequest, "model-backed triage requires a configured model; use triager DETERMINISTIC")
			}
			if proposed, err = triage.Model(ctx, client, in); err != nil {
				return 0, nil, err
			}
			proposer = PlanProposer{Kind: triage.ProposerModel, Model: model}
		default:
			return 0, nil, newError(CodeInvalidRequest, "triager must be DETERMINISTIC or MODEL")
		}
		decisions, err := triage.Normalize(in.Profile, proposed, proposer.Kind)
		if err != nil {
			return 0, nil, newError(CodeInvalidRequest, "%v", err)
		}
		plan := SelectionPlan{Question: req.Question, Proposer: proposer, HypothesisIDs: req.HypothesisIDs, Gaps: gaps, Decisions: decisions}
		return e.storePlan(ctx, deps, subjectID, profileID, "", plan, true)
	})
}

// ReviseSelectionPlan applies human moves to a plan and stores the result as
// a new version. Only a human revision may EXCLUDE a variable, and exclusion
// never deletes source data.
func (e *Engine) ReviseSelectionPlan(ctx context.Context, planID string, req ReviseSelectionPlanRequest) (int, []byte, error) {
	deps, err := e.triageEnabled()
	if err != nil {
		return 0, nil, err
	}
	return e.idempotent(ctx, req.ContractVersion, req.IdempotencyKey, "reviseSelectionPlan:"+planID, req, func() (int, any, error) {
		base, err := e.GetSelectionPlan(ctx, planID)
		if err != nil {
			return 0, nil, err
		}
		if strings.TrimSpace(req.Actor) == "" || len(req.Actor) > maxIdentifierLength {
			return 0, nil, newError(CodeInvalidRequest, "actor must be 1-%d characters", maxIdentifierLength)
		}
		decisions, moves, err := triage.Revise(base.Decisions, req.Moves)
		if err != nil {
			return 0, nil, newError(CodeInvalidRequest, "%v", err)
		}
		plan := SelectionPlan{Question: base.Question, Proposer: PlanProposer{Kind: triage.ProposerHuman, Actor: req.Actor},
			HypothesisIDs: base.HypothesisIDs, Gaps: base.Gaps, Decisions: decisions, Moves: moves}
		return e.storePlan(ctx, deps, base.SubjectID, base.ProfileID, base.PlanID, plan, false)
	})
}

func (e *Engine) storePlan(ctx context.Context, deps *triageDeps, subjectID, profileID, parentID string, plan SelectionPlan, diffFromLatest bool) (int, any, error) {
	existing, err := deps.repo.ListPlans(ctx, profileID)
	if err != nil {
		return 0, nil, err
	}
	if len(existing) > 0 && parentID == "" {
		parentID = existing[len(existing)-1].PlanID
		if diffFromLatest {
			var latest SelectionPlan
			if err := json.Unmarshal(existing[len(existing)-1].Body, &latest); err != nil {
				return 0, nil, err
			}
			plan.Moves = diffMoves(latest.Decisions, plan.Decisions)
		}
	}
	plan.ContractVersion, plan.SubjectID, plan.ProfileID = ContractVersion, subjectID, profileID
	plan.PlanID, plan.Version, plan.ParentPlanID = newID("plan"), len(existing)+1, parentID
	plan.ProcessingBoundary, plan.CreatedAt = triage.ProcessingBoundary, formatTime(e.now())
	body, err := json.Marshal(plan)
	if err != nil {
		return 0, nil, err
	}
	stored := &repository.StoredSelectionPlan{PlanID: plan.PlanID, ProjectID: subjectID, ProfileID: profileID, Version: plan.Version, ParentPlanID: parentID, Body: body, CreatedAt: e.now()}
	if err := deps.repo.CreatePlan(ctx, stored); err != nil {
		return 0, nil, err
	}
	return http.StatusCreated, plan, nil
}

// diffMoves records how a re-proposal differs from the previous version.
func diffMoves(prev, next []VariableDecision) []PlanMove {
	before := map[string]triage.Bucket{}
	for _, d := range prev {
		before[d.Name] = d.Bucket
	}
	var moves []PlanMove
	for _, d := range next {
		if from, ok := before[d.Name]; ok && from != d.Bucket {
			moves = append(moves, PlanMove{Name: d.Name, FromBucket: from, ToBucket: d.Bucket, Role: d.Role, Classifications: d.Classifications, Rationale: d.Rationale})
		}
	}
	return moves
}

// GetSelectionPlan returns one plan version.
func (e *Engine) GetSelectionPlan(ctx context.Context, planID string) (SelectionPlan, error) {
	var out SelectionPlan
	deps, err := e.triageEnabled()
	if err != nil {
		return out, err
	}
	stored, err := deps.repo.GetPlan(ctx, planID)
	if errors.Is(err, repository.ErrNotFound) {
		return out, newError(CodeNotFound, "selection plan %q not found", planID)
	}
	if err != nil {
		return out, err
	}
	if _, err := e.requireSubject(ctx, stored.ProjectID); err != nil {
		return out, newError(CodeNotFound, "selection plan %q not found", planID)
	}
	return out, json.Unmarshal(stored.Body, &out)
}

// ListSelectionPlans returns every version for a profile, oldest first.
func (e *Engine) ListSelectionPlans(ctx context.Context, subjectID, profileID string) (SelectionPlanList, error) {
	out := SelectionPlanList{ContractVersion: ContractVersion, SubjectID: subjectID, ProfileID: profileID, Plans: []SelectionPlan{}}
	if _, err := e.GetDatasetProfile(ctx, subjectID, profileID); err != nil {
		return out, err
	}
	stored, err := e.triage.repo.ListPlans(ctx, profileID)
	if err != nil {
		return out, err
	}
	for _, s := range stored {
		var p SelectionPlan
		if err := json.Unmarshal(s.Body, &p); err != nil {
			return out, err
		}
		out.Plans = append(out.Plans, p)
	}
	return out, nil
}
