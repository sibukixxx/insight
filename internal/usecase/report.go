package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"insight-lab/internal/domain"
	"insight-lab/internal/service"
)

// ProjectReport is a portable, human-readable snapshot of the latest
// analysis. Keeping report assembly in the use-case layer lets HTTP remain a
// delivery concern and makes future PDF/export adapters reuse the same data.
type ProjectReport struct {
	Project          *domain.Project
	Documents        map[string]*domain.Document
	Insights         []*InsightDetail
	Metrics          *service.Metrics
	GeneratedAt      time.Time
	ResearchRun      *domain.ResearchRun
	HumanEvaluations map[string]*domain.HumanEvaluation
}

func (a *Application) ExportProjectMarkdown(ctx context.Context, projectID string) ([]byte, error) {
	report, err := a.buildProjectReport(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return renderProjectMarkdown(report), nil
}

func (a *Application) ExportResearchMarkdown(ctx context.Context, runID string) ([]byte, error) {
	run, err := a.repos.Research.GetResearchRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	report, err := a.buildProjectReport(ctx, run.ProjectID)
	if err != nil {
		return nil, err
	}
	report.ResearchRun = run
	referenced := map[string]bool{}
	for _, iteration := range run.Iterations {
		for _, id := range iteration.InsightIDs {
			referenced[id] = true
		}
	}
	filtered := report.Insights[:0]
	for _, detail := range report.Insights {
		if referenced[detail.Insight.ID] {
			filtered = append(filtered, detail)
		}
	}
	report.Insights = filtered
	report.HumanEvaluations = map[string]*domain.HumanEvaluation{}
	for _, iteration := range run.Iterations {
		if evaluation, err := a.repos.Research.GetHumanEvaluation(ctx, run.ID, iteration.ID); err == nil {
			report.HumanEvaluations[iteration.ID] = evaluation
		}
	}
	return renderProjectMarkdown(report), nil
}

func (a *Application) buildProjectReport(ctx context.Context, projectID string) (ProjectReport, error) {
	project, err := a.repos.Projects.Get(ctx, projectID)
	if err != nil {
		return ProjectReport{}, err
	}
	documents, err := a.repos.Documents.ListByProject(ctx, projectID)
	if err != nil {
		return ProjectReport{}, fmt.Errorf("list report documents: %w", err)
	}
	insights, err := a.repos.Insights.ListByProject(ctx, projectID)
	if err != nil {
		return ProjectReport{}, fmt.Errorf("list report insights: %w", err)
	}

	details := make([]*InsightDetail, 0, len(insights))
	for _, insight := range insights {
		detail, err := a.GetInsight(ctx, insight.ID)
		if err != nil {
			return ProjectReport{}, fmt.Errorf("build report insight %s: %w", insight.ID, err)
		}
		details = append(details, detail)
	}

	var metrics *service.Metrics
	if analysis, err := a.repos.Analyses.LatestByProject(ctx, projectID); err == nil && analysis.Metrics != "" {
		var parsed service.Metrics
		if json.Unmarshal([]byte(analysis.Metrics), &parsed) == nil {
			metrics = &parsed
		}
	}
	documentIndex := make(map[string]*domain.Document, len(documents))
	for _, document := range documents {
		documentIndex[document.ID] = document
	}

	return ProjectReport{
		Project: project, Documents: documentIndex, Insights: details,
		Metrics: metrics, GeneratedAt: a.now(),
	}, nil
}

func renderProjectMarkdown(report ProjectReport) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s — Insight Report\n\n", markdownInline(report.Project.Name))
	fmt.Fprintf(&b, "Generated: %s  \n", report.GeneratedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "Insights: %d\n\n", len(report.Insights))

	if report.Metrics != nil {
		m := report.Metrics
		b.WriteString("## Quality summary\n\n")
		fmt.Fprintf(&b, "- Evidence Coverage: %.0f%%\n", m.EvidenceCoverage*100)
		fmt.Fprintf(&b, "- Trace-backed Insights: %.0f%%\n", m.TraceBackedInsightRate*100)
		fmt.Fprintf(&b, "- Quality Flagged: %.0f%%\n", m.QualityFlaggedInsightRate*100)
		fmt.Fprintf(&b, "- Source-verified observations: %d / %d\n\n", m.GroundedObservations, m.TotalObservationCandidates)
		writeRunProvenance(&b, m.Provenance)
	}

	writeResearchLoop(&b, report)

	if len(report.Insights) == 0 {
		b.WriteString("No analyzed insights are available.\n")
		return []byte(b.String())
	}

	writeHypothesisComparisons(&b, report.Insights)

	b.WriteString("## Insights\n\n")
	for index, detail := range report.Insights {
		i := detail.Insight
		fmt.Fprintf(&b, "### %d. %s\n\n", index+1, markdownInline(i.Title))
		fmt.Fprintf(&b, "**Latent need:** %s\n\n", markdownInline(i.LatentNeed))
		fmt.Fprintf(&b, "**Evidence quality score:** %.0f%% (not a probability that a causal claim is true)\n\n", i.Confidence*100)
		writeReportField(&b, "Observation", i.Observation)
		writeReportField(&b, "Stated need", i.StatedNeed)
		writeReportField(&b, "Expected behavior", i.Expectation)
		writeReportField(&b, "Expectation basis", string(i.ExpectationBasis))
		provenance := i.ExpectationBasis.Normalize()
		writeReportField(&b, "Expectation provenance", string(provenance))
		writeReportField(&b, "Observed relative to this data", observationTimingLabel(provenance.ObservationTiming()))
		writeReportField(&b, "Finding kind", string(service.InsightFindingKind(reportStage(report), i.ExpectationBasis)))
		writeReportField(&b, "Surprising deviation", i.SurprisingFact)
		writeReportField(&b, "Rationale", i.Rationale)
		writeReportField(&b, "JTBD", i.JTBD)
		writeReportField(&b, "Product Opportunity", i.ProductOpportunity)
		writeReportField(&b, "Monetization Angle", i.MonetizationAngle)
		writeReportField(&b, "Alternative interpretation", i.AlternativeInterpretation)
		writeInsightSemantics(&b, i)
		writeReportField(&b, "Causal status", string(i.CausalStatus))
		writeReportField(&b, "Identification status", string(i.IdentificationStatus))
		writeReportField(&b, "Validation status", string(i.ValidationStatus))
		writeStringList(&b, "Competing hypotheses", hypothesisLines(i.CompetingHypotheses))
		writeStringList(&b, "Missing evidence", i.MissingEvidence)
		writeStringList(&b, "Falsification criteria (not observed counter-evidence)", i.FalsificationCriteria)
		writeCausalStructure(&b, i.CausalStructure)
		writeStringList(&b, "Next validation — data", i.NextValidation.Data)
		writeStringList(&b, "Next validation — comparisons", i.NextValidation.Comparison)
		writeStringList(&b, "Next validation — candidate designs", i.NextValidation.Design)

		if len(i.QualityFlags) > 0 {
			b.WriteString("**Quality warnings:**\n\n")
			for _, flag := range i.QualityFlags {
				fmt.Fprintf(&b, "- `%s`", flag.Code)
				if flag.Detail != "" {
					fmt.Fprintf(&b, ": %s", markdownInline(flag.Detail))
				}
				b.WriteByte('\n')
			}
			b.WriteByte('\n')
		}

		if len(detail.Evidence) > 0 {
			b.WriteString("**Source-verified evidence:**\n\n")
			for _, evidence := range detail.Evidence {
				label := "support"
				if evidence.Type == domain.EvidenceCounter {
					label = "counter-evidence"
				}
				title := evidence.DocumentID
				if document := report.Documents[evidence.DocumentID]; document != nil && document.Title != "" {
					title = document.Title
				}
				fmt.Fprintf(&b, "- **%s / %s**\n%s\n", label, markdownInline(title), markdownQuote(evidence.Quote))
			}
			b.WriteByte('\n')
		}
	}

	b.WriteString("---\n\nGenerated by Insight Lab. These are AI-generated hypotheses; verify the original evidence before making decisions.\n")
	return []byte(b.String())
}

