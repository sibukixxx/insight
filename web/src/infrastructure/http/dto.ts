// Decoders for the Go HTTP DTOs (internal/http/handler/*.go).

import type {
  AnalysisRun, BuildInfo, ColumnProfile, DatasetProfile, DocumentImportResult, Evidence, EvidenceDocument,
  ExecutionSnapshot, ImportFormat, ImportPreview, ImportRowError, Insight, InsightDetail, LlmSettings, Pattern,
  PatternObservation, Project, QualityFlag, ResearchIteration, ResearchRun, ResearchRunSummary, RunComparison,
  RunMetrics, RunProvenance, SampleScenario, SampleSource,
} from "../../domain/models";
import {
  arr, bool, boolRecord, compact, num, numberRecord, numOrNull, obj, optArr, optNum, optStr, str, stringItem,
  stringRecord, strOrEmpty,
} from "./decode";

export function decodeBuildInfo(v: unknown): BuildInfo {
  const o = obj(v, "health");
  return { demoBuild: o.demoBuild === true, clientName: typeof o.clientName === "string" ? o.clientName : "" };
}

export function decodeProject(v: unknown, path = "project"): Project {
  const o = obj(v, path);
  return { id: str(o, "id", path), name: str(o, "name", path), createdAt: strOrEmpty(o, "createdAt", path) };
}

export function decodeDocument(v: unknown, path = "document"): EvidenceDocument {
  const o = obj(v, path);
  return {
    id: str(o, "id", path), projectId: strOrEmpty(o, "projectId", path), source: str(o, "source", path),
    title: strOrEmpty(o, "title", path), content: strOrEmpty(o, "content", path),
    metadata: stringRecord(o.metadata, `${path}.metadata`), createdAt: strOrEmpty(o, "createdAt", path),
  };
}

function decodeProvenance(v: unknown, path: string): RunProvenance | undefined {
  if (v === undefined || v === null) return undefined;
  const o = obj(v, path);
  return compact({
    mode: optStr(o, "mode", path), model: optStr(o, "model", path),
    promptFingerprint: optStr(o, "promptFingerprint", path), ruleVersion: optStr(o, "ruleVersion", path),
  });
}

export function decodeMetrics(v: unknown, path = "metrics"): RunMetrics {
  const o = obj(v, path);
  return compact({
    totalObservationCandidates: optNum(o, "totalObservationCandidates", path),
    groundedObservations: optNum(o, "groundedObservations", path),
    unsupportedClaimRate: optNum(o, "unsupportedClaimRate", path),
    patternCount: optNum(o, "patternCount", path),
    traceCount: optNum(o, "traceCount", path),
    totalInsightDrafts: optNum(o, "totalInsightDrafts", path),
    finalInsightCount: optNum(o, "finalInsightCount", path),
    insightDuplicationRate: optNum(o, "insightDuplicationRate", path),
    evidenceCoverage: optNum(o, "evidenceCoverage", path),
    counterEvidenceCoverage: optNum(o, "counterEvidenceCoverage", path),
    averageEvidencePerInsight: optNum(o, "averageEvidencePerInsight", path),
    traceBackedInsightRate: optNum(o, "traceBackedInsightRate", path),
    qualityFlaggedInsightRate: optNum(o, "qualityFlaggedInsightRate", path),
    qualityFlagCounts: numberRecord(o.qualityFlagCounts, `${path}.qualityFlagCounts`),
    provenance: decodeProvenance(o.provenance, `${path}.provenance`),
  });
}

function decodeExecutionSnapshot(v: unknown, path: string): ExecutionSnapshot | undefined {
  if (v === undefined || v === null) return undefined;
  const o = obj(v, path);
  return compact({ engineVersion: optStr(o, "engineVersion", path), gitCommit: optStr(o, "gitCommit", path), gitDirty: optStr(o, "gitDirty", path) });
}

