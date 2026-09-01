package domain

import "time"

type EvidenceType string

const (
	EvidenceSupport EvidenceType = "support"
	EvidenceCounter EvidenceType = "counter"
	EvidenceNeutral EvidenceType = "neutral"
)

type Evidence struct {
	ID             string
	InsightID      string
	DocumentID     string
	ObservationID  *string
	Quote          string
	Type           EvidenceType
	RelevanceScore float64
	StartOffset    int
	EndOffset      int
}

type Insight struct {
	ID                        string
	ProjectID                 string
	AnalysisID                *string
	Title                     string
	Observation               string
	StatedNeed                string
	LatentNeed                string
	JTBD                      string
	Interpretation            string
	AlternativeInterpretation string
	ProductOpportunity        string
	Confidence                float64
	Evidence                  []Evidence
	CreatedAt                 time.Time
}