// writeRunProvenance renders the reproducibility trail from Issue #16: the
// mode (deterministic vs model-backed), rule/model/prompt versions, the
// dataset files a run drew from, and any comparability warnings between
// them. An empty RuleVersion means the analysis predates this field or
// carries no dataset provenance at all, so the whole section is omitted
// rather than printing a misleading empty heading.
func writeRunProvenance(b *strings.Builder, prov service.RunProvenance) {
	if prov.RuleVersion == "" {
		return
	}
	b.WriteString("## Run provenance\n\n")
	fmt.Fprintf(b, "- Mode: `%s`\n", prov.Mode)
	if prov.Model != "" {
		fmt.Fprintf(b, "- Model: %s\n", markdownInline(prov.Model))
	}
	if prov.PromptFingerprint != "" {
		fmt.Fprintf(b, "- Prompt fingerprint: `%s`\n", prov.PromptFingerprint)
	}
	fmt.Fprintf(b, "- Rule version: `%s`\n", prov.RuleVersion)
	writeStringList(b, "Dataset file hashes", prov.DatasetHashes)

	for _, d := range prov.Datasets {
		fmt.Fprintf(b, "- **Dataset:** %s (%s), retrieval: `%s` at %s, schema: `%s`",
			markdownInline(d.SourceName), markdownInline(d.DatasetID), d.RetrievalMethod, d.RetrievedAt.UTC().Format(time.RFC3339), markdownInline(d.SchemaID))
		if d.SchemaVersion != "" {
			fmt.Fprintf(b, " v%s", markdownInline(d.SchemaVersion))
		}
		if d.RecipeRef != "" {
			fmt.Fprintf(b, ", recipe: %s", markdownInline(d.RecipeRef))
		}
		if d.FileHash != "" {
			fmt.Fprintf(b, ", file hash: `%s`", d.FileHash)
		}
		b.WriteByte('\n')
	}
	if len(prov.Datasets) > 0 {
		b.WriteByte('\n')
	}

	if len(prov.CompatibilityWarnings) > 0 {
		b.WriteString("**Dataset compatibility warnings:**\n\n")
		for _, w := range prov.CompatibilityWarnings {
			fmt.Fprintf(b, "- `%s` (%s): %s\n", w.Code, markdownInline(strings.Join(w.DatasetIDs, ", ")), markdownInline(w.Detail))
		}
		b.WriteByte('\n')
	}
	writeStringList(b, "Pre-analysis notes", prov.Notes)
}

