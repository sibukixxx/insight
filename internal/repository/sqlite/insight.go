package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"insight-lab/internal/domain"
	"insight-lab/internal/repository"
)

type InsightRepository struct{ db *DB }

func NewInsightRepository(db *DB) *InsightRepository {
	return &InsightRepository{db: db}
}

const insightColumns = `id, project_id, analysis_id, title, observation, stated_need, latent_need, jtbd,
	 expectation, surprising_fact, rationale, interpretation, alternative_interpretation,
	 product_opportunity, monetization_angle, confidence, quality_flags, expectation_basis,
	 causal_status, validation_status, identification_status, causal_context,
	 hypothesis_set_id, hypothesis_role, created_at`

type causalContext struct {
	CompetingHypotheses   []domain.CompetingHypothesis    `json:"competingHypotheses"`
	CausalStructure       domain.CandidateCausalStructure `json:"causalStructure"`
	MissingEvidence       []string                        `json:"missingEvidence"`
	FalsificationCriteria []string                        `json:"falsificationCriteria"`
	NextValidation        domain.ValidationNeed           `json:"nextValidation"`
	Connection            domain.InsightConnection        `json:"connection,omitempty"`
	Mechanism             domain.MechanismCandidate       `json:"mechanism,omitempty"`
	Generalization        domain.InsightGeneralization    `json:"generalization,omitempty"`
}

func (r *InsightRepository) Create(ctx context.Context, insight *domain.Insight) error {
	flags, err := encodeQualityFlags(insight.QualityFlags)
	if err != nil {
		return err
	}
	contextJSON, err := json.Marshal(causalContext{
		CompetingHypotheses:   insight.CompetingHypotheses,
		CausalStructure:       insight.CausalStructure,
		MissingEvidence:       insight.MissingEvidence,
		FalsificationCriteria: insight.FalsificationCriteria,
		NextValidation:        insight.NextValidation,
		Connection:            insight.Connection,
		Mechanism:             insight.Mechanism,
		Generalization:        insight.Generalization,
	})
	if err != nil {
		return fmt.Errorf("encode causal context: %w", err)
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO insights (`+insightColumns+`)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		insight.ID, insight.ProjectID, nullableString(insight.AnalysisID), insight.Title, insight.Observation,
		insight.StatedNeed, insight.LatentNeed, insight.JTBD,
		nullableStringLiteral(insight.Expectation), nullableStringLiteral(insight.SurprisingFact), insight.Rationale,
		insight.Interpretation, insight.AlternativeInterpretation, insight.ProductOpportunity, insight.MonetizationAngle,
		insight.Confidence, flags, insight.ExpectationBasis, insight.CausalStatus, insight.ValidationStatus,
		insight.IdentificationStatus, string(contextJSON), nullableStringLiteral(insight.HypothesisSetID),
		insight.HypothesisRole, formatTime(insight.CreatedAt))
	return err
}

func (r *InsightRepository) Get(ctx context.Context, id string) (*domain.Insight, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+insightColumns+` FROM insights WHERE id = ?`, id)
	return scanInsight(row)
}

func (r *InsightRepository) ListByProject(ctx context.Context, projectID string) ([]*domain.Insight, error) {
	return r.list(ctx, `SELECT `+insightColumns+` FROM insights WHERE project_id = ? ORDER BY confidence DESC`, projectID)
}

func (r *InsightRepository) ListByAnalysis(ctx context.Context, analysisID string) ([]*domain.Insight, error) {
	return r.list(ctx, `SELECT `+insightColumns+` FROM insights WHERE analysis_id = ? ORDER BY confidence DESC`, analysisID)
}

func (r *InsightRepository) list(ctx context.Context, query string, arg string) ([]*domain.Insight, error) {
	rows, err := r.db.QueryContext(ctx, query, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Insight
	for rows.Next() {
		i, err := scanInsight(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func nullableString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// encodeQualityFlags stores flags as a JSON array; an insight with no
// flags is stored as NULL rather than "[]" so a plain SQL query can tell
// the two apart.
func encodeQualityFlags(flags []domain.QualityFlag) (any, error) {
	if len(flags) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(flags)
	if err != nil {
		return nil, fmt.Errorf("encode quality flags: %w", err)
	}
	return string(b), nil
}

func scanInsight(s scanner) (*domain.Insight, error) {
	var i domain.Insight
	var createdAt string
	var analysisID, expectation, surprisingFact, rationale, monetizationAngle, qualityFlags, causalContextJSON, hypothesisSetID sql.NullString
	if err := s.Scan(&i.ID, &i.ProjectID, &analysisID, &i.Title, &i.Observation, &i.StatedNeed, &i.LatentNeed,
		&i.JTBD, &expectation, &surprisingFact, &rationale, &i.Interpretation, &i.AlternativeInterpretation,
		&i.ProductOpportunity, &monetizationAngle, &i.Confidence, &qualityFlags, &i.ExpectationBasis,
		&i.CausalStatus, &i.ValidationStatus, &i.IdentificationStatus, &causalContextJSON,
		&hypothesisSetID, &i.HypothesisRole, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	if analysisID.Valid {
		v := analysisID.String
		i.AnalysisID = &v
	}
	i.Expectation = expectation.String
	i.SurprisingFact = surprisingFact.String
	i.Rationale = rationale.String
	i.MonetizationAngle = monetizationAngle.String
	i.HypothesisSetID = hypothesisSetID.String
	if qualityFlags.Valid && qualityFlags.String != "" {
		if err := json.Unmarshal([]byte(qualityFlags.String), &i.QualityFlags); err != nil {
			return nil, fmt.Errorf("decode quality flags for %s: %w", i.ID, err)
		}
	}
	if causalContextJSON.Valid && causalContextJSON.String != "" {
		var value causalContext
		if err := json.Unmarshal([]byte(causalContextJSON.String), &value); err != nil {
			return nil, fmt.Errorf("decode causal context for %s: %w", i.ID, err)
		}
		i.CompetingHypotheses, i.CausalStructure, i.MissingEvidence = value.CompetingHypotheses, value.CausalStructure, value.MissingEvidence
		i.FalsificationCriteria, i.NextValidation = value.FalsificationCriteria, value.NextValidation
		i.Connection, i.Mechanism, i.Generalization = value.Connection, value.Mechanism, value.Generalization
	}
	t, err := parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	i.CreatedAt = t
	return &i, nil
}
