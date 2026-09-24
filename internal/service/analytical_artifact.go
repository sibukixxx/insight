package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"insight-lab/internal/analytical"
	"insight-lab/internal/domain"
)

// analyticalArtifactRuleVersion versions how an Analytical Artifact becomes a
// document and its results become observations. Bump it whenever the result
// statement format or the materialization below changes.
const analyticalArtifactRuleVersion = "analytical-artifact/v1"

// Document metadata keys for an ingested Analytical Artifact. The whole
// artifact is kept under MetadataAnalyticalArtifact so observations can be
// re-derived deterministically on every run.
const (
	MetadataAnalyticalArtifact           = "analytical_artifact"
	MetadataAnalyticalArtifactID         = "analytical_artifact_id"
	MetadataAnalyticalArtifactHash       = "analytical_artifact_hash"
	MetadataAnalyticalReproducibilityKey = "analytical_reproducibility_key"
	MetadataAnalyticalProducer           = "analytical_producer"
)

// AnalyticalArtifactDocument stores a validated Analytical Artifact as one
// dataset document: one line per result statement, with the artifact and its
// identity in metadata. A result stays a calculation output. It becomes an
// observation to interpret, never a hypothesis, claim or insight.
func AnalyticalArtifactDocument(projectID string, artifact analytical.Artifact, now time.Time) (*domain.Document, error) {
	candidates, err := analytical.ToCandidates(artifact)
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(candidates))
	for _, c := range candidates {
		lines = append(lines, c.Observation.Quote)
	}
	encoded, err := json.Marshal(artifact)
	if err != nil {
		return nil, fmt.Errorf("encode analytical artifact: %w", err)
	}
	return &domain.Document{
		ID: newID("doc"), ProjectID: projectID, Source: domain.SourceDataset,
		Title:   fmt.Sprintf("Analytical artifact %s (%s %s)", artifact.ID, artifact.Producer, artifact.ProducerVersion),
		Content: strings.Join(lines, "\n"),
		Metadata: map[string]string{
			MetadataAnalyticalArtifact:           string(encoded),
			MetadataAnalyticalArtifactID:         artifact.ID,
			MetadataAnalyticalArtifactHash:       strings.ToLower(artifact.ArtifactHash.Algorithm + ":" + artifact.ArtifactHash.Value),
			MetadataAnalyticalReproducibilityKey: artifact.ReproducibilityKey(),
			MetadataAnalyticalProducer:           artifact.Producer + "@" + artifact.ProducerVersion,
		},
		CreatedAt: now,
	}, nil
}

// ArtifactFromDocument recovers the Analytical Artifact stored on doc.
func ArtifactFromDocument(doc *domain.Document) (*analytical.Artifact, bool) {
	raw, ok := doc.Metadata[MetadataAnalyticalArtifact]
	if !ok {
		return nil, false
	}
	artifact, err := analytical.Import([]byte(raw))
	if err != nil {
		return nil, false
	}
	return artifact, true
}

// MaterializeAnalyticalArtifactObservations turns every result of every
// artifact document into an observation grounded in the document text,
// without a model. A result whose statement cannot be grounded is reported
// in notes rather than guessed.
func MaterializeAnalyticalArtifactObservations(docs []*domain.Document, now time.Time) ([]*domain.Observation, []string) {
	var out []*domain.Observation
	var notes []string
	for _, d := range docs {
		if _, present := d.Metadata[MetadataAnalyticalArtifact]; !present {
			continue
		}
		artifact, ok := ArtifactFromDocument(d)
		if !ok {
			notes = append(notes, fmt.Sprintf("document %s: stored analytical artifact is invalid; skipped", d.ID))
			continue
		}
		candidates, err := analytical.ToCandidates(*artifact)
		if err != nil {
			notes = append(notes, fmt.Sprintf("document %s: %v; skipped", d.ID, err))
			continue
		}
		for _, c := range candidates {
			grounded, ok := Ground(d.Content, c.Observation.Quote)
			if !ok {
				notes = append(notes, fmt.Sprintf("document %s: result %d of artifact %s could not be grounded; skipped", d.ID, c.ResultIndex, artifact.ID))
				continue
			}
			out = append(out, &domain.Observation{
				ID: newID("obs"), DocumentID: d.ID, Quote: grounded.Quote,
				StartOffset: grounded.StartOffset, EndOffset: grounded.EndOffset,
				Behavior: c.Observation.Behavior, Topic: firstNonEmpty(c.Observation.Topic, "analytical_result"),
				CreatedAt: now,
			})
		}
	}
	return out, notes
}
