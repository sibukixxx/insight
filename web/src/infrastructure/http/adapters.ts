// HTTP adapters for the application ports. Paths and payloads are the
// existing Reference Web REST API (internal/http/router.go); nothing here
// changes or extends its contract.

import type {
  AnalysisPort, EvidencePort, LinkPort, ProjectPort, ResearchPort, ResultsPort, SamplePort, SettingsPort, SystemPort, UploadFile,
} from "../../application/ports";
import { enc, ignoreBody, type HttpClient } from "./client";
import {
  decodeBuildInfo, decodeComparison, decodeDocument, decodeImportFormat, decodeImportPreview, decodeImportResult,
  decodeInsight, decodeInsightDetail, decodeMetrics, decodePattern, decodeProject, decodeResearchRun,
  decodeResearchSummary, decodeRun, decodeSampleScenario, decodeSettings, list,
} from "./dto";
import { obj, str } from "./decode";

const project = (id: string) => `/api/projects/${enc(id)}`;
const runQuery = (runId: string) => `?analysisId=${enc(runId)}`;
const importPath: Readonly<Record<string, string>> = { documents: "/documents/import", analysis: "/documents/import/analysis" };

function uploadForm(file: UploadFile): FormData {
  const form = new FormData();
  form.append("file", file.blob, file.name);
  return form;
}

function importEndpoint(kind: string): string {
  const path = importPath[kind];
  if (!path) throw new Error(`unsupported import kind ${kind}`);
  return path;
}

export function httpSystem(http: HttpClient): SystemPort {
  return {
    health: () => http.getJson("/api/health", decodeBuildInfo),
    importFormats: () => http.getJson("/api/import-formats", list(decodeImportFormat, "importFormats")),
  };
}

export function httpProjects(http: HttpClient): ProjectPort {
  return {
    list: () => http.getJson("/api/projects", list(decodeProject, "projects")),
    get: (id) => http.getJson(project(id), decodeProject),
    create: (name) => http.sendJson("POST", "/api/projects", { name }, decodeProject),
    createFromQuestion: (researchQuestion) => http.sendJson("POST", "/api/projects", { researchQuestion }, decodeProject),
    createSample: () => http.sendJson("POST", "/api/demo", undefined, decodeProject),
  };
}

export function httpEvidence(http: HttpClient): EvidencePort {
  return {
    list: (projectId) => http.getJson(`${project(projectId)}/documents`, list(decodeDocument, "documents")),
    get: (documentId) => http.getJson(`/api/documents/${enc(documentId)}`, decodeDocument),
    addText: (projectId, input) => http.sendJson("POST", `${project(projectId)}/documents`, input, decodeDocument),
    preview: (projectId, kind, file) =>
      http.sendForm(`${project(projectId)}/documents/import/preview?kind=${enc(kind)}`, uploadForm(file), decodeImportPreview),
    import: (projectId, kind, file) => http.sendForm(`${project(projectId)}${importEndpoint(kind)}`, uploadForm(file), decodeImportResult),
  };
}

export function httpAnalysis(http: HttpClient): AnalysisPort {
  return {
    list: (projectId) => http.getJson(`${project(projectId)}/analyses`, list(decodeRun, "analyses")),
    get: (runId) => http.getJson(`/api/analysis/${enc(runId)}`, decodeRun),
    start: (projectId, input) => {
      const body: Record<string, string | boolean> = { researchQuestion: input.researchQuestion, reasoningProfile: input.reasoningProfile };
      if (input.outputLocale) body.outputLocale = input.outputLocale;
      if (input.exploratory) body.exploratory = true;
      return http.sendJson("POST", `${project(projectId)}/analysis`, body, decodeRun);
    },
    compare: (projectId, fromId, toId) =>
      http.getJson(`${project(projectId)}/analyses/compare?a=${enc(fromId)}&b=${enc(toId)}`, decodeComparison),
  };
}

export function httpResults(http: HttpClient): ResultsPort {
  return {
    insights: (projectId, runId) => http.getJson(`${project(projectId)}/insights${runQuery(runId)}`, list(decodeInsight, "insights")),
    insight: (insightId) => http.getJson(`/api/insights/${enc(insightId)}`, decodeInsightDetail),
    patterns: (projectId, runId) => http.getJson(`${project(projectId)}/patterns${runQuery(runId)}`, list(decodePattern, "patterns")),
    evaluation: (projectId, runId) => http.getJson(`${project(projectId)}/evaluation${runQuery(runId)}`, (v) => decodeMetrics(v, "evaluation")),
  };
}

export function httpResearch(http: HttpClient): ResearchPort {
  const run = (id: string) => `/api/research-runs/${enc(id)}`;
  return {
    list: (projectId) => http.getJson(`${project(projectId)}/research-runs`, (v) => (v === null ? [] : list(decodeResearchSummary, "researchRuns")(v))),
    create: (projectId, question) => http.sendJson("POST", `${project(projectId)}/research-runs`, { question }, decodeResearchSummary),
    get: (id) => http.getJson(run(id), decodeResearchRun),
    submitReview: (id, iterationId, review) =>
      http.sendJson("PUT", `${run(id)}/iterations/${enc(iterationId)}/promotion-review`, review, ignoreBody),
    transition: (id, iterationId, targetState) =>
      http.sendJson("PUT", `${run(id)}/iterations/${enc(iterationId)}/promotion-transition`, { targetState }, ignoreBody),
  };
}

export function httpSettings(http: HttpClient): SettingsPort {
  return {
    get: () => http.getJson("/api/settings", decodeSettings),
    update: (input) => http.sendJson("PUT", "/api/settings", input, decodeSettings),
    test: () => http.sendJson("POST", "/api/settings/test", undefined, (v) => str(obj(v, "settingsTest"), "mode", "settingsTest")),
  };
}

export const httpLinks: LinkPort = {
  projectReport: (projectId, runId) => `${project(projectId)}/report.md${runQuery(runId)}`,
  importTemplate: (kind) => `/api/import-formats/${enc(kind)}/template.csv`,
  researchReport: (id) => `/api/research-runs/${enc(id)}/report.md`,
  researchArtifact: (id) => `/api/research-runs/${enc(id)}/artifact.json`,
  approvedResearchArtifact: (id) => `/api/research-runs/${enc(id)}/approved-artifact.json`,
  sampleInput: (id) => `/api/demo/scenarios/${enc(id)}/input.csv`,
};

export function httpSamples(http: HttpClient): SamplePort {
  return {
    list: () => http.getJson("/api/demo/scenarios", list(decodeSampleScenario, "scenarios")),
    openProject: (id) => http.sendJson("POST", `/api/demo/scenarios/${enc(id)}/project`, undefined, decodeProject),
    input: async (id) => ({ name: `insight-sample-${id}.csv`, blob: await http.getBlob(httpLinks.sampleInput(id)) }),
  };
}