// reportStage is the stage findings in this report must be labelled against.
// A report with no attached research run has never left plain exploratory
// analysis, so it defaults to EXPLORATORY rather than leaving stage unknown.
func reportStage(report ProjectReport) domain.ResearchStage {
	if report.ResearchRun != nil {
		if stage := report.ResearchRun.CurrentStage(); stage != "" {
			return stage
		}
	}
	return domain.StageExploratory
}

func observationTimingLabel(timing domain.ObservationTiming) string {
	switch timing {
	case domain.TimingPreObservation:
		return "created before observing this dataset"
	case domain.TimingPostObservation:
		return "created after observing this dataset"
	default:
		return "unknown"
	}
}

func writeResearchLoop(b *strings.Builder, report ProjectReport) {
	if report.ResearchRun == nil {
		return
	}
	run := report.ResearchRun
	b.WriteString("## Research Question\n\n")
	fmt.Fprintf(b, "%s\n\n", markdownInline(run.Question))
	b.WriteString("## Research Stage\n\n")
	fmt.Fprintf(b, "**Current stage:** `%s`\n\n", run.CurrentStage())
	b.WriteString("## Inputs / Provenance\n\n")
	if len(run.Iterations) > 0 {
		writeStringList(b, "Input references", run.Iterations[len(run.Iterations)-1].InputReferences)
	}
	b.WriteString("## Observations / Expectations / Surprises\n\n")
	b.WriteString("See the source-grounded Insight details below; iteration references preserve which results were used.\n\n")
	b.WriteString("## Competing Hypotheses / Evidence / Counter Evidence\n\n")
	b.WriteString("The comparison table and source-verified evidence below remain authoritative. Falsification criteria are not counter-evidence.\n\n")
	b.WriteString("## Research Gaps\n\n")
	if len(run.Iterations) > 0 {
		for _, gap := range run.Iterations[len(run.Iterations)-1].ResearchGaps {
			fmt.Fprintf(b, "- `%s` %s — %s (resolved: %t)\n", gap.Category, markdownInline(gap.Need), markdownInline(gap.WhyItMatters), gap.Resolved)
		}
		b.WriteByte('\n')
	}
	b.WriteString("## Next Data Requirements\n\n")
	if len(run.Iterations) > 0 {
		for _, req := range run.Iterations[len(run.Iterations)-1].DataRequirements {
			fmt.Fprintf(b, "- **Need:** %s; **Reason:** %s; **Dimensions:** %s; **Period:** %s; **Source category:** %s\n", markdownInline(req.Need), markdownInline(req.Reason), markdownInline(strings.Join(req.RequiredDimensions, " × ")), markdownInline(req.RequiredPeriod), markdownInline(req.SuggestedSourceCategory))
		}
		b.WriteByte('\n')
	}
	b.WriteString("## Validation / Identification\n\n")
	b.WriteString("Statuses are shown per hypothesis below. History descriptors do not represent causal probability.\n\n")
	if len(run.Iterations) > 0 {
		latest := run.Iterations[len(run.Iterations)-1]
		if len(latest.ValidationEvidence) > 0 {
			b.WriteString("**Independent validation evidence provenance:**\n\n")
			for _, v := range latest.ValidationEvidence {
				fmt.Fprintf(b, "- expectation=`%s` evidence=%s sourceIteration=`%s` dataset=%s rationale=%s actor=`%s`\n",
					markdownInline(v.ExpectationID), markdownInline(v.EvidenceReference), markdownInline(v.SourceIterationID),
					markdownInline(v.DatasetReference), markdownInline(v.IndependenceRationale), markdownInline(v.Actor))
			}
			b.WriteByte('\n')
		}
	}
	writeDecisionReadiness(b, run)
	writePromotionStatus(b, run)
	b.WriteString("## What We Cannot Conclude\n\n")
	if len(run.Iterations) > 0 {
		for _, statement := range run.Iterations[len(run.Iterations)-1].WhatWeCannotConclude {
			fmt.Fprintf(b, "- %s\n", markdownInline(statement))
		}
		b.WriteByte('\n')
	}
	b.WriteString("## Iteration History\n\n")
	for _, iteration := range run.Iterations {
		fmt.Fprintf(b, "### Iteration %d\n\n- ID: `%s`\n- Added evidence: %s\n", iteration.Sequence, markdownInline(iteration.ID), markdownInline(strings.Join(iteration.AddedEvidence, ", ")))
		for _, link := range iteration.AddedEvidenceLinks {
			fmt.Fprintf(b, "- Evidence linkage: %s → gaps [%s]", markdownInline(link.Reference), markdownInline(strings.Join(link.GapIDs, ", ")))
			if link.Note != "" {
				fmt.Fprintf(b, " — %s", markdownInline(link.Note))
			}
			b.WriteByte('\n')
		}
		for _, change := range iteration.HypothesisChanges {
			fmt.Fprintf(b, "- `%s`: **%s** — %s\n", markdownInline(change.HypothesisID), change.Evolution, markdownInline(change.Reason))
		}
		b.WriteByte('\n')
	}
	if latest, ok := run.LatestIteration(); ok && latest.Delta != nil {
		b.WriteString("## What Changed Since Previous Iteration\n\n")
		writeStringList(b, "Input variables added", latest.Delta.Input.Variables.Added)
		writeStringList(b, "Input variables removed", latest.Delta.Input.Variables.Removed)
		writeStringList(b, "Evidence added", latest.Delta.Input.EvidenceReferences.Added)
		writeStringList(b, "Evidence removed", latest.Delta.Input.EvidenceReferences.Removed)
		writeStringList(b, "Insights added", latest.Delta.Result.InsightIDsAdded)
		writeStringList(b, "Insights removed", latest.Delta.Result.InsightIDsRemoved)
		writeStringList(b, "Gaps created", latest.Delta.Result.ResearchGapIDsCreated)
		writeStringList(b, "Gaps resolved", latest.Delta.Result.ResearchGapIDsResolved)
		writeStringList(b, "Delta interpretation notes", latest.Delta.Explanation)
	}
	b.WriteString("## Human Evaluation\n\n")
	if len(report.HumanEvaluations) == 0 {
		b.WriteString("No human evaluation has been recorded. The LLM cannot assign novelty.\n\n")
		return
	}
	for _, iteration := range run.Iterations {
		if evaluation := report.HumanEvaluations[iteration.ID]; evaluation != nil {
			fmt.Fprintf(b, "- Iteration %d: novelty `%s`, overall usefulness %d/5. %s\n", iteration.Sequence, evaluation.Novelty, evaluation.OverallUsefulness, markdownInline(evaluation.Notes))
		}
	}
	b.WriteByte('\n')
}

