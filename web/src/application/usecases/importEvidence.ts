import type { DocumentImportResult, EvidenceDocument, ImportFormat, ImportKind, ImportPreview, Project } from "../../domain/models";
import type { Ports, TextEvidenceInput, UploadFile } from "../ports";

export interface InputWorkspace {
  readonly project: Project;
  readonly documents: readonly EvidenceDocument[];
  readonly formats: readonly ImportFormat[];
}

/** True when the file name carries one of the format's accepted extensions. */
export function matchesFormat(fileName: string, format: ImportFormat): boolean {
  const lower = fileName.toLowerCase();
  return format.extensions.some((ext) => lower.endsWith(ext.toLowerCase()));
}

export function importEvidenceUseCases({ projects, evidence, system }: Pick<Ports, "projects" | "evidence" | "system">) {
  return {
    loadInput: async (projectId: string): Promise<InputWorkspace> => {
      const [project, documents, formats] = await Promise.all([projects.get(projectId), evidence.list(projectId), system.importFormats()]);
      return { project, documents, formats };
    },
    addTextEvidence: (projectId: string, input: TextEvidenceInput): Promise<EvidenceDocument> => evidence.addText(projectId, input),
    previewImport: (projectId: string, kind: ImportKind, file: UploadFile): Promise<ImportPreview> => evidence.preview(projectId, kind, file),
    importFile: (projectId: string, kind: ImportKind, file: UploadFile): Promise<DocumentImportResult> => evidence.import(projectId, kind, file),
    sourceDocument: (documentId: string): Promise<EvidenceDocument> => evidence.get(documentId),
  };
}
