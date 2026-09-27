// Read models for the Reference Web, derived from the Go HTTP DTOs
// (internal/http/handler) and the Public Engine / analytical contracts.
// They carry what the server recorded, nothing inferred. A value the
// server omitted is `undefined`, never coerced to "" / 0 / false.

export interface BuildInfo {
  readonly demoBuild: boolean;
  readonly clientName: string;
}

export interface Project {
  readonly id: string;
  readonly name: string;
  readonly createdAt: string;
}

export interface EvidenceDocument {
  readonly id: string;
  readonly projectId: string;
  /** Raw source code (see SOURCE_LABELS); unknown codes stay visible. */
  readonly source: string;
  readonly title: string;
  readonly content: string;
  readonly metadata: Readonly<Record<string, string>>;
  readonly createdAt: string;
}

export type RunStatus = "queued" | "running" | "completed" | "failed" | (string & {});

export interface ExecutionSnapshot {
  readonly engineVersion?: string;
  readonly gitCommit?: string;
  readonly gitDirty?: string;
}

export interface RunProvenance {
  readonly mode?: string;
  readonly model?: string;
  readonly promptFingerprint?: string;
  readonly ruleVersion?: string;
}

export interface RunMetrics {
  readonly totalObservationCandidates?: number;
  readonly groundedObservations?: number;
  readonly unsupportedClaimRate?: number;
  readonly patternCount?: number;
  readonly traceCount?: number;
  readonly totalInsightDrafts?: number;
  readonly finalInsightCount?: number;
  readonly insightDuplicationRate?: number;
  readonly evidenceCoverage?: number;
  readonly counterEvidenceCoverage?: number;
  readonly averageEvidencePerInsight?: number;
  readonly traceBackedInsightRate?: number;
  readonly qualityFlaggedInsightRate?: number;
  readonly qualityFlagCounts?: Readonly<Record<string, number>>;
  readonly provenance?: RunProvenance;
}

export interface AnalysisRun {
  readonly id: string;
  readonly projectId: string;
  readonly status: RunStatus;
  readonly currentStep?: string;
  readonly progress: number;
  readonly error?: string;
  readonly startedAt?: string;
  readonly finishedAt?: string;
  readonly createdAt: string;
  readonly metrics?: RunMetrics;
  readonly label?: string;
  readonly researchQuestion?: string;
  readonly reasoningProfile?: string;
  readonly outputLocale?: string;
  readonly executionSnapshot?: ExecutionSnapshot;
  readonly executionFingerprint?: string;
  readonly inputFingerprint?: string;
}

export interface QualityFlag {
  readonly code: string;
  readonly detail?: string;
}

export interface Insight {
  readonly id: string;
  readonly projectId: string;
  readonly analysisId?: string;
  readonly title: string;
  readonly observation: string;
  readonly statedNeed: string;
  readonly latentNeed: string;
  readonly hypothesis: string;
  readonly jtbd: string;
  readonly expectation: string;
  readonly surprisingFact: string;
  readonly rationale: string;
  readonly interpretation: string;
  readonly alternativeInterpretation: string;
  readonly productOpportunity: string;
  readonly monetizationAngle: string;
  readonly confidence: number;
  readonly qualityFlags: readonly QualityFlag[];
  readonly createdAt: string;
}

/** A quoted span of a source document; shared by evidence and pattern observations. */
export interface QuoteSpan {
  readonly id: string;
  readonly documentId: string;
  readonly quote: string;
  readonly startOffset: number;
  readonly endOffset: number;
}

export interface Evidence extends QuoteSpan {
  /** "support" or "counter"; other codes are kept as-is. */
  readonly type: string;
  readonly relevanceScore: number;
}

export interface PatternObservation extends QuoteSpan {
  readonly behavior: string;
  readonly topic?: string;
}

export interface Pattern {
  readonly id: string;
  readonly analysisId?: string;
  /** "deviation" (expectation mismatch) or "repetition". */
  readonly kind: string;
  readonly title: string;
  readonly description?: string;
  readonly expectation?: string;
  readonly deviationType?: string;
  readonly observations: readonly PatternObservation[];
}

export interface InsightDetail extends Insight {
  readonly evidence: readonly Evidence[];
  readonly patterns: readonly Pattern[];
}

