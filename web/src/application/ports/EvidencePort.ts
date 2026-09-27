import type { DocumentImportResult, EvidenceDocument, ImportKind, ImportPreview } from "../../domain/models";

/** A file chosen for upload. Blob keeps this layer free of DOM types. */
export interface UploadFile {
  readonly name: string;
  readonly blob: Blob;
}

export interface TextEvidenceInput {
  readonly source: string;
  readonly title: string;
  readonly content: string;
}

export interface EvidencePort {
  list(projectId: string): Promise<readonly EvidenceDocument[]>;
  get(documentId: string): Promise<EvidenceDocument>;
  addText(projectId: string, input: TextEvidenceInput): Promise<EvidenceDocument>;
  /** Dry run of an import by the server's own importer; stores nothing. */
  preview(projectId: string, kind: ImportKind, file: UploadFile): Promise<ImportPreview>;
  import(projectId: string, kind: ImportKind, file: UploadFile): Promise<DocumentImportResult>;
}
