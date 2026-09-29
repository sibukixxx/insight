// In-memory fakes of every application port, for component tests.
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { vi } from "vitest";
import type { Ports, StartAnalysisInput } from "../application/ports";
import type { Dictionaries } from "../domain/locale";
import type { AnalysisRun, BuildInfo, EvidenceDocument, ImportFormat, IngestReceipt, Insight, LlmSettings, Project, SampleScenario } from "../domain/models";

const localesDir = resolve(dirname(fileURLToPath(import.meta.url)), "../../public/locales");
export const dictionaries: Dictionaries = {
  en: JSON.parse(readFileSync(resolve(localesDir, "en.json"), "utf8")),
  ja: JSON.parse(readFileSync(resolve(localesDir, "ja.json"), "utf8")),
};

export const formats: ImportFormat[] = [
  { kind: "documents", extensions: [".csv"], mediaTypes: ["text/csv"], encoding: "UTF-8", columns: ["id", "source", "title", "content"].map((name) => ({ name, required: true })), sourceTypes: ["document", "web", "dataset"] },
  { kind: "analysis", extensions: [".csv"], mediaTypes: ["text/csv"], encoding: "UTF-8", columns: [{ name: "corporate_number", required: true }, { name: "name", required: false }], sourceTypes: [] },
];

export interface FakeState {
  projects: Project[];
  documents: EvidenceDocument[];
  runs: AnalysisRun[];
  insights: Insight[];
  settings: LlmSettings;
  samples: SampleScenario[];
  ingests: IngestReceipt[];
}

export function fakeState(overrides: Partial<FakeState> = {}): FakeState {
  return {
    projects: [{ id: "p1", name: "Project One", createdAt: "2026-01-01T00:00:00Z" }],
    documents: [],
    runs: [],
    insights: [],
    settings: { model: "", baseUrl: "", maskedApiKey: "", hasApiKey: false, configured: false },
    samples: [],
    ingests: [],
    ...overrides,
  };
}

