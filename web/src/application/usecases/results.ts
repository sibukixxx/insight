import { evidenceChain, type EvidenceChain } from "../../domain/evidence";
import type { AnalysisRun, EvaluationMetrics, Insight, InsightDetail, Pattern, Project } from "../../domain/models";
import { completedRuns, pickRun, type RunSelection } from "../../domain/runs";
import type { Ports } from "../ports";

/** A page bound to one analysis run (?run=), with the project's run list. */
export interface RunScoped<T> {
  readonly project: Project;
  readonly runs: readonly AnalysisRun[];
  readonly selection: RunSelection;
  /** undefined when there is no completed run to show. */
  readonly data: T | undefined;
}

export interface InsightInspection {
  readonly insight: InsightDetail;
  readonly chain: EvidenceChain;
}

export interface NextActions {
  readonly canCompare: boolean;
  readonly hasFindings: boolean;
  readonly flaggedInsights: number;
}

export function nextActions(runs: readonly AnalysisRun[], insights: readonly Insight[]): NextActions {
  return {
    canCompare: completedRuns(runs).length >= 2,
    hasFindings: insights.length > 0,
    flaggedInsights: insights.filter((i) => i.qualityFlags.length > 0).length,
  };
}

export function resultsUseCases({ projects, analysis, results }: Pick<Ports, "projects" | "analysis" | "results">) {
  async function runScoped<T>(projectId: string, runId: string | undefined, load: (run: AnalysisRun) => Promise<T>): Promise<RunScoped<T>> {
    const [project, runs] = await Promise.all([projects.get(projectId), analysis.list(projectId)]);
    const selection = pickRun(runs, runId);
    const data = selection.run ? await load(selection.run) : undefined;
    return { project, runs, selection, data };
  }
  return {
    loadFindings: (projectId: string, runId: string | undefined): Promise<RunScoped<readonly Insight[]>> =>
      runScoped(projectId, runId, (run) => results.insights(projectId, run.id)),
    loadPatterns: (projectId: string, runId: string | undefined): Promise<RunScoped<readonly Pattern[]>> =>
      runScoped(projectId, runId, (run) => results.patterns(projectId, run.id)),
    loadEvaluation: (projectId: string, runId: string | undefined): Promise<RunScoped<EvaluationMetrics>> =>
      runScoped(projectId, runId, (run) => results.evaluation(projectId, run.id)),
    inspectInsight: async (insightId: string): Promise<InsightInspection> => {
      const insight = await results.insight(insightId);
      return { insight, chain: evidenceChain(insight.patterns, insight.evidence) };
    },
  };
}
