package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/llm"
	"insight-lab/internal/repository"
)

// ProgressFunc is invoked at each pipeline milestone so the caller (the
// job manager) can update the analyses row and broadcast an SSE event.
type ProgressFunc func(step string, progress int, message string)

type Pipeline struct {
	Documents    repository.DocumentRepository
	Observations repository.ObservationRepository
	Patterns     repository.PatternRepository
	Insights     repository.InsightRepository
	Evidence     repository.EvidenceRepository
	LLM          llm.Client
}

// Metrics is the evaluation summary computed at the end of a run and
// persisted as analyses.metrics (see docs/detailed-design.md §15). Every
// value here comes from counting grounded/discarded/linked records, never
// from asking the model to self-report quality.
type Metrics struct {
	TotalObservationCandidates int     `json:"totalObservationCandidates"`
	GroundedObservations       int     `json:"groundedObservations"`
	UnsupportedClaimRate       float64 `json:"unsupportedClaimRate"`
	PatternCount               int     `json:"patternCount"`
	// TraceCount is how many deviation-from-expectation patterns (traces
	// of desire) were found. PatternCount includes them.
	TraceCount                int     `json:"traceCount"`
	TotalInsightDrafts        int     `json:"totalInsightDrafts"`
	FinalInsightCount         int     `json:"finalInsightCount"`
	InsightDuplicationRate    float64 `json:"insightDuplicationRate"`
	EvidenceCoverage          float64 `json:"evidenceCoverage"`
	CounterEvidenceCoverage   float64 `json:"counterEvidenceCoverage"`
	AverageEvidencePerInsight float64 `json:"averageEvidencePerInsight"`
	// TraceBackedInsightRate is the share of final insights whose
	// hypothesis cites at least one deviation pattern - i.e. insights
	// anchored to a surprising fact rather than to repetition alone.
	TraceBackedInsightRate float64 `json:"traceBackedInsightRate"`
	// QualityFlaggedInsightRate is the share of final insights carrying
	// at least one app-side quality warning (see quality.go).
	QualityFlaggedInsightRate float64 `json:"qualityFlaggedInsightRate"`
	// QualityFlagCounts is how many insights carry each flag code.
	QualityFlagCounts map[string]int `json:"qualityFlagCounts"`
}

type draftInsight struct {
	hypothesis      hypothesisCandidate
	writeup         insightWriteup
	supporting      []*domain.Observation
	counter         []*domain.Observation
	counterSearched bool
}

