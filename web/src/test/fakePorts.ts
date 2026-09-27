// In-memory fakes of every application port, for component tests.
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { vi } from "vitest";
import type { Ports } from "../application/ports";
import type { Dictionaries } from "../domain/locale";
import type { AnalysisRun, EvidenceDocument, ImportFormat, Insight, LlmSettings, Project } from "../domain/models";

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
}

export function fakeState(overrides: Partial<FakeState> = {}): FakeState {
  return {
    projects: [{ id: "p1", name: "Project One", createdAt: "2026-01-01T00:00:00Z" }],
    documents: [],
    runs: [],
    insights: [],
    settings: { model: "", baseUrl: "", maskedApiKey: "", hasApiKey: false, configured: false },
    ...overrides,
  };
}

export function fakePorts(state: FakeState = fakeState()) {
  const project = (id: string) => {
    const p = state.projects.find((x) => x.id === id);
    return p ? Promise.resolve(p) : Promise.reject(new Error("project not found"));
  };
  const ports = {
    system: { health: vi.fn(async () => ({ demoBuild: false, clientName: "" })), importFormats: vi.fn(async () => formats) },
    projects: {
      list: vi.fn(async () => state.projects),
      get: vi.fn(project),
      create: vi.fn(async (name: string) => { const p = { id: `p${state.projects.length + 1}`, name, createdAt: "" }; state.projects.push(p); return p; }),
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
      start: vi.fn(async () => { const r: AnalysisRun = { id: `r${state.runs.length + 1}`, projectId: "p1", status: "queued", progress: 0, createdAt: "" }; state.runs.unshift(r); return r; }),
      compare: vi.fn(),
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
    },
    locale: { loadDictionaries: vi.fn(async () => dictionaries), savedLocale: vi.fn(() => undefined), saveLocale: vi.fn(), browserLanguages: () => ["en"] },
  } satisfies Ports;
  return ports;
}
