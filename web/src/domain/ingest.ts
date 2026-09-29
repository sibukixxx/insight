// Large-CSV ingest read models (server: internal/domain/ingest.go).

import type { IngestReceipt } from "./models";

/** READY, FAILED and CANCELLED never change again; anything else (including an unknown state) is still open. */
export function isTerminalIngest(state: string): boolean {
  return state === "READY" || state === "FAILED" || state === "CANCELLED";
}

/** Share of the staged file the server has read, 0-100. */
export function ingestPercent(receipt: Pick<IngestReceipt, "state" | "bytesRead" | "sizeBytes">): number {
  if (receipt.state === "READY") return 100;
  if (receipt.sizeBytes <= 0) return 0;
  return Math.min(100, Math.floor((receipt.bytesRead / receipt.sizeBytes) * 100));
}