func (p *Pipeline) Run(ctx context.Context, analysisID, projectID string, progress ProgressFunc) (*Metrics, error) {
	if progress == nil {
		progress = func(string, int, string) {}
	}

	docs, err := p.Documents.ListByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("the project has no documents")
	}

	metrics := &Metrics{}

	progress("extracting_observations", 5, "Reading documents...")
	allObs, err := p.extractAndGroundAll(ctx, docs, metrics)
	if err != nil {
		return nil, fmt.Errorf("observation extraction: %w", err)
	}
	if len(allObs) == 0 {
		return nil, fmt.Errorf("no observations could be verified against the source text")
	}
	if err := p.Observations.CreateBatch(ctx, allObs); err != nil {
		return nil, fmt.Errorf("save observations: %w", err)
	}
	progress("extracting_observations", 25, fmt.Sprintf("Verified %d observations (%d unverified observations discarded)",
		len(allObs), metrics.TotalObservationCandidates-metrics.GroundedObservations))

	obsByID := indexObservations(allObs)

	// Step 1 of the method: predict how a person "should" behave, then
	// treat behavior that breaks the prediction as the trace an unconscious
	// desire left behind. This runs before repetition detection because an
	// insight anchored to a surprising fact is much less likely to be a
	// restatement of what customers already say.
	progress("detecting_traces", 28, "Looking for deviations from expected behavior...")
	traceCandidates, err := p.detectTraces(ctx, allObs)
	if err != nil {
		return nil, fmt.Errorf("trace detection: %w", err)
	}
	traces := buildTracePatterns(projectID, analysisID, traceCandidates, obsByID)
	progress("detecting_traces", 33, fmt.Sprintf("Found %d behavioral deviations", len(traces)))

	progress("detecting_patterns", 35, "Looking for recurring patterns...")
	patternCandidates, err := p.detectPatterns(ctx, allObs)
	if err != nil {
		return nil, fmt.Errorf("pattern detection: %w", err)
	}
	repetitions := buildPatterns(projectID, analysisID, patternCandidates, obsByID)
	patterns := append(append([]*domain.Pattern{}, traces...), repetitions...)
	if err := p.Patterns.CreateBatch(ctx, patterns); err != nil {
		return nil, fmt.Errorf("save patterns: %w", err)
	}
	metrics.PatternCount = len(patterns)
	metrics.TraceCount = len(traces)
	progress("detecting_patterns", 40, fmt.Sprintf("Found %d recurring patterns", len(repetitions)))

	progress("generating_hypotheses", 45, "Generating hidden-need hypotheses...")
	hypotheses, err := p.generateHypotheses(ctx, patterns, allObs)
	if err != nil {
		return nil, fmt.Errorf("hypothesis generation: %w", err)
	}

	patternsByID := indexPatterns(patterns)
	docByID := indexDocuments(docs)

	progress("searching_evidence", 55, "Searching for evidence and counter-evidence...")
	drafts := make([]draftInsight, 0, len(hypotheses))
	for i, h := range hypotheses {
		evOut, err := p.retrieveEvidence(ctx, h, allObs)
		if err != nil {
			return nil, fmt.Errorf("evidence retrieval (%s): %w", h.Title, err)
		}
		supporting := resolveObservations(evOut.SupportingObservationIDs, obsByID)
		counter := resolveObservations(evOut.CounterObservationIDs, obsByID)

		writeup, err := p.writeupInsight(ctx, h, supporting, counter)
		if err != nil {
			return nil, fmt.Errorf("insight writeup (%s): %w", h.Title, err)
		}

		drafts = append(drafts, draftInsight{
			hypothesis: h, writeup: *writeup,
			supporting: supporting, counter: counter, counterSearched: evOut.CounterSearched,
		})
		progress("searching_evidence", 55+int(20*float64(i+1)/float64(len(hypotheses))),
			fmt.Sprintf("Verified evidence for %q", h.Title))
	}
	metrics.TotalInsightDrafts = len(drafts)

	progress("deduplicating_insights", 80, "Merging duplicate insights...")
	keepIdx, mergedAway, err := p.dedupeDrafts(ctx, drafts)
	if err != nil {
		return nil, fmt.Errorf("dedupe: %w", err)
	}
	if len(drafts) > 0 {
		metrics.InsightDuplicationRate = float64(mergedAway) / float64(len(drafts))
	}

	progress("scoring_confidence", 90, "Calculating confidence and saving insights...")
	if err := p.persistInsights(ctx, analysisID, projectID, drafts, keepIdx, docByID, patternsByID, len(docs), metrics); err != nil {
		return nil, err
	}

	progress("completed", 100, fmt.Sprintf("Found %d insights", metrics.FinalInsightCount))
	return metrics, nil
}

func (p *Pipeline) extractAndGroundAll(ctx context.Context, docs []*domain.Document, metrics *Metrics) ([]*domain.Observation, error) {
	var allObs []*domain.Observation
	for _, d := range docs {
		for _, chunk := range Chunk(d.Content) {
			out, err := p.extractObservations(ctx, chunk)
			if err != nil {
				return nil, fmt.Errorf("document %s: %w", d.ID, err)
			}
			for _, cand := range out.Observations {
				metrics.TotalObservationCandidates++
				grounded, ok := Ground(d.Content, cand.Quote)
				if !ok {
					continue
				}
				metrics.GroundedObservations++
				allObs = append(allObs, &domain.Observation{
					ID: newID("obs"), DocumentID: d.ID, Quote: grounded.Quote,
					StartOffset: grounded.StartOffset, EndOffset: grounded.EndOffset,
					Behavior: cand.Behavior, Topic: cand.Topic, CreatedAt: time.Now().UTC(),
				})
			}
		}
	}
	if metrics.TotalObservationCandidates > 0 {
		metrics.UnsupportedClaimRate = 1 - float64(metrics.GroundedObservations)/float64(metrics.TotalObservationCandidates)
	}
	return allObs, nil
}

