import { latestCompletedRun } from "../../domain/runs";
import type { AnalysisRun, BuildInfo, Project, SampleScenario } from "../../domain/models";
import type { Ports, UploadFile } from "../ports";

/** A gallery entry: the scenario and, once it was tried, its latest completed run. */
export interface SampleCard {
  readonly scenario: SampleScenario;
  readonly projectExists: boolean;
  readonly latestCompleted?: AnalysisRun | undefined;
}

export function samplesUseCases({ samples, analysis }: Pick<Ports, "samples" | "analysis">) {
  let cached: Promise<readonly SampleScenario[]> | undefined;
  const list = (build: BuildInfo): Promise<readonly SampleScenario[]> => {
    if (!build.demoBuild) return Promise.resolve([]);
    cached ??= samples.list().catch((e: unknown) => { cached = undefined; throw e; });
    return cached;
  };
  return {
    listScenarios: list,
    /** Gallery cards; a run link is offered only for a run that actually completed. */
    loadGallery: async (build: BuildInfo, projects: readonly Project[]): Promise<readonly SampleCard[]> => {
      const scenarios = await list(build);
      return Promise.all(scenarios.map(async (scenario): Promise<SampleCard> => {
        if (!projects.some((p) => p.id === scenario.projectId)) return { scenario, projectExists: false };
        return { scenario, projectExists: true, latestCompleted: latestCompletedRun(await analysis.list(scenario.projectId)) };
      }));
    },
    open: (scenarioId: string): Promise<Project> => samples.openProject(scenarioId),
    loadInput: (scenarioId: string): Promise<UploadFile> => samples.input(scenarioId),
    /** The scenario a project belongs to, if it is a sample project. */
    forProject: async (build: BuildInfo, projectId: string): Promise<SampleScenario | undefined> =>
      (await list(build)).find((s) => s.projectId === projectId),
  };
}
