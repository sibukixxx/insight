import type { PromotionReview, ResearchRun, ResearchRunSummary } from "../../domain/models";

export interface ResearchPort {
  list(projectId: string): Promise<readonly ResearchRunSummary[]>;
  create(projectId: string, question: string): Promise<ResearchRunSummary>;
  get(researchRunId: string): Promise<ResearchRun>;
  submitReview(researchRunId: string, iterationId: string, review: PromotionReview): Promise<void>;
  transition(researchRunId: string, iterationId: string, targetState: string): Promise<void>;
}
