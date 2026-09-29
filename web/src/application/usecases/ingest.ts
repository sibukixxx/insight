import type { BuildInfo, IngestReceipt, ImportKind } from "../../domain/models";
import { isTerminalIngest } from "../../domain/ingest";
import type { Ports, UploadFile } from "../ports";

/**
 * Files above this size cannot be dry-run in one request (the server's
 * MaxImportPreviewBytes); they go through the large-ingest path instead.
 */
export const LARGE_CSV_BYTES = 32 * 1024 * 1024;

/** True when the file must use large ingest and the engine offers it. */
export function shouldIngest(build: Pick<BuildInfo, "largeIngest">, sizeBytes: number): boolean {
  return build.largeIngest?.enabled === true && sizeBytes > LARGE_CSV_BYTES;
}

export const INGEST_POLL_MS = 1000;

export function ingestUseCases({ ingest }: Pick<Ports, "ingest">) {
  return {
    submitIngest: (projectId: string, kind: ImportKind, file: UploadFile): Promise<IngestReceipt> => ingest.submit(projectId, kind, file),
    listIngests: (projectId: string): Promise<readonly IngestReceipt[]> => ingest.list(projectId),
    cancelIngest: (projectId: string, ingestId: string): Promise<IngestReceipt> => ingest.cancel(projectId, ingestId),

    /**
     * Follows an ingest by polling until it reaches READY, FAILED or
     * CANCELLED. The first read is immediate, so reopening a page shows the
     * recorded state at once. onUnknown(true) says the state cannot be read
     * now (the server is unreachable): the ingest may have finished either
     * way, and polling continues.
     */
    watchIngest: (
      projectId: string, ingestId: string, listener: (receipt: IngestReceipt) => void,
      onUnknown: (unknown: boolean) => void = () => undefined, intervalMs: number = INGEST_POLL_MS,
    ): (() => void) => {
      let stopped = false;
      let timer: ReturnType<typeof setTimeout> | undefined;
      const tick = () => {
        ingest.get(projectId, ingestId).then(
          (receipt) => {
            if (stopped) return;
            onUnknown(false);
            listener(receipt);
            if (!isTerminalIngest(receipt.state)) timer = setTimeout(tick, intervalMs);
          },
          () => {
            if (stopped) return;
            onUnknown(true);
            timer = setTimeout(tick, intervalMs);
          },
        );
      };
      tick();
      return () => {
        stopped = true;
        if (timer !== undefined) clearTimeout(timer);
      };
    },
  };
}
