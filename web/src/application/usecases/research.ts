import type { Project, PromotionReview, ResearchIteration, ResearchRun, ResearchRunSummary } from "../../domain/models";
import { latestCompletedRun } from "../../domain/runs";
import type { Ports } from "../ports";

export interface ResearchOverview {
  readonly project: Project;
  readonly runs: readonly ResearchRunSummary[];
  /** A research run starts from the latest completed analysis. */
  readonly hasCompletedAnalysis: boolean;
}

export interface PromotionView {
  readonly run: ResearchRun;
  /** The latest iteration; undefined when the run has none. */
  readonly iteration: ResearchIteration | undefined;
}

export function researchUseCases({ research, projects, analysis }: Pick<Ports, "research" | "projects" | "analysis">) {
  return {
    loadResearch: async (projectId: string): Promise<ResearchOverview> => {
      const [project, runs, analyses] = await Promise.all([projects.get(projectId), research.list(projectId), analysis.list(projectId)]);
      return { project, runs, hasCompletedAnalysis: latestCompletedRun(analyses) !== undefined };
    },
    createResearch: (projectId: string, question: string): Promise<ResearchRunSummary> => research.create(projectId, question),
    openPromotion: async (researchRunId: string): Promise<PromotionView> => {
      const run = await research.get(researchRunId);
      return { run, iteration: run.iterations[run.iterations.length - 1] };
    },
    submitReview: (researchRunId: string, iterationId: string, review: PromotionReview): Promise<void> =>
      research.submitReview(researchRunId, iterationId, review),
    transition: (researchRunId: string, iterationId: string, targetState: string): Promise<void> =>
      research.transition(researchRunId, iterationId, targetState),
  };
}
