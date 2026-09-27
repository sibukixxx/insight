import type { AnalysisRun, Project, RunComparison } from "../../domain/models";
import { completedRuns } from "../../domain/runs";
import type { Ports } from "../ports";

export interface RunHistory {
  readonly project: Project;
  readonly runs: readonly AnalysisRun[];
  /** Default comparison pair: the two most recent completed runs (older → newer). */
  readonly defaultPair: readonly [string, string] | undefined;
}

export function defaultPair(runs: readonly AnalysisRun[]): readonly [string, string] | undefined {
  const [newest, previous] = completedRuns(runs);
  return newest && previous ? [previous.id, newest.id] : undefined;
}

export function historyUseCases({ projects, analysis }: Pick<Ports, "projects" | "analysis">) {
  return {
    loadHistory: async (projectId: string): Promise<RunHistory> => {
      const [project, runs] = await Promise.all([projects.get(projectId), analysis.list(projectId)]);
      return { project, runs, defaultPair: defaultPair(runs) };
    },
    /** Records what differs between two runs; never a ranking or a causal claim. */
    compareRuns: (projectId: string, fromId: string, toId: string): Promise<RunComparison> => analysis.compare(projectId, fromId, toId),
  };
}
