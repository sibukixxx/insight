import type { ImportKind } from "../../domain/models";
import type { Ports } from "../ports";

export function exportUseCases({ links }: Pick<Ports, "links">) {
  return {
    /** The Markdown report of exactly this run. */
    reportLink: (projectId: string, runId: string): string => links.projectReport(projectId, runId),
    templateLink: (kind: ImportKind): string => links.importTemplate(kind),
    researchReportLink: (researchRunId: string): string => links.researchReport(researchRunId),
    researchArtifactLink: (researchRunId: string): string => links.researchArtifact(researchRunId),
    approvedArtifactLink: (researchRunId: string): string => links.approvedResearchArtifact(researchRunId),
  };
}
