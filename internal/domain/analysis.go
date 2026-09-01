package domain

import "time"

type Observation struct {
	ID          string
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
	StartedAt   *time.Time
	FinishedAt  *time.Time
	CreatedAt   time.Time
}
