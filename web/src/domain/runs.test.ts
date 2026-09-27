import { describe, expect, it } from "vitest";
import type { AnalysisRun } from "./models";
import { pickRun, provenanceItems, shortFingerprint } from "./runs";

const run = (id: string, status: string, finishedAt?: string, extra: Partial<AnalysisRun> = {}): AnalysisRun => ({
  id, projectId: "p1", status, progress: 100, createdAt: "2026-01-01T00:00:00Z", ...(finishedAt ? { finishedAt } : {}), ...extra,
});

describe("pickRun", () => {
  const runs = [run("r3", "running"), run("r2", "completed", "2026-01-03T00:00:00Z"), run("r1", "completed", "2026-01-02T00:00:00Z")];

  it("returns the requested run when the project has it", () => {
    expect(pickRun(runs, "r1")).toEqual({ run: runs[2], requestedRunMissing: false });
  });

  it("falls back to the latest completed run with a notice when the requested run is unknown", () => {
    expect(pickRun(runs, "nope")).toEqual({ run: runs[1], requestedRunMissing: true });
  });

  it("returns the latest completed run and no notice when nothing is requested", () => {
    expect(pickRun(runs, undefined)).toEqual({ run: runs[1], requestedRunMissing: false });
  });

  it("returns no run when none has completed", () => {
    expect(pickRun([run("r1", "failed")], undefined).run).toBeUndefined();
  });
});

describe("provenance", () => {
  it("reports a missing fingerprint as not recorded, never as a value", () => {
    expect(shortFingerprint(undefined)).toEqual({ kind: "notRecorded" });
    expect(shortFingerprint("sha256:abcdef0123")).toEqual({ kind: "value", value: "abcdef0", full: "sha256:abcdef0123" });
  });

  it("records no model for a deterministic run instead of a gap", () => {
    const items = provenanceItems(run("r1", "completed", undefined, { metrics: { provenance: { mode: "deterministic", ruleVersion: "v1" } } }));
    expect(items.find((i) => i.field === "model")?.value).toEqual({ kind: "noModel" });
    expect(items.find((i) => i.field === "engine")?.value).toEqual({ kind: "notRecorded" });
  });

  it("treats a model-backed run without a model name as not recorded", () => {
    const items = provenanceItems(run("r1", "completed", undefined, { metrics: { provenance: { mode: "model_backed" } } }));
    expect(items.find((i) => i.field === "model")?.value).toEqual({ kind: "notRecorded" });
  });

  it("marks a dirty engine build and short commit", () => {
    const items = provenanceItems(run("r1", "completed", undefined, { executionSnapshot: { engineVersion: "v1.2.0", gitCommit: "0123456789", gitDirty: "true" } }));
    expect(items.find((i) => i.field === "engine")?.value).toMatchObject({ kind: "value", value: "v1.2.0@0123456+dirty" });
  });
});
