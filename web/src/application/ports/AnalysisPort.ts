import type { AnalysisEvent, AnalysisRun, RunComparison } from "../../domain/models";

export interface StartAnalysisInput {
  readonly researchQuestion: string;
  readonly reasoningProfile: string;
  /** "" leaves the server default. */
  readonly outputLocale: string;
  /** Question-only exploration (#158): no evidence, needs a model. */
  readonly exploratory?: boolean;
}

export interface AnalysisPort {
  list(projectId: string): Promise<readonly AnalysisRun[]>;
  get(runId: string): Promise<AnalysisRun>;
  start(projectId: string, input: StartAnalysisInput): Promise<AnalysisRun>;
  /** Asks the server to stop a queued or running run; resolves with its state (CANCELLED or CANCEL_REQUESTED). */
  cancel(runId: string): Promise<AnalysisRun>;
  /** Enqueues a failed run again as a new run with the server-recorded request (question, profile, output language). */
  retry(runId: string): Promise<AnalysisRun>;
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
