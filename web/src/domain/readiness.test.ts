import { describe, expect, it } from "vitest";
import type { AnalysisRun, EvidenceDocument, LlmSettings } from "./models";
import { assessReadiness } from "./readiness";

const doc = (source: string): EvidenceDocument => ({ id: source, projectId: "p", source, title: "", content: "x", metadata: {}, createdAt: "" });
const settings = (configured: boolean): LlmSettings => ({ model: configured ? "m" : "", baseUrl: "", maskedApiKey: "", hasApiKey: false, configured });
const running: AnalysisRun = { id: "r1", projectId: "p", status: "running", progress: 10, createdAt: "" };

describe("assessReadiness", () => {
  it("blocks a run when there is no evidence", () => {
    const r = assessReadiness([], [], settings(true));
    expect(r.canStart).toBe(false);
    expect(r.checks[0]).toMatchObject({ id: "evidence", level: "blocked", fix: "input" });
  });

  it("blocks a second run while one is active", () => {
    expect(assessReadiness([doc("web")], [running], settings(true)).canStart).toBe(false);
  });

  it("warns but does not block when no model is configured and no dataset exists", () => {
    const r = assessReadiness([doc("web")], [], settings(false));
    expect(r.canStart).toBe(true);
    expect(r.checks.find((c) => c.id === "model")).toMatchObject({ level: "warning", message: "readiness.noModelNoDataset" });
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
