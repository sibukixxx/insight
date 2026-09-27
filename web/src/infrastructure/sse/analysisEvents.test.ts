import { describe, expect, it, vi } from "vitest";
import { sseAnalysisStream } from "./analysisEvents";

class FakeSource {
  listeners: Record<string, ((ev: Event) => void)[]> = {};
  closed = false;
  addEventListener(type: string, fn: (ev: Event) => void) { (this.listeners[type] ??= []).push(fn); }
  close() { this.closed = true; }
  emit(type: string, ev: object) { for (const fn of this.listeners[type] ?? []) fn(ev as Event); }
}

describe("sseAnalysisStream", () => {
  it("tells a server error event apart from a dropped connection", () => {
    const source = new FakeSource();
    const onEvent = vi.fn();
    const onDisconnect = vi.fn();
    sseAnalysisStream(() => source).watch("r1", { onEvent, onDisconnect, onOpen: vi.fn() });

    source.emit("error", new Event("error"));
    expect(onDisconnect).toHaveBeenCalledTimes(1);
    expect(source.closed).toBe(false);

    source.emit("progress", { data: JSON.stringify({ step: "detecting_patterns", progress: 40, message: "Found 3" }) });
    source.emit("error", { data: JSON.stringify({ message: "the LLM is not configured" }) });
    expect(onEvent.mock.calls.map((c) => c[0])).toEqual([
      { type: "progress", step: "detecting_patterns", progress: 40, message: "Found 3" },
      { type: "failed", message: "the LLM is not configured" },
    ]);
    expect(source.closed).toBe(true);
  });
});