export function fakePorts(state: FakeState = fakeState()) {
  const project = (id: string) => {
    const p = state.projects.find((x) => x.id === id);
    return p ? Promise.resolve(p) : Promise.reject(new Error("project not found"));
  };
  const ports = {
    system: { health: vi.fn(async (): Promise<BuildInfo> => ({ demoBuild: false, clientName: "", largeIngest: { enabled: true, maxUploadBytes: 2 * 1024 ** 3 } })), importFormats: vi.fn(async () => formats) },
    projects: {
      list: vi.fn(async () => state.projects),
      get: vi.fn(project),
      create: vi.fn(async (name: string) => { const p = { id: `p${state.projects.length + 1}`, name, createdAt: "" }; state.projects.push(p); return p; }),
      createFromQuestion: vi.fn(async (question: string) => { const p = { id: `p${state.projects.length + 1}`, name: question.slice(0, 40), researchQuestion: question, createdAt: "" }; state.projects.push(p); return p; }),
      createSample: vi.fn(async () => state.projects[0] as Project),
    },
    evidence: {
      list: vi.fn(async () => state.documents),
      get: vi.fn(async (id: string) => state.documents.find((d) => d.id === id) ?? Promise.reject(new Error("document not found"))),
      addText: vi.fn(async (_: string, input: { source: string; title: string; content: string }) => {
        const d = { id: `d${state.documents.length + 1}`, projectId: "p1", metadata: {}, createdAt: "", ...input };
        state.documents.push(d);
        return d;
      }),
      preview: vi.fn(async () => ({ kind: "documents", recordsRead: 2, importable: 1, skipped: 1, errors: [{ row: 2, reason: "invalid source: \"bogus\"" }], fileHash: "abc", documents: [], totalDocuments: 1 })),
      import: vi.fn(async () => ({ imported: 1, skipped: 1, errors: [] })),
    },
    analysis: {
      list: vi.fn(async () => state.runs),
      get: vi.fn(async (id: string) => state.runs.find((r) => r.id === id) ?? Promise.reject(new Error("analysis not found"))),
      // Echoes the recorded settings like the server; outputLocale is omitted when unset.
      start: vi.fn(async (projectId: string, input: StartAnalysisInput) => {
        const r: AnalysisRun = { id: `r${state.runs.length + 1}`, projectId, status: "queued", progress: 0, createdAt: "", researchQuestion: input.researchQuestion, reasoningProfile: input.reasoningProfile, ...(input.outputLocale ? { outputLocale: input.outputLocale } : {}) };
        state.runs.unshift(r);
        return r;
      }),
      cancel: vi.fn(async (id: string) => {
        const r = state.runs.find((x) => x.id === id);
        if (!r) throw new Error("analysis not found");
        const cancelled: AnalysisRun = { ...r, status: "running", lifecycle: "CANCEL_REQUESTED" };
        state.runs = state.runs.map((x) => (x.id === id ? cancelled : x));
        return cancelled;
      }),
      // Echoes the failed run's own request like the server, as a new queued run.
      retry: vi.fn(async (id: string) => {
        const r = state.runs.find((x) => x.id === id);
        if (!r) throw new Error("analysis not found");
        const again: AnalysisRun = { id: `r${state.runs.length + 1}`, projectId: r.projectId, status: "queued", progress: 0, createdAt: "", retryOf: id, ...(r.researchQuestion ? { researchQuestion: r.researchQuestion } : {}), ...(r.reasoningProfile ? { reasoningProfile: r.reasoningProfile } : {}), ...(r.outputLocale ? { outputLocale: r.outputLocale } : {}) };
        state.runs.unshift(again);
        return again;
      }),
      compare: vi.fn(),
    },
    ingest: {
      submit: vi.fn(async (projectId: string, kind: string, file: { name: string; blob: Blob }) => {
        const r: IngestReceipt = { id: `i${state.ingests.length + 1}`, projectId, kind, state: "VALIDATING", stage: "PARSING", fileName: file.name, sizeBytes: file.blob.size, bytesRead: 0, rowsRead: 0, rowsSkipped: 0, documentsCreated: 0, errorCount: 0, errorExamples: [], errorsTruncated: false, createdAt: "" };
        state.ingests.unshift(r);
        return r;
      }),
      list: vi.fn(async () => state.ingests),
      get: vi.fn(async (_: string, id: string) => state.ingests.find((i) => i.id === id) ?? Promise.reject(new Error("ingest not found"))),
      cancel: vi.fn(async (_: string, id: string) => {
        const i = state.ingests.find((x) => x.id === id);
        if (!i) throw new Error("ingest not found");
        const cancelled: IngestReceipt = { ...i, state: "CANCELLED", stage: "DONE" };
        state.ingests = state.ingests.map((x) => (x.id === id ? cancelled : x));
        return cancelled;
      }),
    },
    stream: { watch: vi.fn(() => () => undefined) },
    results: {
      insights: vi.fn(async () => state.insights),
      insight: vi.fn(),
      patterns: vi.fn(async () => []),
      evaluation: vi.fn(async () => ({})),
    },
    research: { list: vi.fn(async () => []), create: vi.fn(), get: vi.fn(), submitReview: vi.fn(), transition: vi.fn() },
    settings: { get: vi.fn(async () => state.settings), update: vi.fn(async () => state.settings), test: vi.fn(async () => "model_backed") },
    links: {
      projectReport: (p: string, r: string) => `/report/${p}/${r}`, importTemplate: (k: string) => `/template/${k}`,
      researchReport: () => "", researchArtifact: () => "", approvedResearchArtifact: () => "",
      sampleInput: (id: string) => `/sample/${id}.csv`, ingestErrors: (p: string, i: string) => `/ingest-errors/${p}/${i}.csv`,
    },
    samples: {
      list: vi.fn(async () => state.samples),
      openProject: vi.fn(async (id: string) => {
        const scenario = state.samples.find((s) => s.id === id);
        if (!scenario) throw new Error("unknown sample scenario");
        let p = state.projects.find((x) => x.id === scenario.projectId);
        if (!p) { p = { id: scenario.projectId, name: scenario.projectName, createdAt: "" }; state.projects.push(p); }
        return p;
      }),
      input: vi.fn(async (id: string) => ({ name: `insight-sample-${id}.csv`, blob: new Blob(["id,source,title,content\n"], { type: "text/csv" }) })),
    },
    locale: { loadDictionaries: vi.fn(async () => dictionaries), savedLocale: vi.fn(() => undefined), saveLocale: vi.fn(), browserLanguages: () => ["en"] },
  } satisfies Ports;
  return ports;
}

/** A synthetic sample scenario as GET /api/demo/scenarios returns it. */
export function sampleScenario(overrides: Partial<SampleScenario> = {}): SampleScenario {
  return {
    id: "ja-shop-records", projectId: "demo-scenario-ja-shop-records", projectName: "Sample 01",
    dataKind: "synthetic", importKind: "documents", inputSha256: "49979dc0", rows: 9,
    sources: [{ kind: "synthetic", description: "fictional", rows: ["r001"], regions: [], periods: [] }],
    transform: { script: "", version: "1", description: "hand-written" },
    limitations: ["All data is fictional."],
    ...overrides,
  };
}
