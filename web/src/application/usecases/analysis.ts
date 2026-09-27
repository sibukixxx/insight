import type { AnalysisEvent, AnalysisRun, EvidenceDocument, LlmSettings, Project } from "../../domain/models";
import { assessReadiness, type Readiness } from "../../domain/readiness";
import { latestRun } from "../../domain/runs";
import type { Ports, StartAnalysisInput } from "../ports";
import { loadSettingsOrUndefined } from "./workspace";

export interface AnalysisWorkspace {
  readonly project: Project;
  readonly documents: readonly EvidenceDocument[];
  readonly runs: readonly AnalysisRun[];
  readonly settings: LlmSettings | undefined;
  readonly readiness: Readiness;
  readonly latest: AnalysisRun | undefined;
}

/** The run's own settings, to start it again after a failure. */
export function retryInput(run: AnalysisRun): StartAnalysisInput {
  return {
    researchQuestion: run.researchQuestion ?? "",
    reasoningProfile: run.reasoningProfile ?? "GENERAL_RESEARCH",
    outputLocale: run.outputLocale ?? "",
  };
}

export function snapshotEvent(run: AnalysisRun): AnalysisEvent {
  if (run.status === "completed") return { type: "completed" };
  if (run.status === "failed") return run.error === undefined ? { type: "failed" } : { type: "failed", message: run.error };
  return { type: "progress", step: run.currentStep ?? run.status, progress: run.progress };
}

export function analysisUseCases({ projects, evidence, analysis, stream, settings }: Pick<Ports, "projects" | "evidence" | "analysis" | "stream" | "settings">) {
  return {
    loadAnalysis: async (projectId: string): Promise<AnalysisWorkspace> => {
      const [project, documents, runs, llm] = await Promise.all([
        projects.get(projectId), evidence.list(projectId), analysis.list(projectId), loadSettingsOrUndefined(settings),
      ]);
      return { project, documents, runs, settings: llm, readiness: assessReadiness(documents, runs, llm), latest: latestRun(runs) };
    },
    startAnalysis: (projectId: string, input: StartAnalysisInput): Promise<AnalysisRun> =>
      analysis.start(projectId, { ...input, researchQuestion: input.researchQuestion.trim() }),
    retryAnalysis: (run: AnalysisRun): Promise<AnalysisRun> => analysis.start(run.projectId, retryInput(run)),

    /**
     * Follows a run until it completes or fails. The server only pushes
     * events that happen after subscribing, so the recorded snapshot is read
     * whenever the stream opens or drops: a run that finished in between is
     * still reported exactly once.
     */
    watchRun: (runId: string, listener: (event: AnalysisEvent) => void): (() => void) => {
      let done = false;
      let unsubscribe: () => void = () => undefined;
      const emit = (event: AnalysisEvent) => {
        if (done) return;
        if (event.type !== "progress") {
          done = true;
          unsubscribe();
        }
        listener(event);
      };
      const reconcile = () => {
        if (done) return;
        analysis.get(runId).then(
          (run) => emit(snapshotEvent(run)),
          () => undefined, // the stream keeps retrying; the next open reconciles again
        );
      };
      unsubscribe = stream.watch(runId, { onEvent: emit, onOpen: reconcile, onDisconnect: reconcile });
      if (done) unsubscribe();
      return () => {
        done = true;
        unsubscribe();
      };
    },
  };
}
