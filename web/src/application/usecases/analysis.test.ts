import { describe, expect, it, vi } from "vitest";
import type { AnalysisRun } from "../../domain/models";
import type { AnalysisStreamHandlers } from "../ports";
import { analysisUseCases, outputLocaleNotRecorded, retryInput, snapshotEvent } from "./analysis";

const run = (status: string, extra: Partial<AnalysisRun> = {}): AnalysisRun => ({ id: "r1", projectId: "p1", status, progress: 0, createdAt: "t", ...extra });

function setup(snapshot: AnalysisRun) {
  let handlers: AnalysisStreamHandlers | undefined;
  const unsubscribe = vi.fn();
  const ports = {
    projects: {} as never, evidence: {} as never, settings: {} as never,
    analysis: { get: vi.fn(async () => snapshot), list: vi.fn(), start: vi.fn(), compare: vi.fn(), cancel: vi.fn(async () => snapshot), retry: vi.fn(async () => snapshot) },
    stream: { watch: (_: string, h: AnalysisStreamHandlers) => { handlers = h; return unsubscribe; } },
  };
  return { uc: analysisUseCases(ports), handlers: () => handlers as AnalysisStreamHandlers, unsubscribe, ports };
}

describe("watchRun", () => {
  it("reports a run that completed before the stream opened", async () => {
    const { uc, handlers, unsubscribe } = setup(run("completed"));
    const events: unknown[] = [];
    uc.watchRun("r1", (e) => events.push(e));
    handlers().onOpen();
    await vi.waitFor(() => expect(events).toEqual([{ type: "completed" }]));
    expect(unsubscribe).toHaveBeenCalled();
  });

  it("reports a terminal event exactly once", async () => {
    const { uc, handlers } = setup(run("failed", { error: "boom" }));
    const events: unknown[] = [];
    uc.watchRun("r1", (e) => events.push(e));
    handlers().onEvent({ type: "failed", message: "boom" });
    handlers().onDisconnect();
    handlers().onEvent({ type: "completed" });
    await new Promise((r) => setTimeout(r, 0));
    expect(events).toEqual([{ type: "failed", message: "boom" }]);
  });

  it("passes progress through until the run finishes", () => {
    const { uc, handlers } = setup(run("running"));
    const events: unknown[] = [];
    uc.watchRun("r1", (e) => events.push(e));
    handlers().onEvent({ type: "progress", step: "starting", progress: 0 });
    expect(events).toEqual([{ type: "progress", step: "starting", progress: 0 }]);
  });
});

describe("retryAnalysis and cancelAnalysis", () => {
  it("retries through the server so the recorded output language is kept, without re-sending the request", async () => {
    const failed = run("failed", { outputLocale: "ja-JP", researchQuestion: "why" });
    const { uc, ports } = setup(failed);
    await uc.retryAnalysis(failed);
    expect(ports.analysis.retry).toHaveBeenCalledWith("r1");
    expect(ports.analysis.start).not.toHaveBeenCalled();
  });

  it("cancels by run id", async () => {
    const { uc, ports } = setup(run("running"));
    await uc.cancelAnalysis(run("running"));
    expect(ports.analysis.cancel).toHaveBeenCalledWith("r1");
  });
});

describe("watchRun when the state cannot be read", () => {
  it("reports unknown, then clears it once the run can be read again", async () => {
    const { uc, handlers, ports } = setup(run("running"));
    const states: boolean[] = [];
    ports.analysis.get.mockRejectedValueOnce(new Error("network"));
    uc.watchRun("r1", () => undefined, (unknown) => states.push(unknown));
    handlers().onDisconnect();
    await vi.waitFor(() => expect(states).toEqual([true]));
    handlers().onDisconnect();
    await vi.waitFor(() => expect(states).toEqual([true, false]));
  });
});

describe("retryInput", () => {
  it("reuses the failed run's question, profile and output locale", () => {
    expect(retryInput(run("failed", { researchQuestion: "why", reasoningProfile: "CUSTOMER_INSIGHT", outputLocale: "ja-JP" })))
      .toEqual({ researchQuestion: "why", reasoningProfile: "CUSTOMER_INSIGHT", outputLocale: "ja-JP" });
    expect(retryInput(run("failed"))).toEqual({ researchQuestion: "", reasoningProfile: "GENERAL_RESEARCH", outputLocale: "" });
  });
});

describe("snapshotEvent", () => {
  it("carries the failure code so a cancelled run is not reported as an error", () => {
    expect(snapshotEvent(run("failed", { error: "cancelled by request", failureCode: "CANCELLED" })))
      .toEqual({ type: "failed", message: "cancelled by request", code: "CANCELLED" });
  });
});

describe("outputLocaleNotRecorded", () => {
  const input = (outputLocale: string) => ({ researchQuestion: "", reasoningProfile: "GENERAL_RESEARCH", outputLocale });

  it("returns true when the server run lacks an explicitly requested locale", () => {
    expect(outputLocaleNotRecorded(input("ja-JP"), run("queued"))).toBe(true);
    expect(outputLocaleNotRecorded(input("ja-JP"), run("queued", { outputLocale: "en-US" }))).toBe(true);
  });

  it("returns false when the locale was recorded or none was requested", () => {
    expect(outputLocaleNotRecorded(input("ja-JP"), run("queued", { outputLocale: "ja-JP" }))).toBe(false);
    expect(outputLocaleNotRecorded(input(""), run("queued"))).toBe(false);
  });
});
