import type { IngestReceipt, ImportKind } from "../../domain/models";
import type { UploadFile } from "./EvidencePort";

/** Large-CSV ingest (POST /api/projects/{id}/ingests): staged upload, durable receipt, status by polling. */
export interface IngestPort {
  /** Streams the file to the server; resolves with the receipt as soon as it is staged. */
  submit(projectId: string, kind: ImportKind, file: UploadFile): Promise<IngestReceipt>;
  list(projectId: string): Promise<readonly IngestReceipt[]>;
  /** The receipt with a SAMPLE preview of its first documents. */
  get(projectId: string, ingestId: string): Promise<IngestReceipt>;
  cancel(projectId: string, ingestId: string): Promise<IngestReceipt>;
}
