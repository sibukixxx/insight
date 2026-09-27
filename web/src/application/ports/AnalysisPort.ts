import type { AnalysisEvent, AnalysisRun, RunComparison } from "../../domain/models";

export interface StartAnalysisInput {
  readonly researchQuestion: string;
  readonly reasoningProfile: string;
  /** "" leaves the server default. */
  readonly outputLocale: string;
}

export interface AnalysisPort {
  list(projectId: string): Promise<readonly AnalysisRun[]>;
  get(runId: string): Promise<AnalysisRun>;
  start(projectId: string, input: StartAnalysisInput): Promise<AnalysisRun>;
  compare(projectId: string, fromId: string, toId: string): Promise<RunComparison>;
}

export interface AnalysisStreamHandlers {
  onEvent(event: AnalysisEvent): void;
  /** The connection dropped without a server event (it may reconnect). */
  onDisconnect(): void;
  /** The stream is open; events from here on are delivered. */
  onOpen(): void;
}

export interface AnalysisStreamPort {
  /** Subscribes to a run's progress; returns the unsubscribe function. */
  watch(runId: string, handlers: AnalysisStreamHandlers): () => void;
}