export function decodeRun(v: unknown, path = "analysis"): AnalysisRun {
  const o = obj(v, path);
  return compact({
    id: str(o, "id", path), projectId: strOrEmpty(o, "projectId", path), status: str(o, "status", path),
    currentStep: optStr(o, "currentStep", path), progress: optNum(o, "progress", path) ?? 0, error: optStr(o, "error", path),
    startedAt: optStr(o, "startedAt", path), finishedAt: optStr(o, "finishedAt", path), createdAt: strOrEmpty(o, "createdAt", path),
    metrics: o.metrics === undefined || o.metrics === null ? undefined : decodeMetrics(o.metrics, `${path}.metrics`),
    label: optStr(o, "label", path), researchQuestion: optStr(o, "researchQuestion", path),
    reasoningProfile: optStr(o, "reasoningProfile", path), outputLocale: optStr(o, "outputLocale", path),
    executionSnapshot: decodeExecutionSnapshot(o.executionSnapshot, `${path}.executionSnapshot`),
    executionFingerprint: optStr(o, "executionFingerprint", path), inputFingerprint: optStr(o, "inputFingerprint", path),
  });
}

function decodeQualityFlag(v: unknown, path: string): QualityFlag {
  const o = obj(v, path);
  return compact({ code: str(o, "code", path), detail: optStr(o, "detail", path) });
}

export function decodeInsight(v: unknown, path = "insight"): Insight {
  const o = obj(v, path);
  const s = (key: string) => strOrEmpty(o, key, path);
  return compact({
    id: str(o, "id", path), projectId: s("projectId"), analysisId: optStr(o, "analysisId", path), title: s("title"),
    observation: s("observation"), statedNeed: s("statedNeed"), latentNeed: s("latentNeed"), hypothesis: s("hypothesis"),
    jtbd: s("jtbd"), expectation: s("expectation"), surprisingFact: s("surprisingFact"), rationale: s("rationale"),
    interpretation: s("interpretation"), alternativeInterpretation: s("alternativeInterpretation"),
    productOpportunity: s("productOpportunity"), monetizationAngle: s("monetizationAngle"),
    confidence: optNum(o, "confidence", path) ?? 0,
    qualityFlags: optArr(o, "qualityFlags", path, decodeQualityFlag), createdAt: s("createdAt"),
  });
}

function decodeEvidence(v: unknown, path: string): Evidence {
  const o = obj(v, path);
  return {
    id: str(o, "id", path), documentId: str(o, "documentId", path), quote: strOrEmpty(o, "quote", path),
    type: str(o, "type", path), relevanceScore: optNum(o, "relevanceScore", path) ?? 0,
    startOffset: num(o, "startOffset", path), endOffset: num(o, "endOffset", path),
  };
}

function decodeObservation(v: unknown, path: string): PatternObservation {
  const o = obj(v, path);
  return compact({
    id: str(o, "id", path), documentId: str(o, "documentId", path), quote: strOrEmpty(o, "quote", path),
    behavior: strOrEmpty(o, "behavior", path), topic: optStr(o, "topic", path),
    startOffset: num(o, "startOffset", path), endOffset: num(o, "endOffset", path),
  });
}

export function decodePattern(v: unknown, path = "pattern"): Pattern {
  const o = obj(v, path);
  return compact({
    id: str(o, "id", path), analysisId: optStr(o, "analysisId", path), kind: str(o, "kind", path), title: strOrEmpty(o, "title", path),
    description: optStr(o, "description", path), expectation: optStr(o, "expectation", path),
    deviationType: optStr(o, "deviationType", path), observations: optArr(o, "observations", path, decodeObservation),
  });
}

export function decodeInsightDetail(v: unknown): InsightDetail {
  const o = obj(v, "insight");
  return {
    ...decodeInsight(v),
    evidence: optArr(o, "evidence", "insight", decodeEvidence),
    patterns: optArr(o, "patterns", "insight", decodePattern),
  };
}

export function decodeSettings(v: unknown): LlmSettings {
  const o = obj(v, "settings");
  return {
    model: strOrEmpty(o, "model", "settings"), baseUrl: strOrEmpty(o, "baseUrl", "settings"),
    maskedApiKey: strOrEmpty(o, "maskedApiKey", "settings"), hasApiKey: bool(o, "hasApiKey", "settings"),
    configured: bool(o, "configured", "settings"),
  };
}