// writeDecisionReadiness reports how prepared the research process is for a
// responsible human decision, and — only when the loop has actually stopped
// — why it stopped and what remained unresolved at that point. It is not a
// truth probability, causal certainty, or commercial recommendation.
func writeDecisionReadiness(b *strings.Builder, run *domain.ResearchRun) {
	latest, ok := run.LatestIteration()
	if !ok {
		return
	}
	b.WriteString("## Decision Readiness\n\n")
	fmt.Fprintf(b, "**State:** `%s` (effective: `%s`)\n\n", latest.Readiness.State, latest.EffectiveReadiness())
	writeStringList(b, "Reasons", latest.Readiness.Reasons)
	writeStringList(b, "Unresolved gap IDs", latest.Readiness.UnresolvedGapIDs)

	if len(latest.HumanOverrides) > 0 {
		b.WriteString("## Human Overrides\n\n")
		for _, override := range latest.HumanOverrides {
			fmt.Fprintf(b, "- readiness=`%s` stopReason=`%s`: %s\n", override.Readiness, override.StopReason, markdownInline(override.Note))
		}
		b.WriteByte('\n')
	}

	if latest.Stop == nil {
		return
	}
	b.WriteString("## Stop Decision\n\n")
	fmt.Fprintf(b, "**Reason:** `%s` (source: `%s`)\n\n", latest.Stop.Reason, latest.Stop.Source)
	if latest.Stop.Note != "" {
		fmt.Fprintf(b, "%s\n\n", markdownInline(latest.Stop.Note))
	}
	writeStringList(b, "Unresolved at stop", latest.Stop.UnresolvedGapIDs)
}

