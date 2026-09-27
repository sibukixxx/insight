import type { Project } from "../../domain/models";

export interface ProjectPort {
  list(): Promise<readonly Project[]>;
  get(projectId: string): Promise<Project>;
  create(name: string): Promise<Project>;
  /** Loads the bundled sample project; fails on delivery builds. */
  createSample(): Promise<Project>;
}