export function decodeImportFormat(v: unknown, path: string): ImportFormat {
  const o = obj(v, path);
  return {
    kind: str(o, "kind", path),
    extensions: optArr(o, "extensions", path, stringItem),
    mediaTypes: optArr(o, "mediaTypes", path, stringItem),
    encoding: strOrEmpty(o, "encoding", path),
    columns: optArr(o, "columns", path, (c, p) => {
      const co = obj(c, p);
      return { name: str(co, "name", p), required: co.required === true };
    }),
    sourceTypes: optArr(o, "sourceTypes", path, stringItem),
  };
}

function decodeRowError(v: unknown, path: string): ImportRowError {
  const o = obj(v, path);
  return { row: num(o, "row", path), reason: strOrEmpty(o, "reason", path) };
}

function decodeColumnProfile(v: unknown, path: string): ColumnProfile {
  const o = obj(v, path);
  return compact({
    name: str(o, "name", path), type: str(o, "type", path), nonNullCount: num(o, "nonNullCount", path),
    nullCount: num(o, "nullCount", path), distinctCount: num(o, "distinctCount", path),
    distinctCapped: o.distinctCapped === true, min: optStr(o, "min", path), max: optStr(o, "max", path),
    sampleValues: optArr(o, "sampleValues", path, stringItem),
  });
}

function decodeProfile(v: unknown, path: string): DatasetProfile | undefined {
  if (v === undefined || v === null) return undefined;
  const o = obj(v, path);
  return {
    contentSha256: strOrEmpty(o, "contentSha256", path), rowCount: num(o, "rowCount", path),
    columns: optArr(o, "columns", path, decodeColumnProfile), profilerVersion: strOrEmpty(o, "profilerVersion", path),
  };
}

export function decodeImportPreview(v: unknown): ImportPreview {
  const path = "preview";
  const o = obj(v, path);
  return compact({
    kind: str(o, "kind", path), recordsRead: num(o, "recordsRead", path), importable: num(o, "importable", path),
    skipped: num(o, "skipped", path), errors: optArr(o, "errors", path, decodeRowError), fileHash: strOrEmpty(o, "fileHash", path),
    documents: optArr(o, "documents", path, decodeDocument), totalDocuments: num(o, "totalDocuments", path),
    profile: decodeProfile(o.profile, `${path}.profile`), profileError: optStr(o, "profileError", path),
  });
}

export function decodeImportResult(v: unknown): DocumentImportResult {
  const path = "import";
  const o = obj(v, path);
  return compact({
    imported: num(o, "imported", path), skipped: num(o, "skipped", path),
    recordsRead: optNum(o, "recordsRead", path), errors: optArr(o, "errors", path, decodeRowError),
  });
}

export function decodeResearchSummary(v: unknown, path = "researchRun"): ResearchRunSummary {
  const o = obj(v, path);
  return { id: str(o, "id", path), question: strOrEmpty(o, "question", path) };
}

function decodeIteration(v: unknown, path: string): ResearchIteration {
  const o = obj(v, path);
  const promotion = o.promotion === undefined || o.promotion === null ? undefined : obj(o.promotion, `${path}.promotion`);
  const gate = o.promotionGateInput === undefined || o.promotionGateInput === null ? undefined : obj(o.promotionGateInput, `${path}.promotionGateInput`);
  const approved = o.approvedArtifact === undefined || o.approvedArtifact === null ? undefined : obj(o.approvedArtifact, `${path}.approvedArtifact`);
  return compact({
    id: str(o, "id", path), sequence: num(o, "sequence", path),
    promotion: promotion && compact({
      state: optStr(promotion, "state", `${path}.promotion`),
      reasons: optArr(promotion, "reasons", `${path}.promotion`, stringItem),
    }),
    checklist: boolRecord(gate?.Checklist, `${path}.promotionGateInput.Checklist`),
    approvedArtifactReference: approved ? strOrEmpty(approved, "reference", `${path}.approvedArtifact`) : undefined,
  });
}

export function decodeResearchRun(v: unknown): ResearchRun {
  const path = "researchRun";
  const o = obj(v, path);
  return {
    id: str(o, "id", path), projectId: str(o, "projectId", path), question: strOrEmpty(o, "question", path),
    iterations: optArr(o, "iterations", path, decodeIteration),
  };
}

