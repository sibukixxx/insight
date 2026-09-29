import { describe, expect, it } from "vitest";
import type { AnalysisRun, EvidenceDocument, LlmSettings } from "./models";
import { assessReadiness } from "./readiness";

const doc = (source: string): EvidenceDocument => ({ id: source, projectId: "p", source, title: "", content: "x", metadata: {}, createdAt: "" });
const settings = (configured: boolean): LlmSettings => ({ model: configured ? "m" : "", baseUrl: "", maskedApiKey: "", hasApiKey: false, configured });
const running: AnalysisRun = { id: "r1", projectId: "p", status: "running", progress: 10, createdAt: "" };

describe("assessReadiness", () => {
  it("cannot start a data-backed run without evidence, but a question-only exploration can start", () => {
    const r = assessReadiness([], [], settings(true));
    expect(r.canStart).toBe(false);
    expect(r.canExplore).toBe(true);
    expect(r.status).toBe("exploratory");
    expect(r.checks[0]).toMatchObject({ id: "evidence", level: "info", fix: "input" });
  });

  it("keeps the theme saveable but cannot explore without a model", () => {
    const r = assessReadiness([], [], settings(false));
    expect(r.canExplore).toBe(false);
    expect(r.checks.find((c) => c.id === "model")).toMatchObject({ level: "warning", message: "readiness.explorationNeedsModel", fix: "settings" });
    expect(r.checks.some((c) => c.level === "blocked")).toBe(false);
  });

  it("lets the server decide when settings are unknown and blocks exploration during an active run", () => {
    expect(assessReadiness([], [], undefined).canExplore).toBe(true);
    expect(assessReadiness([], [running], settings(true)).canExplore).toBe(false);
  });

  it("never offers exploration once evidence exists", () => {
    expect(assessReadiness([doc("web")], [], settings(true)).canExplore).toBe(false);
  });

  it("blocks a second run while one is active", () => {
    expect(assessReadiness([doc("web")], [running], settings(true)).canStart).toBe(false);
  });

  it("blocks when no model is configured and no dataset exists", () => {
    const r = assessReadiness([doc("web")], [], settings(false));
    expect(r.canStart).toBe(false);
    expect(r.checks.find((c) => c.id === "model")).toMatchObject({ level: "blocked", message: "readiness.noModelNoDataset" });
  });

  it("explains the deterministic mode when datasets exist without a model", () => {
    const r = assessReadiness([doc("dataset")], [], settings(false));
    expect(r.checks.find((c) => c.id === "model")).toMatchObject({ level: "info", message: "readiness.deterministicOnly" });
  });

  it("leaves the decision to the server when settings are unknown", () => {
    const r = assessReadiness([doc("web")], [], undefined);
    expect(r.canStart).toBe(true);
    expect(r.checks.find((c) => c.id === "model")?.level).toBe("warning");
  });
});