export interface LlmSettings {
  readonly model: string;
  readonly baseUrl: string;
  readonly maskedApiKey: string;
  readonly hasApiKey: boolean;
  readonly configured: boolean;
}

export interface ImportColumn {
  readonly name: string;
  readonly required: boolean;
}

export type ImportKind = "documents" | "analysis" | (string & {});

/** A file format the server's import endpoints accept (GET /api/import-formats). */
export interface ImportFormat {
  readonly kind: ImportKind;
  readonly extensions: readonly string[];
  readonly mediaTypes: readonly string[];
  readonly encoding: string;
  readonly columns: readonly ImportColumn[];
  readonly sourceTypes: readonly string[];
}

export interface ImportRowError {
  readonly row: number;
  readonly reason: string;
}

/** Data Triage dataset profile of one column (internal/triage). */
export interface ColumnProfile {
  readonly name: string;
  readonly type: string;
  readonly nonNullCount: number;
  readonly nullCount: number;
  readonly distinctCount: number;
  readonly distinctCapped: boolean;
  readonly min?: string;
  readonly max?: string;
  readonly sampleValues: readonly string[];
}

export interface DatasetProfile {
  readonly contentSha256: string;
  readonly rowCount: number;
  readonly columns: readonly ColumnProfile[];
  readonly profilerVersion: string;
}

/** Dry-run result of an import, computed by the server's own importer. */
export interface ImportPreview {
  readonly kind: ImportKind;
  readonly recordsRead: number;
  readonly importable: number;
  readonly skipped: number;
  readonly errors: readonly ImportRowError[];
  readonly fileHash: string;
  readonly documents: readonly EvidenceDocument[];
  readonly totalDocuments: number;
  readonly profile?: DatasetProfile;
  readonly profileError?: string;
}

export interface DocumentImportResult {
  readonly imported: number;
  readonly skipped: number;
  /** Only the analysis CSV reports the records it read. */
  readonly recordsRead?: number;
  readonly errors: readonly ImportRowError[];
}

export type EvaluationMetrics = RunMetrics;

export interface ResearchRunSummary {
  readonly id: string;
  readonly question: string;
}

export interface PromotionDecision {
  readonly state?: string;
  readonly reasons: readonly string[];
}

export interface ResearchIteration {
  readonly id: string;
  readonly sequence: number;
  readonly promotion?: PromotionDecision;
  readonly checklist: Readonly<Record<string, boolean>>;
  readonly approvedArtifactReference?: string;
}

export interface ResearchRun {
  readonly id: string;
  readonly projectId: string;
  readonly question: string;
  readonly iterations: readonly ResearchIteration[];
}

export interface PromotionReview {
  readonly contribution: string;
  readonly competingHypothesisConsidered: boolean;
  readonly independentValidationStatusAccurate: boolean;
  readonly decisionReadinessHonestlyStated: boolean;
  readonly makesStrongClaim: boolean;
  readonly hasUnresolvedCriticalGap: boolean;
  readonly humanReviewCompleted: boolean;
}

export interface FieldChange {
  readonly field: string;
  readonly from: string;
  readonly to: string;
}

export interface MetricDelta {
  readonly metric: string;
  /** null when the run did not record the metric. */
  readonly from: number | null;
  readonly to: number | null;
  readonly delta: number | null;
}

export interface RunComparison {
  readonly fromId: string;
  readonly toId: string;
  /** SAME | CHANGED | UNKNOWN */
  readonly inputState: string;
  readonly executionState: string;
  readonly executionChanges: readonly FieldChange[];
  readonly researchQuestionChange?: FieldChange;
  readonly documentsAdded: readonly string[];
  readonly documentsRemoved: readonly string[];
  readonly attribution: string;
  readonly metrics: readonly MetricDelta[];
  readonly insightsAdded: readonly string[];
  readonly insightsRemoved: readonly string[];
  readonly insightsMatched: number;
  readonly explanation: readonly string[];
}

/** Progress pushed by the server while a run executes (SSE). */
export type AnalysisEvent =
  | { readonly type: "progress"; readonly step: string; readonly progress: number; readonly message?: string }
  | { readonly type: "completed" }
  | { readonly type: "failed"; readonly message?: string };