function decodeFieldChange(v: unknown, path: string) {
  const o = obj(v, path);
  return { field: str(o, "field", path), from: strOrEmpty(o, "from", path), to: strOrEmpty(o, "to", path) };
}

export function decodeComparison(v: unknown): RunComparison {
  const path = "comparison";
  const o = obj(v, path);
  const input = obj(o.input, `${path}.input`);
  const execution = obj(o.execution, `${path}.execution`);
  const insights = obj(o.insights, `${path}.insights`);
  return compact({
    fromId: str(obj(o.from, `${path}.from`), "analysisId", `${path}.from`),
    toId: str(obj(o.to, `${path}.to`), "analysisId", `${path}.to`),
    inputState: str(input, "state", `${path}.input`),
    executionState: str(execution, "state", `${path}.execution`),
    executionChanges: optArr(execution, "changes", `${path}.execution`, decodeFieldChange),
    researchQuestionChange: input.researchQuestion === undefined || input.researchQuestion === null
      ? undefined : decodeFieldChange(input.researchQuestion, `${path}.input.researchQuestion`),
    documentsAdded: optArr(input, "documentsAdded", `${path}.input`, stringItem),
    documentsRemoved: optArr(input, "documentsRemoved", `${path}.input`, stringItem),
    attribution: str(o, "attribution", path),
    metrics: optArr(o, "metrics", path, (m, p) => {
      const mo = obj(m, p);
      return { metric: str(mo, "metric", p), from: numOrNull(mo, "from", p), to: numOrNull(mo, "to", p), delta: numOrNull(mo, "delta", p) };
    }),
    insightsAdded: optArr(insights, "added", `${path}.insights`, stringItem),
    insightsRemoved: optArr(insights, "removed", `${path}.insights`, stringItem),
    insightsMatched: optArr(insights, "matched", `${path}.insights`, (x) => x).length,
    explanation: optArr(o, "explanation", path, stringItem),
  });
}

export const list = <T>(decode: (v: unknown, path: string) => T, path: string) => (v: unknown): T[] => arr(v, path, decode);

function decodeSampleSource(v: unknown, path: string): SampleSource {
  const o = obj(v, path);
  return {
    kind: str(o, "kind", path),
    description: optStr(o, "description", path),
    rows: optArr(o, "rows", path, stringItem),
    publisher: optStr(o, "publisher", path),
    survey: optStr(o, "survey", path),
    dataset: optStr(o, "dataset", path),
    provider: optStr(o, "provider", path),
    indicatorCode: optStr(o, "indicatorCode", path),
    regions: optArr(o, "regions", path, (r, p) => { const ro = obj(r, p); return { code: str(ro, "code", p), name: str(ro, "name", p) }; }),
    periods: optArr(o, "periods", path, stringItem),
    unit: optStr(o, "unit", path),
    url: optStr(o, "url", path),
    retrievedAt: optStr(o, "retrievedAt", path),
    rawFile: optStr(o, "rawFile", path),
    rawSha256: optStr(o, "rawSha256", path),
    license: optStr(o, "license", path),
    licenseUrl: optStr(o, "licenseUrl", path),
    attribution: optStr(o, "attribution", path),
  };
}

export function decodeSampleScenario(v: unknown, path: string): SampleScenario {
  const o = obj(v, path);
  const transform = obj(o.transform, `${path}.transform`);
  return {
    id: str(o, "id", path),
    projectId: str(o, "projectId", path),
    projectName: str(o, "projectName", path),
    dataKind: str(o, "dataKind", path),
    importKind: str(o, "importKind", path),
    inputSha256: str(o, "inputSha256", path),
    rows: num(o, "rows", path),
    sources: optArr(o, "sources", path, decodeSampleSource),
    transform: {
      script: strOrEmpty(transform, "script", `${path}.transform`),
      version: strOrEmpty(transform, "version", `${path}.transform`),
      description: strOrEmpty(transform, "description", `${path}.transform`),
    },
    limitations: optArr(o, "limitations", path, stringItem),
  };
}
