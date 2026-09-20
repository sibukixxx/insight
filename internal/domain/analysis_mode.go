package domain

import (
	"fmt"
	"strings"
)

// AnalysisMode describes how an input should be interpreted. It is
// orthogonal to ExecutionMode (deterministic/model-backed) and ResearchStage.
type AnalysisMode string

const (
	AnalysisModeDiscovery       AnalysisMode = "DISCOVERY"
	AnalysisModeDatasetAnalysis AnalysisMode = "DATASET_ANALYSIS"
	AnalysisModeResearchReview  AnalysisMode = "RESEARCH_REVIEW"
)

func (m AnalysisMode) Valid() bool {
	switch m {
	case AnalysisModeDiscovery, AnalysisModeDatasetAnalysis, AnalysisModeResearchReview:
		return true
	}
	return false
}

func (m AnalysisMode) Normalize() AnalysisMode {
	if m == "" {
		return AnalysisModeDiscovery
	}
	return m
}

type InputArtifactKind string

const (
	ArtifactInterview      InputArtifactKind = "INTERVIEW"
	ArtifactReview         InputArtifactKind = "REVIEW"
	ArtifactDataset        InputArtifactKind = "DATASET"
	ArtifactDocument       InputArtifactKind = "DOCUMENT"
	ArtifactResearchReport InputArtifactKind = "RESEARCH_REPORT"
	ArtifactAIAnalysis     InputArtifactKind = "AI_ANALYSIS"
	ArtifactOther          InputArtifactKind = "OTHER"
)

func (k InputArtifactKind) Valid() bool {
	switch k {
	case ArtifactInterview, ArtifactReview, ArtifactDataset, ArtifactDocument,
		ArtifactResearchReport, ArtifactAIAnalysis, ArtifactOther:
		return true
	}
	return false
}

type InputArtifact struct {
	Reference string            `json:"reference"`
	Kind      InputArtifactKind `json:"kind"`
	Title     string            `json:"title,omitempty"`
}

// ResearchClaim represents a statement imported for review. The claim itself
// is never primary evidence; EvidenceReferences point to the underlying
// material that may support or contradict it.
type ResearchClaim struct {
	ID                 string   `json:"id"`
	Statement          string   `json:"statement"`
	SourceReference    string   `json:"sourceReference,omitempty"`
	EvidenceReferences []string `json:"evidenceReferences,omitempty"`
	Assumptions        []string `json:"assumptions,omitempty"`
}

func (c ResearchClaim) Validate() error {
	if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Statement) == "" {
		return fmt.Errorf("research claim requires id and statement")
	}
	return nil
}

func ValidateAnalysisModeInput(mode AnalysisMode, artifacts []InputArtifact, claims []ResearchClaim) error {
	mode = mode.Normalize()
	if !mode.Valid() {
		return fmt.Errorf("invalid semantic analysis mode %q", mode)
	}
	for _, artifact := range artifacts {
		if strings.TrimSpace(artifact.Reference) == "" || !artifact.Kind.Valid() {
			return fmt.Errorf("invalid input artifact")
		}
	}
	for _, claim := range claims {
		if err := claim.Validate(); err != nil {
			return err
		}
	}
	return nil
}
