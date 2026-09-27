import type { EvaluationMetrics, Insight, InsightDetail, Pattern } from "../../domain/models";

export interface ResultsPort {
  insights(projectId: string, runId: string): Promise<readonly Insight[]>;
  insight(insightId: string): Promise<InsightDetail>;
  patterns(projectId: string, runId: string): Promise<readonly Pattern[]>;
  evaluation(projectId: string, runId: string): Promise<EvaluationMetrics>;
}
