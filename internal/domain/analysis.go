package domain

import (
 "time"
 "insight-lab/internal/analytical/model"
)

type Observation struct {
 Temporal *model.TemporalEvidence `json:"temporal,omitempty"`
	ID string
	// AnalysisID is the run that produced the observation. It is empty for
	// observations recorded before runs owned them, when the run could not
	// be proven from pattern or evidence links.
	AnalysisID  string
	DocumentID  string
	Quote       string
	StartOffset int
	EndOffset   int
	Behavior    string
	Topic       string
	CreatedAt   time.Time
}

type AnalysisStatus string

const (
	AnalysisQueued    AnalysisStatus = "queued"
	AnalysisRunning   AnalysisStatus = "running"
	AnalysisCompleted AnalysisStatus = "completed"
	AnalysisFailed    AnalysisStatus = "failed"
)

type Analysis struct {
	ID          string
	ProjectID   string
	Status      AnalysisStatus
	CurrentStep string
	Progress    int
	Error       string
	Metrics     string
	// Label and Note are optional human descriptions of why the run exists.
	Label string
	Note  string
	// SemanticAnalysisMode is how the requester asked the input to be read,
	// or empty when the request did not say.
	SemanticAnalysisMode AnalysisMode
	// ExecutionSnapshot and InputSnapshot are the JSON snapshots captured at
	// enqueue time and at run start. Both are empty for runs recorded before
	// snapshots existed; that means "not recorded", never "same".
	ExecutionSnapshot    string
	InputSnapshot        string
	ExecutionFingerprint string
	InputFingerprint     string
	StartedAt            *time.Time
	FinishedAt           *time.Time
	CreatedAt            time.Time
}
