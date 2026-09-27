import { describe, expect, it, vi } from "vitest";
import type { AnalysisRun } from "../../domain/models";
import type { AnalysisStreamHandlers } from "../ports";
import { analysisUseCases, retryInput } from "./analysis";

const run = (status: string, extra: Partial<AnalysisRun> = {}): AnalysisRun => ({ id: "r1", projectId: "p1", status, progress: 0, createdAt: "t", ...extra });

function setup(snapshot: AnalysisRun) {
  let handlers: AnalysisStreamHandlers | undefined;
  const unsubscribe = vi.fn();
  const ports = {
    projects: {} as never, evidence: {} as never, settings: {} as never,
    analysis: { get: vi.fn(async () => snapshot), list: vi.fn(), start: vi.fn(), compare: vi.fn() },
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

describe("retryInput", () => {
  it("reuses the failed run's question, profile and output locale", () => {
    expect(retryInput(run("failed", { researchQuestion: "why", reasoningProfile: "CUSTOMER_INSIGHT", outputLocale: "ja-JP" })))
      .toEqual({ researchQuestion: "why", reasoningProfile: "CUSTOMER_INSIGHT", outputLocale: "ja-JP" });
    expect(retryInput(run("failed"))).toEqual({ researchQuestion: "", reasoningProfile: "GENERAL_RESEARCH", outputLocale: "" });
  });
});
