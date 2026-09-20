package domain

import (
	"fmt"
	"strings"
)

// AnalysisMode describes how supplied information should be interpreted.
// It is orthogonal to ResearchStage (research maturity) and ExecutionMode
// (whether a model executed).
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

// ArtifactClass describes the semantic maturity of an input, not its file
// extension. PDF/CSV/DOCX remain adapter concerns.
type ArtifactClass string

const (
	ArtifactClassRawEvidence        ArtifactClass = "RAW_EVIDENCE"
	ArtifactClassStructuredDataset  ArtifactClass = "STRUCTURED_DATASET"
	ArtifactClassResearchArtifact   ArtifactClass = "RESEARCH_ARTIFACT"
	ArtifactClassMixed              ArtifactClass = "MIXED"
)

// ResolveAnalysisMode applies explicit mode selection first, then a small
// deterministic mapping from semantic artifact class. Mixed inputs require an
// explicit choice so the application never silently treats a report as raw
// evidence or a dataset as prose.
func ResolveAnalysisMode(explicit AnalysisMode, class ArtifactClass) (AnalysisMode, error) {
	if explicit != "" {
		if !explicit.Valid() {
			return "", fmt.Errorf("invalid analysis mode %q", explicit)
		}
		return explicit, nil
	}
	switch class {
	case "", ArtifactClassRawEvidence:
		return AnalysisModeDiscovery, nil
	case ArtifactClassStructuredDataset:
		return AnalysisModeDatasetAnalysis, nil
	case ArtifactClassResearchArtifact:
		return AnalysisModeResearchReview, nil
	case ArtifactClassMixed:
		return "", fmt.Errorf("mixed artifacts require an explicit analysis mode")
	default:
		return "", fmt.Errorf("invalid artifact class %q", class)
	}
}

type ClaimKind string

const (
	ClaimSourceStatement ClaimKind = "SOURCE_STATEMENT"
	ClaimInterpretation  ClaimKind = "INTERPRETATION"
	ClaimAssumption      ClaimKind = "ASSUMPTION"
	ClaimRecommendation  ClaimKind = "RECOMMENDATION"
)

// Claim is used by RESEARCH_REVIEW to keep an external report/AI answer's
// assertions separate from direct observations. A claim is never primary
// evidence by itself; UnderlyingEvidenceReferences point to evidence that may
// independently support it.
type Claim struct {
	ID                           string    `json:"id"`
	Statement                    string    `json:"statement"`
	Kind                         ClaimKind `json:"kind"`
	SourceReference              string    `json:"sourceReference,omitempty"`
	UnderlyingEvidenceReferences []string  `json:"underlyingEvidenceReferences,omitempty"`
	Assumptions                  []string  `json:"assumptions,omitempty"`
}

func (c Claim) Validate() error {
	if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Statement) == "" {
		return fmt.Errorf("claim id and statement are required")
	}
	switch c.Kind {
	case ClaimSourceStatement, ClaimInterpretation, ClaimAssumption, ClaimRecommendation:
		return nil
	default:
		return fmt.Errorf("invalid claim kind %q", c.Kind)
	}
}

func (c Claim) HasUnderlyingEvidence() bool {
	return len(c.UnderlyingEvidenceReferences) > 0
}