// writePromotionStatus reports how far a research run's latest iteration
// has been cleared to move toward a public report (issue #24): its current
// promotion state, the stated contribution, why further promotion is
// blocked (if it is), and which publication checklist items remain unmet.
// Research validity and publication attractiveness are never conflated:
// this section only ever reflects what the gate actually checked.
func writePromotionStatus(b *strings.Builder, run *domain.ResearchRun) {
	latest, ok := run.LatestIteration()
	if !ok {
		return
	}
	b.WriteString("## Promotion Status\n\n")
	fmt.Fprintf(b, "**State:** `%s`\n\n", run.CurrentPromotionState())
	if latest.PromotionGateInput.Contribution != "" {
		fmt.Fprintf(b, "**Contribution:** `%s`\n\n", latest.PromotionGateInput.Contribution)
	}
	writeStringList(b, "Blocking reasons", latest.Promotion.Reasons)
	writeStringList(b, "Unmet publication checklist items", latest.PromotionGateInput.Checklist.UnmetItems())
}

func writeHypothesisComparisons(b *strings.Builder, details []*InsightDetail) {
	groups := map[string][]*InsightDetail{}
	var order []string
	for _, detail := range details {
		id := detail.Insight.HypothesisSetID
		if id == "" {
			continue
		}
		if _, ok := groups[id]; !ok {
			order = append(order, id)
		}
		groups[id] = append(groups[id], detail)
	}
	for _, id := range order {
		group := groups[id]
		if len(group) < 2 {
			continue
		}
		b.WriteString("## Competing hypothesis comparison\n\n")
		writeReportField(b, "Surprising fact", group[0].Insight.SurprisingFact)
		b.WriteString("| Role | Hypothesis | Validation | Identification | Support | Counter | Missing |\n")
		b.WriteString("|---|---|---|---|---:|---:|---:|\n")
		for _, detail := range group {
			support, counter := 0, 0
			for _, evidence := range detail.Evidence {
				if evidence.Type == domain.EvidenceCounter {
					counter++
				} else if evidence.Type == domain.EvidenceSupport {
					support++
				}
			}
			i := detail.Insight
			fmt.Fprintf(b, "| %s | %s | %s | %s | %d | %d | %d |\n",
				markdownTableCell(string(i.HypothesisRole)), markdownTableCell(i.Title),
				markdownTableCell(string(i.ValidationStatus)), markdownTableCell(string(i.IdentificationStatus)),
				support, counter, len(i.MissingEvidence))
		}
		b.WriteByte('\n')
	}
}

func markdownTableCell(value string) string {
	return strings.ReplaceAll(strings.Join(strings.Fields(value), " "), "|", "\\|")
}

func hypothesisLines(values []domain.CompetingHypothesis) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, v.Title+": "+v.Explanation)
	}
	return out
}