func (p *Pipeline) persistInsights(ctx context.Context, analysisID, projectID string, drafts []draftInsight, keepIdx []int,
	docByID map[string]*domain.Document, patternsByID map[string]*domain.Pattern, totalDocuments int, metrics *Metrics) error {

	var insightsWithSupport, counterSearchedCount, totalEvidenceRows, traceBacked, flagged int
	flagCounts := map[string]int{}
	aID := analysisID

	for _, idx := range keepIdx {
		d := drafts[idx]
		insight := &domain.Insight{
			ID: newID("ins"), ProjectID: projectID, AnalysisID: &aID, Title: d.writeup.Title,
			Observation: d.writeup.ObservationSummary, StatedNeed: d.hypothesis.StatedNeed,
			LatentNeed: d.hypothesis.LatentNeed, JTBD: d.hypothesis.JTBD,
			Expectation: d.hypothesis.Expectation, SurprisingFact: d.hypothesis.SurprisingFact, Rationale: d.hypothesis.Rationale,
			Interpretation: d.writeup.Interpretation, AlternativeInterpretation: d.writeup.AlternativeInterpretation,
			ProductOpportunity: d.writeup.ProductOpportunity, MonetizationAngle: d.writeup.MonetizationAngle,
			CreatedAt: time.Now().UTC(),
		}

		supportRows := buildEvidenceRows(insight.ID, d.supporting, domain.EvidenceSupport)
		counterRows := buildEvidenceRows(insight.ID, d.counter, domain.EvidenceCounter)
		evidenceRows := append(supportRows, counterRows...)

		documentsWithSupport := map[string]struct{}{}
		var sourceTypes []domain.SourceType
		for _, o := range d.supporting {
			documentsWithSupport[o.DocumentID] = struct{}{}
			if doc := docByID[o.DocumentID]; doc != nil {
				sourceTypes = append(sourceTypes, doc.Source)
			}
		}

		insight.Confidence = Confidence(ConfidenceInput{
			SupportingEvidence:    supportRows,
			CounterEvidence:       counterRows,
			SupportingSourceTypes: sourceTypes,
			DocumentsWithSupport:  len(documentsWithSupport),
			TotalDocuments:        totalDocuments,
			PatternDocumentCount:  len(documentsWithSupport),
		})

		// Pattern references are validated the same way quotes are
		// grounded: a cited pattern that doesn't exist is dropped, never
		// stored. The surviving patterns also feed the quality gate (an
		// insight built from repetition alone, with no trace, is flagged).
		var validPatternIDs []string
		var citedPatterns []*domain.Pattern
		for _, pid := range d.hypothesis.BasedOnPatternIDs {
			if pat, ok := patternsByID[pid]; ok {
				validPatternIDs = append(validPatternIDs, pid)
				citedPatterns = append(citedPatterns, pat)
			}
		}

		insight.QualityFlags = AssessQuality(QualityInput{
			StatedNeed: insight.StatedNeed, LatentNeed: insight.LatentNeed,
			Expectation: insight.Expectation, SurprisingFact: insight.SurprisingFact,
			Patterns: citedPatterns,
		})

		if err := p.Insights.Create(ctx, insight); err != nil {
			return fmt.Errorf("save insight: %w", err)
		}
		if err := p.Evidence.CreateBatch(ctx, evidenceRows); err != nil {
			return fmt.Errorf("save evidence: %w", err)
		}
		if err := p.Patterns.LinkInsight(ctx, insight.ID, validPatternIDs); err != nil {
			return fmt.Errorf("link insight patterns: %w", err)
		}

		if !insight.HasQualityFlag(domain.QualityNoTrace) {
			traceBacked++
		}
		if len(insight.QualityFlags) > 0 {
			flagged++
		}
		for _, f := range insight.QualityFlags {
			flagCounts[string(f.Code)]++
		}

		if len(d.supporting) > 0 {
			insightsWithSupport++
		}
		if d.counterSearched {
			counterSearchedCount++
		}
		totalEvidenceRows += len(evidenceRows)
	}

	metrics.FinalInsightCount = len(keepIdx)
	metrics.QualityFlagCounts = flagCounts
	if metrics.FinalInsightCount > 0 {
		n := float64(metrics.FinalInsightCount)
		metrics.EvidenceCoverage = float64(insightsWithSupport) / n
		metrics.CounterEvidenceCoverage = float64(counterSearchedCount) / n
		metrics.AverageEvidencePerInsight = float64(totalEvidenceRows) / n
		metrics.TraceBackedInsightRate = float64(traceBacked) / n
		metrics.QualityFlaggedInsightRate = float64(flagged) / n
	}
	return nil
}

