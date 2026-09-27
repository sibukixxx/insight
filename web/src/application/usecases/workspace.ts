import type { AnalysisRun, EvidenceDocument, Insight, LlmSettings, Project } from "../../domain/models";
import { assessReadiness, type Readiness } from "../../domain/readiness";
import { isActive, latestRun, pickRun, type RunSelection } from "../../domain/runs";
import type { Ports } from "../ports";

export interface Workspace {
  readonly project: Project;
  readonly documents: readonly EvidenceDocument[];
  readonly runs: readonly AnalysisRun[];
  readonly selection: RunSelection;
  readonly insights: readonly Insight[];
  readonly settings: LlmSettings | undefined;
  readonly readiness: Readiness;
  /** The newest run while it is still queued or running. */
  readonly activeRun: AnalysisRun | undefined;
  readonly step: WorkflowStep;
}

/** Where the project is in input → analysis → results. */
export type WorkflowStep = "input" | "analysis" | "running" | "results";

export function workflowStep(documents: readonly EvidenceDocument[], runs: readonly AnalysisRun[], selection: RunSelection): WorkflowStep {
  if (isActive(latestRun(runs))) return "running";
  if (selection.run) return "results";
  if (documents.length === 0) return "input";
  return "analysis";
}

export async function loadSettingsOrUndefined(settings: Ports["settings"]): Promise<LlmSettings | undefined> {
  try {
    return await settings.get();
  } catch {
    return undefined;
  }
}

export function workspaceUseCases({ projects, evidence, analysis, results, settings }: Pick<Ports, "projects" | "evidence" | "analysis" | "results" | "settings">) {
  return {
    openWorkspace: async (projectId: string, runId: string | undefined): Promise<Workspace> => {
      const [project, documents, runs, llm] = await Promise.all([
        projects.get(projectId),
        evidence.list(projectId),
        analysis.list(projectId),
        loadSettingsOrUndefined(settings),
      ]);
      const selection = pickRun(runs, runId);
      const insights = selection.run ? await results.insights(projectId, selection.run.id) : [];
      const newest = latestRun(runs);
      return {
        project, documents, runs, selection, insights, settings: llm,
        readiness: assessReadiness(documents, runs, llm),
        activeRun: isActive(newest) ? newest : undefined,
        step: workflowStep(documents, runs, selection),
      };
    },
  };
}
