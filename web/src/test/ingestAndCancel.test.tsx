import { fireEvent, screen, waitFor } from "@testing-library/preact";
import { describe, expect, it } from "vitest";
import type { IngestReceipt } from "../domain/models";
import { fakePorts, fakeState } from "./fakePorts";
import { renderApp } from "./renderApp";

const build = { demoBuild: false, clientName: "", largeIngest: { enabled: true, maxUploadBytes: 2 * 1024 ** 3 } };
const doc = { id: "d1", projectId: "p1", source: "dataset", title: "", content: "Sales fell in March.", metadata: {}, createdAt: "" };

const receipt = (overrides: Partial<IngestReceipt> = {}): IngestReceipt => ({
  id: "i1", projectId: "p1", kind: "documents", state: "VALIDATING", stage: "PARSING", fileName: "big.csv", sizeBytes: 100 * 1024 * 1024,
  bytesRead: 25 * 1024 * 1024, rowsRead: 1234, rowsSkipped: 0, documentsCreated: 0, errorCount: 0, errorExamples: [], errorsTruncated: false, createdAt: "", ...overrides,
});

/** A File that reports a size without allocating it. */
function fileOfSize(name: string, bytes: number): File {
  const file = new File(["id,source,title,content\n"], name, { type: "text/csv" });
  Object.defineProperty(file, "size", { value: bytes });
  return file;
}

