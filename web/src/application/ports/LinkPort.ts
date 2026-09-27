import type { ImportKind } from "../../domain/models";

/** URLs of server downloads, so views never build API paths themselves. */
export interface LinkPort {
  projectReport(projectId: string, runId: string): string;
  importTemplate(kind: ImportKind): string;
  researchReport(researchRunId: string): string;
  researchArtifact(researchRunId: string): string;
  approvedResearchArtifact(researchRunId: string): string;
}