func writeStringList(b *strings.Builder, label string, values []string) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(b, "**%s:**\n\n", label)
	for _, value := range values {
		fmt.Fprintf(b, "- %s\n", markdownInline(value))
	}
	b.WriteByte('\n')
}


func writeInsightSemantics(b *strings.Builder, i *domain.Insight) {
	if i.Connection.Statement != "" || len(i.Connection.Sources) > 0 || len(i.Connection.Targets) > 0 {
		b.WriteString("**Non-obvious connection:**\n\n")
		if i.Connection.Kind != "" {
			fmt.Fprintf(b, "- Kind: `%s` (descriptive, not causal status)\n", i.Connection.Kind)
		}
		if i.Connection.Statement != "" {
			fmt.Fprintf(b, "- Statement: %s\n", markdownInline(i.Connection.Statement))
		}
		if i.Connection.WhyItMatters != "" {
			fmt.Fprintf(b, "- Why it matters: %s\n", markdownInline(i.Connection.WhyItMatters))
		}
		for _, ref := range i.Connection.Sources {
			fmt.Fprintf(b, "- Source: `%s` %s %s\n", ref.Kind, markdownInline(ref.ID), markdownInline(ref.Label))
		}
		for _, ref := range i.Connection.Targets {
			fmt.Fprintf(b, "- Target: `%s` %s %s\n", ref.Kind, markdownInline(ref.ID), markdownInline(ref.Label))
		}
		b.WriteByte('\n')
	}
	if i.Mechanism.Statement != "" || len(i.Mechanism.Steps) > 0 {
		b.WriteString("**Candidate mechanism (proposed, not validated by narrative coherence):**\n\n")
		writeReportField(b, "Mechanism", i.Mechanism.Statement)
		for idx, step := range i.Mechanism.Steps {
			fmt.Fprintf(b, "- Step %d: %s\n", idx+1, markdownInline(step.Statement))
			writeStringList(b, "  Evidence refs", step.EvidenceRefs)
			writeStringList(b, "  Bridge assumptions", step.Assumptions)
			writeStringList(b, "  Missing evidence", step.MissingEvidence)
		}
		writeStringList(b, "Alternative mechanisms", i.Mechanism.AlternativeMechanisms)
		writeStringList(b, "Mechanism counter-evidence refs", i.Mechanism.CounterEvidenceRefs)
		writeStringList(b, "Mechanism falsification criteria", i.Mechanism.FalsificationCriteria)
	}
	if i.Generalization.Principle != "" || i.Generalization.Status != "" {
		b.WriteString("**Generalization / transfer candidate:**\n\n")
		writeReportField(b, "Source context", i.Generalization.SourceContext)
		writeReportField(b, "Transferable principle", i.Generalization.Principle)
		writeReportField(b, "Target context", i.Generalization.TargetContext)
		writeReportField(b, "Generalization status", string(i.Generalization.Status))
		writeStringList(b, "Applicability conditions", i.Generalization.ApplicabilityConditions)
		writeStringList(b, "Boundary conditions", i.Generalization.BoundaryConditions)
		writeStringList(b, "Known failure conditions", i.Generalization.KnownFailureConditions)
	}
}

func writeCausalStructure(b *strings.Builder, structure domain.CandidateCausalStructure) {
	if len(structure.Variables) == 0 && len(structure.Relations) == 0 {
		return
	}
	b.WriteString("**Candidate causal structure (proposed, not proven):**\n\n")
	for _, variable := range structure.Variables {
		fmt.Fprintf(b, "- Variable `%s`: %s (%s, %s)\n", markdownInline(variable.ID), markdownInline(variable.Name), variable.Role, variable.Status)
	}
	for _, relation := range structure.Relations {
		fmt.Fprintf(b, "- Relation `%s → %s` (%s)\n", markdownInline(relation.From), markdownInline(relation.To), relation.Status)
	}
	b.WriteByte('\n')
}

func writeReportField(b *strings.Builder, label, value string) {
	if strings.TrimSpace(value) != "" {
		fmt.Fprintf(b, "**%s:** %s\n\n", label, markdownInline(value))
	}
}

func markdownInline(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	return strings.NewReplacer("\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]").Replace(value)
}

func markdownQuote(value string) string {
	lines := strings.Split(strings.TrimSpace(value), "\n")
	for i := range lines {
		lines[i] = "> " + lines[i]
	}
	return strings.Join(lines, "\n")
}