describe("large CSV ingest", () => {
  it("keeps the small-file dry-run path for a file under the limit", async () => {
    const ports = fakePorts();
    renderApp(ports, { hash: "#/projects/p1/input", build });
    fireEvent.change(await screen.findByLabelText("CSV file"), { target: { files: [fileOfSize("small.csv", 1024)] } });
    expect(await screen.findByRole("button", { name: "Import" })).toBeTruthy();
    expect(ports.evidence.preview).toHaveBeenCalled();
    expect(ports.ingest.submit).not.toHaveBeenCalled();
  });

  it("stages a file over 32 MiB through ingest after an explicit confirmation, without a one-request preview", async () => {
    const ports = fakePorts();
    renderApp(ports, { hash: "#/projects/p1/input", build });
    fireEvent.change(await screen.findByLabelText("CSV file"), { target: { files: [fileOfSize("big.csv", 100 * 1024 * 1024)] } });
    expect(await screen.findByText(/over the 32 MiB limit/)).toBeTruthy();
    expect(ports.evidence.preview).not.toHaveBeenCalled();
    expect(ports.ingest.submit).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Upload and check" }));
    await waitFor(() => expect(ports.ingest.submit).toHaveBeenCalledWith("p1", "documents", expect.objectContaining({ name: "big.csv" })));
    expect(await screen.findByText(/big\.csv was received/)).toBeTruthy();
    // The new receipt appears with progress and a cancel control while it is still checking.
    expect(await screen.findByRole("progressbar", { name: "Import progress" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Cancel this import" })).toBeTruthy();
  });

  it("falls back to the ordinary path when the engine does not offer large ingest", async () => {
    const ports = fakePorts();
    renderApp(ports, { hash: "#/projects/p1/input", build: { demoBuild: false, clientName: "" } });
    fireEvent.change(await screen.findByLabelText("CSV file"), { target: { files: [fileOfSize("big.csv", 100 * 1024 * 1024)] } });
    await waitFor(() => expect(ports.evidence.preview).toHaveBeenCalled());
    expect(ports.ingest.submit).not.toHaveBeenCalled();
  });

  it("shows a cancelled receipt as cancelled with no documents, after leaving and reopening the page", async () => {
    const ports = fakePorts(fakeState({ ingests: [receipt({ state: "CANCELLED", stage: "DONE", bytesRead: 10, rowsRead: 5 })] }));
    renderApp(ports, { hash: "#/projects/p1/input", build });
    const card = await screen.findByRole("region", { name: "Large-file import: big.csv" });
    await waitFor(() => expect(card.getAttribute("data-ingest-state")).toBe("CANCELLED"));
    expect(screen.getByText("Cancelled. No documents were added from this file.")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Cancel this import" })).toBeNull();
    expect(screen.queryByRole("link", { name: /Go to analysis/ })).toBeNull();
  });

  it("shows a failed receipt's reason and a capped list of rejected rows with the full export", async () => {
    const errorExamples = Array.from({ length: 50 }, (_, n) => ({ row: n + 1, reason: `bad source ${n + 1}` }));
    const ports = fakePorts(fakeState({ ingests: [receipt({ state: "FAILED", stage: "DONE", failure: "too many rejected rows", errorCount: 4000, errorExamples, errorsTruncated: true })] }));
    renderApp(ports, { hash: "#/projects/p1/input", build });
    expect(await screen.findByText("Import failed: too many rejected rows")).toBeTruthy();
    expect(screen.getByText("Row 10: bad source 10")).toBeTruthy();
    expect(screen.queryByText("Row 11: bad source 11")).toBeNull();
    expect(screen.getByText("Showing the first 10 of 4,000 rejected rows; the CSV has all of them.")).toBeTruthy();
    expect(screen.getByRole("link", { name: "Download all rejected rows (CSV)" }).getAttribute("href")).toBe("/ingest-errors/p1/i1.csv");
  });

  it("shows a ready receipt with a SAMPLE preview labelled as such and a way on to analysis", async () => {
    const ready = receipt({ state: "READY", stage: "DONE", bytesRead: 100 * 1024 * 1024, rowsRead: 900000, documentsCreated: 899990, rowsSkipped: 10, preview: { scope: "SAMPLE", documents: [doc] } });
    renderApp(fakePorts(fakeState({ ingests: [ready] })), { hash: "#/projects/p1/input", build });
    expect(await screen.findByText(/899,990 documents were created from 900,000 rows/)).toBeTruthy();
    expect(screen.getByText("SAMPLE")).toBeTruthy();
    expect(screen.getByText(/first 1 of 899,990 documents\. This is not the whole file\./)).toBeTruthy();
    expect(screen.getAllByRole("link", { name: /Go to analysis/ }).length).toBeGreaterThan(0);
  });

  it("cancels a running ingest and then shows it as cancelled", async () => {
    const ports = fakePorts(fakeState({ ingests: [receipt()] }));
    renderApp(ports, { hash: "#/projects/p1/input", build });
    fireEvent.click(await screen.findByRole("button", { name: "Cancel this import" }));
    await waitFor(() => expect(ports.ingest.cancel).toHaveBeenCalledWith("p1", "i1"));
    expect(await screen.findByText("Cancelled. No documents were added from this file.")).toBeTruthy();
  });

  it("says the state is unknown while the server cannot be read, and does not claim success", async () => {
    const ports = fakePorts(fakeState({ ingests: [receipt()] }));
    ports.ingest.get.mockRejectedValue(new Error("network"));
    renderApp(ports, { hash: "#/projects/p1/input", build });
    expect(await screen.findByText(/this import's state is unknown/)).toBeTruthy();
    expect(screen.queryByText(/documents were created/)).toBeNull();
  });
});

describe("analysis cancel and retry", () => {
  const running = { id: "r1", projectId: "p1", status: "running", progress: 40, createdAt: "", currentStep: "extracting", lifecycle: "RUNNING", outputLocale: "ja-JP" };

  it("cancels an active run through the server and shows the pending stop", async () => {
    const ports = fakePorts(fakeState({ documents: [doc], runs: [running] }));
    renderApp(ports, { hash: "#/projects/p1/analysis" });
    fireEvent.click(await screen.findByRole("button", { name: "Cancel this run" }));
    await waitFor(() => expect(ports.analysis.cancel).toHaveBeenCalledWith("r1"));
    expect(await screen.findByText(/Stop requested\./)).toBeTruthy();
    expect((screen.getByRole("button", { name: "Cancel this run" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("shows a cancelled run as cancelled, not as an error, and retries it with its recorded output language", async () => {
    const cancelled = { ...running, status: "failed", lifecycle: "CANCELLED", failureCode: "CANCELLED", error: "cancelled by request" };
    const ports = fakePorts(fakeState({ documents: [doc], runs: [cancelled] }));
    renderApp(ports, { hash: "#/projects/p1/analysis" });
    expect(await screen.findByText(/Cancelled\. This run was stopped before it finished/)).toBeTruthy();
    expect(screen.queryByText(/Failed: cancelled by request/)).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Retry with the same settings" }));
    await waitFor(() => expect(ports.analysis.retry).toHaveBeenCalledWith("r1"));
    expect(ports.analysis.start).not.toHaveBeenCalled();
    // The retried run carries the language the cancelled run recorded.
    await expect(ports.analysis.retry.mock.results[0]?.value).resolves.toMatchObject({ outputLocale: "ja-JP", retryOf: "r1" });
    expect(screen.queryByText(/did not record the requested output language/)).toBeNull();
  });

  it("shows an interrupted run as interrupted and lets the user retry it", async () => {
    const interrupted = { ...running, status: "failed", lifecycle: "INTERRUPTED", failureCode: "INTERRUPTED", error: "the engine restarted" };
    renderApp(fakePorts(fakeState({ documents: [doc], runs: [interrupted] })), { hash: "#/projects/p1/analysis" });
    expect(await screen.findByText(/Interrupted\. The engine stopped or restarted/)).toBeTruthy();
    expect(screen.getByRole("button", { name: "Retry with the same settings" })).toBeTruthy();
  });

  it("says the run's state is unknown when the server cannot be read, without reporting success", async () => {
    const ports = fakePorts(fakeState({ documents: [doc], runs: [running] }));
    (ports.stream.watch as unknown as { mockImplementation(f: (id: string, h: { onDisconnect(): void }) => () => void): void }).mockImplementation((_id, handlers) => { handlers.onDisconnect(); return () => undefined; });
    ports.analysis.get.mockRejectedValue(new Error("network"));
    renderApp(ports, { hash: "#/projects/p1/analysis" });
    expect(await screen.findByText(/this run's state is unknown/)).toBeTruthy();
    expect(screen.queryByText(/^Completed/)).toBeNull();
  });
});