// --- LLM step calls ---

func (p *Pipeline) extractObservations(ctx context.Context, chunk string) (*observationExtractionOutput, error) {
	resp, err := p.LLM.Generate(ctx, llm.GenerateRequest{
		SystemPrompt: observationExtractionPrompt,
		Messages:     []llm.Message{{Role: "user", Content: chunk}},
		Schema:       observationExtractionSchema(),
		Temperature:  0.2,
	})
	if err != nil {
		return nil, err
	}
	var out observationExtractionOutput
	if err := json.Unmarshal(resp.Content, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Pipeline) detectTraces(ctx context.Context, obs []*domain.Observation) ([]traceCandidate, error) {
	payload, err := json.Marshal(map[string]any{"observations": toObservationRefs(obs)})
	if err != nil {
		return nil, err
	}
	resp, err := p.LLM.Generate(ctx, llm.GenerateRequest{
		SystemPrompt: traceDetectionPrompt,
		Messages:     []llm.Message{{Role: "user", Content: string(payload)}},
		Schema:       traceDetectionSchema(),
		Temperature:  0.3,
	})
	if err != nil {
		return nil, err
	}
	var out traceDetectionOutput
	if err := json.Unmarshal(resp.Content, &out); err != nil {
		return nil, err
	}
	return out.Traces, nil
}

func (p *Pipeline) detectPatterns(ctx context.Context, obs []*domain.Observation) ([]patternCandidate, error) {
	payload, err := json.Marshal(map[string]any{"observations": toObservationRefs(obs)})
	if err != nil {
		return nil, err
	}
	resp, err := p.LLM.Generate(ctx, llm.GenerateRequest{
		SystemPrompt: patternDetectionPrompt,
		Messages:     []llm.Message{{Role: "user", Content: string(payload)}},
		Schema:       patternDetectionSchema(),
		Temperature:  0.3,
	})
	if err != nil {
		return nil, err
	}
	var out patternDetectionOutput
	if err := json.Unmarshal(resp.Content, &out); err != nil {
		return nil, err
	}
	return out.Patterns, nil
}

func (p *Pipeline) generateHypotheses(ctx context.Context, patterns []*domain.Pattern, obs []*domain.Observation) ([]hypothesisCandidate, error) {
	payload, err := json.Marshal(map[string]any{"patterns": toPatternRefs(patterns), "observations": toObservationRefs(obs)})
	if err != nil {
		return nil, err
	}
	resp, err := p.LLM.Generate(ctx, llm.GenerateRequest{
		SystemPrompt: hypothesisPrompt,
		Messages:     []llm.Message{{Role: "user", Content: string(payload)}},
		Schema:       hypothesisSchema(),
		Temperature:  0.4,
	})
	if err != nil {
		return nil, err
	}
	var out hypothesisOutput
	if err := json.Unmarshal(resp.Content, &out); err != nil {
		return nil, err
	}
	return out.Hypotheses, nil
}

func (p *Pipeline) retrieveEvidence(ctx context.Context, h hypothesisCandidate, obs []*domain.Observation) (*evidenceRetrievalOutput, error) {
	payload, err := json.Marshal(map[string]any{
		"hypothesis": map[string]any{
			"title": h.Title, "statedNeed": h.StatedNeed, "latentNeed": h.LatentNeed,
			"jtbd": h.JTBD, "rationale": h.Rationale,
		},
		"observations": toObservationRefs(obs),
	})
	if err != nil {
		return nil, err
	}
	resp, err := p.LLM.Generate(ctx, llm.GenerateRequest{
		SystemPrompt: evidenceRetrievalPrompt,
		Messages:     []llm.Message{{Role: "user", Content: string(payload)}},
		Schema:       evidenceRetrievalSchema(),
		Temperature:  0.2,
	})
	if err != nil {
		return nil, err
	}
	var out evidenceRetrievalOutput
	if err := json.Unmarshal(resp.Content, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Pipeline) writeupInsight(ctx context.Context, h hypothesisCandidate, supporting, counter []*domain.Observation) (*insightWriteup, error) {
	payload, err := json.Marshal(map[string]any{
		"hypothesis":             h,
		"supportingObservations": toObservationRefs(supporting),
		"counterObservations":    toObservationRefs(counter),
	})
	if err != nil {
		return nil, err
	}
	resp, err := p.LLM.Generate(ctx, llm.GenerateRequest{
		SystemPrompt: insightWriteupPrompt,
		Messages:     []llm.Message{{Role: "user", Content: string(payload)}},
		Schema:       insightWriteupSchema(),
		Temperature:  0.4,
	})
	if err != nil {
		return nil, err
	}
	var out insightWriteup
	if err := json.Unmarshal(resp.Content, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Pipeline) dedupeDrafts(ctx context.Context, drafts []draftInsight) (keep []int, mergedAway int, err error) {
	if len(drafts) <= 1 {
		for i := range drafts {
			keep = append(keep, i)
		}
		return keep, 0, nil
	}

	type item struct {
		Index      int    `json:"index"`
		Title      string `json:"title"`
		LatentNeed string `json:"latentNeed"`
	}
	items := make([]item, len(drafts))
	for i, d := range drafts {
		items[i] = item{Index: i, Title: d.writeup.Title, LatentNeed: d.hypothesis.LatentNeed}
	}
	payload, err := json.Marshal(map[string]any{"insights": items})
	if err != nil {
		return nil, 0, err
	}

	resp, err := p.LLM.Generate(ctx, llm.GenerateRequest{
		SystemPrompt: dedupePrompt,
		Messages:     []llm.Message{{Role: "user", Content: string(payload)}},
		Schema:       dedupeSchema(),
		Temperature:  0.1,
	})
	if err != nil {
		return nil, 0, err
	}
	var out dedupeOutput
	if err := json.Unmarshal(resp.Content, &out); err != nil {
		return nil, 0, err
	}

	merged := map[int]bool{}
	for _, group := range out.DuplicateGroups {
		if len(group) < 2 {
			continue
		}
		sorted := append([]int(nil), group...)
		sort.Ints(sorted)
		for _, idx := range sorted[1:] {
			if idx >= 0 && idx < len(drafts) && !merged[idx] {
				merged[idx] = true
				mergedAway++
			}
		}
	}
	for i := range drafts {
		if !merged[i] {
			keep = append(keep, i)
		}
	}
	return keep, mergedAway, nil
}

// --- helpers ---

func indexObservations(obs []*domain.Observation) map[string]*domain.Observation {
	m := make(map[string]*domain.Observation, len(obs))
	for _, o := range obs {
		m[o.ID] = o
	}
	return m
}

func indexDocuments(docs []*domain.Document) map[string]*domain.Document {
	m := make(map[string]*domain.Document, len(docs))
	for _, d := range docs {
		m[d.ID] = d
	}
	return m
}

func indexPatterns(patterns []*domain.Pattern) map[string]*domain.Pattern {
	m := make(map[string]*domain.Pattern, len(patterns))
	for _, p := range patterns {
		m[p.ID] = p
	}
	return m
}

// buildPatterns turns the LLM's raw pattern candidates into persistable
// Patterns, dropping any that cite no observation that was actually
// grounded. A "pattern" built entirely from fabricated observation IDs
// isn't a pattern - it's exactly the kind of invisible, unverifiable claim
// this pipeline exists to prevent.
func buildPatterns(projectID, analysisID string, candidates []patternCandidate, obsByID map[string]*domain.Observation) []*domain.Pattern {
	now := time.Now().UTC()
	var out []*domain.Pattern
	for _, c := range candidates {
		var validIDs []string
		for _, id := range c.ObservationIDs {
			if _, ok := obsByID[id]; ok {
				validIDs = append(validIDs, id)
			}
		}
		if len(validIDs) == 0 {
			continue
		}
		out = append(out, &domain.Pattern{
			ID: newID("pat"), ProjectID: projectID, AnalysisID: analysisID, Kind: domain.PatternRepetition,
			Title: c.Title, Description: c.Description, ObservationIDs: validIDs, CreatedAt: now,
		})
	}
	return out
}

// buildTracePatterns applies the same existence filter to traces: a
// "surprising fact" that cites no grounded observation is discarded. The
// trace's expectation and actual behavior are both kept so a reader can
// judge for themselves whether the gap is real. An out-of-vocabulary
// deviation type is coerced to "other" rather than rejected - the
// classification is a label for the UI, not evidence.
func buildTracePatterns(projectID, analysisID string, candidates []traceCandidate, obsByID map[string]*domain.Observation) []*domain.Pattern {
	now := time.Now().UTC()
	var out []*domain.Pattern
	for _, c := range candidates {
		var validIDs []string
		for _, id := range c.ObservationIDs {
			if _, ok := obsByID[id]; ok {
				validIDs = append(validIDs, id)
			}
		}
		if len(validIDs) == 0 {
			continue
		}
		dt := domain.DeviationType(c.DeviationType)
		if !dt.Valid() {
			dt = domain.DeviationOther
		}
		out = append(out, &domain.Pattern{
			ID: newID("pat"), ProjectID: projectID, AnalysisID: analysisID, Kind: domain.PatternDeviation,
			Title: c.Title, Description: c.ActualBehavior, Expectation: c.Expectation, DeviationType: dt,
			ObservationIDs: validIDs, CreatedAt: now,
		})
	}
	return out
}

func toPatternRefs(patterns []*domain.Pattern) []patternRef {
	refs := make([]patternRef, len(patterns))
	for i, p := range patterns {
		refs[i] = patternRef{
			ID: p.ID, Kind: string(p.Kind), Title: p.Title, Description: p.Description,
			Expectation: p.Expectation, DeviationType: string(p.DeviationType), ObservationCount: len(p.ObservationIDs),
		}
	}
	return refs
}

func resolveObservations(ids []string, index map[string]*domain.Observation) []*domain.Observation {
	seen := make(map[string]bool, len(ids))
	var out []*domain.Observation
	for _, id := range ids {
		if seen[id] {
			continue
		}
		if o, ok := index[id]; ok {
			out = append(out, o)
			seen[id] = true
		}
	}
	return out
}

func toObservationRefs(obs []*domain.Observation) []observationRef {
	refs := make([]observationRef, len(obs))
	for i, o := range obs {
		refs[i] = observationRef{ID: o.ID, Quote: o.Quote, Behavior: o.Behavior, Topic: o.Topic}
	}
	return refs
}

// relevanceForIndex approximates evidence relevance from the order the
// model listed observations in (first = most relevant), since the schema
// doesn't ask for a numeric score per item - a number the model invents
// would be no more trustworthy than one it invents for the whole insight
// (see docs/design-review.md P0-2 on not letting the model self-report
// confidence).
func relevanceForIndex(i int) float64 {
	v := 1.0 - float64(i)*0.05
	if v < 0.5 {
		v = 0.5
	}
	return v
}

func buildEvidenceRows(insightID string, obs []*domain.Observation, evidenceType domain.EvidenceType) []*domain.Evidence {
	rows := make([]*domain.Evidence, 0, len(obs))
	for i, o := range obs {
		oid := o.ID
		rows = append(rows, &domain.Evidence{
			ID: newID("ev"), InsightID: insightID, DocumentID: o.DocumentID, ObservationID: &oid,
			Quote: o.Quote, Type: evidenceType, RelevanceScore: relevanceForIndex(i),
			StartOffset: o.StartOffset, EndOffset: o.EndOffset,
		})
	}
	return rows
}
