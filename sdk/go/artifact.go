package insight

import "encoding/json"

// ResearchArtifactView is a read-only, forward-tolerant view of the research
// artifact fields the contract promises. Unknown artifact fields are ignored.
// Values are research states, never consumer actions: nothing here tells a
// caller what to buy, sell, build or launch.
type ResearchArtifactView struct {
	ArtifactSchema       string              `json:"artifactSchema"`
	SchemaVersion        string              `json:"schemaVersion"`
	AnalysisID           string              `json:"analysisId"`
	ResearchQuestion     string              `json:"researchQuestion"`
	ResearchStage        string              `json:"researchStage"`
	Insights             []ArtifactInsight   `json:"insights"`
	ResearchGaps         []ResearchGap       `json:"researchGaps"`
	NextDataRequirements []DataRequirement   `json:"nextDataRequirements"`
	WhatWeCannotConclude []string            `json:"whatWeCannotConclude"`
	AddedEvidenceLinks   []AddedEvidenceLink `json:"addedEvidenceLinks"`
	InsightDelta         json.RawMessage     `json:"insightDelta"`
	Readiness            Readiness           `json:"decisionReadiness"`
	EffectiveReadiness   string              `json:"effectiveDecisionReadiness"`
	Provenance           *AnalysisProvenance `json:"provenance"`
}

// ArtifactInsight is a hypothesis with its evidence. ValidationStatus and
// IdentificationStatus are research states, not probabilities.
type ArtifactInsight struct {
	ID                   string             `json:"id"`
	Title                string             `json:"title"`
	HypothesisRole       string             `json:"hypothesisRole"`
	HypothesisSetID      string             `json:"hypothesisSetId"`
	ValidationStatus     string             `json:"validationStatus"`
	IdentificationStatus string             `json:"identificationStatus"`
	CausalStatus         string             `json:"causalStatus"`
	MissingEvidence      []string           `json:"missingEvidence"`
	SupportingEvidence   []ArtifactEvidence `json:"supportingEvidence"`
	CounterEvidence      []ArtifactEvidence `json:"counterEvidence"`
	NeutralEvidence      []ArtifactEvidence `json:"neutralEvidence"`
}

type ArtifactEvidence struct {
	DocumentID    string `json:"DocumentID"`
	ObservationID string `json:"ObservationID"`
	Quote         string `json:"Quote"`
}

type ResearchGap struct {
	ID                    string   `json:"id"`
	Category              string   `json:"category"`
	Need                  string   `json:"need"`
	WhyItMatters          string   `json:"whyItMatters"`
	AffectedHypothesisIDs []string `json:"affectedHypothesisIds"`
	Resolved              bool     `json:"resolved"`
}

type DataRequirement struct {
	GapID                   string   `json:"gapId"`
	Need                    string   `json:"need"`
	Reason                  string   `json:"reason"`
	RequiredDimensions      []string `json:"requiredDimensions"`
	RequiredPeriod          string   `json:"requiredPeriod"`
	RequiredPopulation      string   `json:"requiredPopulation"`
	SuggestedSourceCategory string   `json:"suggestedSourceCategory"`
}

type Readiness struct {
	State   string   `json:"state"`
	Reasons []string `json:"reasons"`
}

// View decodes the promised artifact fields.
func (r ResearchResult) View() (ResearchArtifactView, error) {
	var view ResearchArtifactView
	err := json.Unmarshal(r.Artifact, &view)
	return view, err
}
