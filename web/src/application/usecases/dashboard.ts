import type { BuildInfo, Project } from "../../domain/models";
import type { Ports } from "../ports";

export interface Dashboard {
  readonly projects: readonly Project[];
}

export function dashboardUseCases({ projects }: Pick<Ports, "projects">) {
  return {
    loadDashboard: async (): Promise<Dashboard> => ({ projects: await projects.list() }),
    createProject: (name: string): Promise<Project> => projects.create(name.trim()),
    /** Starts an investigation from a question alone; evidence is added later. */
    createFromQuestion: (question: string): Promise<Project> => projects.createFromQuestion(question.trim()),
    /** The sample project exists only in demo builds. */
    openSample: (build: BuildInfo): Promise<Project> => {
      if (!build.demoBuild) return Promise.reject(new Error("sample data is not included in this build"));
      return projects.createSample();
    },
  };
}
