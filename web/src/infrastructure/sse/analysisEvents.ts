// Server-sent run progress (GET /api/analysis/{id}/events). The server
// sends named events "progress", "completed" and "error". A server-sent
// "error" and the browser's own connection error both arrive as type
// "error"; only the former carries data.

import type { AnalysisStreamHandlers, AnalysisStreamPort } from "../../application/ports";
import type { AnalysisEvent } from "../../domain/models";
import { enc } from "../http/client";

interface EventSourceLike {
  addEventListener(type: string, listener: (ev: Event) => void): void;
  close(): void;
}
export type EventSourceFactory = (url: string) => EventSourceLike;

function parse(data: unknown): Record<string, unknown> {
  if (typeof data !== "string") return {};
  try {
    const v: unknown = JSON.parse(data);
    return typeof v === "object" && v !== null ? (v as Record<string, unknown>) : {};
  } catch {
    return {};
  }
}

export function progressEvent(data: unknown): AnalysisEvent {
  const o = parse(data);
  const event: { type: "progress"; step: string; progress: number; message?: string } = {
    type: "progress",
    step: typeof o.step === "string" ? o.step : "",
    progress: typeof o.progress === "number" ? o.progress : 0,
  };
  if (typeof o.message === "string") event.message = o.message;
  return event;
}

export function sseAnalysisStream(factory: EventSourceFactory = (url) => new EventSource(url)): AnalysisStreamPort {
  return {
    watch(runId: string, handlers: AnalysisStreamHandlers) {
      const source = factory(`/api/analysis/${enc(runId)}/events`);
      let closed = false;
      const close = () => {
        if (!closed) {
          closed = true;
          source.close();
        }
      };
      source.addEventListener("open", () => handlers.onOpen());
      source.addEventListener("progress", (ev) => handlers.onEvent(progressEvent((ev as MessageEvent).data)));
      source.addEventListener("completed", () => {
        close();
        handlers.onEvent({ type: "completed" });
      });
      source.addEventListener("error", (ev) => {
        const data = (ev as MessageEvent).data;
        if (typeof data !== "string") {
          handlers.onDisconnect();
          return;
        }
        close();
        const { message, code } = parse(data);
        handlers.onEvent({ type: "failed", ...(typeof message === "string" ? { message } : {}), ...(typeof code === "string" && code !== "" ? { code } : {}) });
      });
      return close;
    },
  };
}
