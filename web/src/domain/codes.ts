// Display-label keys for stable server codes. The codes themselves are
// defined by the Go core; this file only maps them to locale dictionary
// keys. An unknown code is shown verbatim (see labelKey), never guessed.

import type { MessageKey } from "./messages";

export type LabelTable = Readonly<Record<string, MessageKey>>;

export const SOURCE_LABELS: LabelTable = {
  document: "source.document",
  report: "source.report",
  paper: "source.paper",
  web: "source.web",
  record: "source.record",
  other: "source.other",
  dataset: "source.dataset",
  interview: "source.interview",
  review: "source.review",
  support: "source.support",
  sales: "source.sales",
  survey: "source.survey",
  job_posting: "source.jobPosting",
  social_post: "source.socialPost",
};

export const STEP_LABELS: LabelTable = {
  starting: "step.starting",
  extracting_observations: "step.extractingObservations",
  detecting_traces: "step.detectingTraces",
  detecting_patterns: "step.detectingPatterns",
  exploring_question: "step.exploringQuestion",
  generating_hypotheses: "step.generatingHypotheses",
  searching_evidence: "step.searchingEvidence",
  deduplicating_insights: "step.deduplicatingInsights",
  scoring_confidence: "step.scoringConfidence",
  completed: "step.completed",
};

export const RUN_STATUS_LABELS: LabelTable = {
  queued: "runStatus.queued",
  running: "runStatus.running",
  completed: "runStatus.completed",
  failed: "runStatus.failed",
};

export const MODE_LABELS: LabelTable = {
  deterministic: "mode.deterministic",
  model_backed: "mode.modelBacked",
};

export const REASONING_PROFILE_LABELS: LabelTable = {
  GENERAL_RESEARCH: "profile.generalResearch",
  CUSTOMER_INSIGHT: "profile.customerInsight",
};

/** Output locales the server accepts for model-generated text (#125); "" = server default. */
export const OUTPUT_LOCALE_LABELS: LabelTable = {
  "": "outputLocale.default",
  "ja-JP": "outputLocale.jaJP",
  "en-US": "outputLocale.enUS",
};

// Two inspectable finding kinds: a mismatch against an expectation/baseline,
// and a repeated regularity across grounded observations.
export const DEVIATION_LABELS: LabelTable = {
  contradiction: "deviation.contradiction",
  excess_effort: "deviation.excessEffort",
  excess_payment: "deviation.excessPayment",
  persistence: "deviation.persistence",
  absence: "deviation.absence",
  other: "deviation.other",
};

// App-side quality warnings computed deterministically after the model has
// spoken; hints for the researcher, not verdicts.
export const QUALITY_FLAG_LABELS: Readonly<Record<string, { readonly label: MessageKey; readonly desc: MessageKey }>> = {
  stated_need_echo: { label: "quality.statedNeedEcho.label", desc: "quality.statedNeedEcho.desc" },
  generic_term: { label: "quality.genericTerm.label", desc: "quality.genericTerm.desc" },
  no_trace: { label: "quality.noTrace.label", desc: "quality.noTrace.desc" },
  abduction_incomplete: { label: "quality.abductionIncomplete.label", desc: "quality.abductionIncomplete.desc" },
  insufficient_competing_hypotheses: { label: "quality.insufficientCompeting.label", desc: "quality.insufficientCompeting.desc" },
};

export const PROMOTION_STATE_LABELS: LabelTable = {
  DRAFT: "promotionState.draft",
  RESEARCH_COMPLETE: "promotionState.researchComplete",
  HUMAN_REVIEW_REQUIRED: "promotionState.humanReviewRequired",
  PUBLICATION_READY: "promotionState.publicationReady",
  PUBLISHED: "promotionState.published",
  REJECTED_FOR_PUBLICATION: "promotionState.rejected",
};

export const TERMINAL_PROMOTION_STATES: readonly string[] = ["PUBLISHED", "REJECTED_FOR_PUBLICATION"];

export const CONTRIBUTION_LABELS: LabelTable = {
  CORRECTION: "contribution.correction",
  REFINEMENT: "contribution.refinement",
  REPLICATION: "contribution.replication",
  DEFINITION_AUDIT: "contribution.definitionAudit",
  COUNTER_EVIDENCE: "contribution.counterEvidence",
  INCONCLUSIVE_BUT_DECISION_RELEVANT: "contribution.inconclusiveButDecisionRelevant",
  NOVEL_MISMATCH: "contribution.novelMismatch",
  OTHER: "contribution.other",
};

export const PROMOTION_JUDGMENTS = [
  ["competingHypothesisConsidered", "promotion.judgment.competingHypothesisConsidered"],
  ["independentValidationStatusAccurate", "promotion.judgment.independentValidationStatusAccurate"],
  ["decisionReadinessHonestlyStated", "promotion.judgment.decisionReadinessHonestlyStated"],
  ["makesStrongClaim", "promotion.judgment.makesStrongClaim"],
  ["hasUnresolvedCriticalGap", "promotion.judgment.hasUnresolvedCriticalGap"],
  ["humanReviewCompleted", "promotion.judgment.humanReviewCompleted"],
] as const satisfies readonly (readonly [string, MessageKey])[];

/** Run comparison axis states and attribution labels (#83); bookkeeping, never causal. */
export const AXIS_STATE_LABELS: LabelTable = {
  SAME: "compare.axis.same",
  CHANGED: "compare.axis.changed",
  UNKNOWN: "compare.axis.unknown",
};

export const ATTRIBUTION_LABELS: LabelTable = {
  SAME_CONFIGURATION: "compare.attribution.sameConfiguration",
  EXECUTION_CHANGE: "compare.attribution.executionChange",
  INPUT_CHANGE: "compare.attribution.inputChange",
  CONFOUNDED: "compare.attribution.confounded",
  ATTRIBUTION_UNAVAILABLE: "compare.attribution.unavailable",
};

export const IMPORT_KIND_LABELS: LabelTable = {
  documents: "input.format.documents.title",
  analysis: "input.format.analysis.title",
};

/** The label key for code, or undefined when the code is not known here. */
export function labelKey(table: LabelTable, code: string | undefined): MessageKey | undefined {
  return code !== undefined && Object.prototype.hasOwnProperty.call(table, code) ? table[code] : undefined;
}

export const SAMPLE_DATA_KIND_LABELS: LabelTable = {
  synthetic: "samples.kind.synthetic",
  official: "samples.kind.official",
  mixed: "samples.kind.mixed",
};

export const SAMPLE_SOURCE_KIND_LABELS: LabelTable = {
  synthetic: "samples.sourceKind.synthetic",
  official: "samples.sourceKind.official",
};

/** Pre-analysis copy of each bundled scenario: what it is for, not what it found. */
export interface SampleCopy {
  readonly title: MessageKey;
  readonly question: MessageKey;
  readonly learn: MessageKey;
  readonly caution: MessageKey;
}

export const SAMPLE_COPY: Readonly<Record<string, SampleCopy>> = {
  "ja-shop-records": { title: "samples.shop.title", question: "samples.shop.question", learn: "samples.shop.learn", caution: "samples.shop.caution" },
  "ja-official-population": { title: "samples.population.title", question: "samples.population.question", learn: "samples.population.learn", caution: "samples.population.caution" },
  "ja-population-establishments": { title: "samples.mixed.title", question: "samples.mixed.question", learn: "samples.mixed.learn", caution: "samples.mixed.caution" },
};
