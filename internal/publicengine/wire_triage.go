package publicengine

import "insight-lab/internal/triage"

// Data Triage wire types (#92). They mirror schema.json $defs one to one;
// drift_triage_test.go registers them with the drift check. Element types
// shared with the triage domain are aliases so the wire and the invariant
// cannot diverge.

type (
	ColumnProfile    = triage.ColumnProfile
	VariableDecision = triage.Decision
	PlanMove         = triage.Move
	PlanProposer     = triage.Proposer
	TriageGapRef     = triage.GapRef
)

// DatasetRef identifies the dataset a profile describes. sha256, when sent,
// must match the bytes the engine profiles.
type DatasetRef struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	SHA256  string `json:"sha256,omitempty"`
}

type CreateDatasetProfileRequest struct {
	ContractVersion string     `json:"contractVersion"`
	IdempotencyKey  string     `json:"idempotencyKey"`
	Dataset         DatasetRef `json:"dataset"`
	CSV             string     `json:"csv,omitempty"`
	DocumentID      string     `json:"documentId,omitempty"`
}

type DatasetProfile struct {
	ContractVersion    string          `json:"contractVersion"`
	SubjectID          string          `json:"subjectId"`
	ProfileID          string          `json:"profileId"`
	Dataset            DatasetRef      `json:"dataset"`
	DocumentID         string          `json:"documentId,omitempty"`
	ContentSHA256      string          `json:"contentSha256"`
	RowCount           int             `json:"rowCount"`
	Columns            []ColumnProfile `json:"columns"`
	ProfilerVersion    string          `json:"profilerVersion"`
	ProfileFingerprint string          `json:"profileFingerprint"`
	CreatedAt          string          `json:"createdAt"`
}

type TriageRequest struct {
	ContractVersion string         `json:"contractVersion"`
	IdempotencyKey  string         `json:"idempotencyKey"`
	Question        string         `json:"question"`
	Triager         string         `json:"triager,omitempty"`
	HypothesisIDs   []string       `json:"hypothesisIds,omitempty"`
	Gaps            []TriageGapRef `json:"gaps,omitempty"`
	ResearchRunID   string         `json:"researchRunId,omitempty"`
}

type SelectionPlan struct {
	ContractVersion    string             `json:"contractVersion"`
	SubjectID          string             `json:"subjectId"`
	PlanID             string             `json:"planId"`
	ProfileID          string             `json:"profileId"`
	Version            int                `json:"version"`
	ParentPlanID       string             `json:"parentPlanId,omitempty"`
	Question           string             `json:"question"`
	Proposer           PlanProposer       `json:"proposer"`
	HypothesisIDs      []string           `json:"hypothesisIds,omitempty"`
	Gaps               []TriageGapRef     `json:"gaps,omitempty"`
	Decisions          []VariableDecision `json:"decisions"`
	Moves              []PlanMove         `json:"moves,omitempty"`
	ProcessingBoundary string             `json:"processingBoundary"`
	CreatedAt          string             `json:"createdAt"`
}

type SelectionPlanList struct {
	ContractVersion string          `json:"contractVersion"`
	SubjectID       string          `json:"subjectId"`
	ProfileID       string          `json:"profileId"`
	Plans           []SelectionPlan `json:"plans"`
}

type ReviseSelectionPlanRequest struct {
	ContractVersion string     `json:"contractVersion"`
	IdempotencyKey  string     `json:"idempotencyKey"`
	Actor           string     `json:"actor"`
	Moves           []PlanMove `json:"moves"`
}
