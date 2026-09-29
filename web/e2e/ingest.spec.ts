// Delivery build, real server, real browser: a CSV over 32 MiB goes through
// the large-ingest path (staged upload, background check, durable receipt).
// The fixtures are generated into a temporary directory at test time and
// never committed.
import { createWriteStream, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { once } from "node:events";
import { expect, test, type APIRequestContext } from "@playwright/test";

const MIB = 1024 * 1024;

/** A valid Documents CSV of at least `minBytes`, with `badRows` rows whose source is not accepted. */
async function writeDocumentsCsv(path: string, minBytes: number, badRows: number[]): Promise<number> {
  const out = createWriteStream(path, { encoding: "utf8" });
  const write = async (chunk: string) => { if (!out.write(chunk)) await once(out, "drain"); };
  await write("id,source,title,content\n");
  const filler = "The quarterly shop inquiry log records opening hours, queue length and the reason a visitor left without buying. ".repeat(2);
  let written = 24;
  let row = 0;
  while (written < minBytes) {
    row++;
    const source = badRows.includes(row) ? "bogus" : "document";
    const line = `r${row},${source},Inquiry ${row},${filler}entry ${row}\n`;
    written += Buffer.byteLength(line);
    await write(line);
  }
  out.end();
  await once(out, "finish");
  return row;
}

async function newProject(request: APIRequestContext, name: string): Promise<string> {
  return ((await (await request.post("/api/projects", { data: { name } })).json()) as { id: string }).id;
}

test("a CSV over 32 MiB is ingested in the browser with progress, capped errors, a SAMPLE preview and a durable receipt", async ({ page, request }) => {
  test.setTimeout(300_000);
  const dir = mkdtempSync(join(tmpdir(), "insight-e2e-"));
  try {
    const file = join(dir, "large-inquiries.csv");
    const rows = await writeDocumentsCsv(file, 33 * MIB, [7, 4000, 90000]);
    const projectId = await newProject(request, "E2E large ingest");
    await page.goto(`/#/projects/${projectId}/input`);

    await page.getByLabel("CSV file").setInputFiles(file);
    // No one-request dry run for this size: the user is told what will happen and confirms first.
    await expect(page.getByText(/over the 32 MiB limit/)).toBeVisible();
    expect((await (await request.get(`/api/projects/${projectId}/documents`)).json() as unknown[]).length).toBe(0);
    await page.getByRole("button", { name: "Upload and check" }).click();

    const card = page.locator("[data-ingest-id]").first();
    await expect(card).toBeVisible({ timeout: 60_000 });
    await expect(card).toHaveAttribute("data-ingest-state", "READY", { timeout: 240_000 });
    await expect(card).toContainText(`${(rows - 3).toLocaleString("en-US")} documents were created from ${rows.toLocaleString("en-US")} rows`);

    // Capped, honest error reporting with the full export available.
    await expect(card).toContainText("3 rows were rejected");
    await expect(card.getByText(/^Row 7:/)).toBeVisible();
    const exported = card.getByRole("link", { name: "Download all rejected rows (CSV)" });
    const csv = await (await request.get((await exported.getAttribute("href")) ?? "")).text();
    expect(csv.trim().split("\n")).toHaveLength(4);

    // The preview is a SAMPLE, never presented as the whole file.
    await expect(card.getByText("SAMPLE")).toBeVisible();
    await expect(card).toContainText("This is not the whole file.");

    // Leave and reopen: the recorded state comes back from the server, still READY.
    await page.goto(`/#/projects/${projectId}`);
    await page.goto(`/#/projects/${projectId}/input`);
    await page.reload();
    const reopened = page.locator("[data-ingest-id]").first();
    await expect(reopened).toHaveAttribute("data-ingest-state", "READY");
    await expect(reopened).toContainText("documents were created");

    // Ready evidence unlocks analysis; nothing was analyzed by the import.
    await reopened.getByRole("link", { name: /Go to analysis/ }).click();
    await expect(page).toHaveURL(/\/analysis$/);
    await expect(page.getByText("No evidence yet. Add text or import a CSV first.")).toHaveCount(0);
    const runs = (await (await request.get(`/api/projects/${projectId}/analyses`)).json()) as unknown[] | null;
    expect(runs ?? []).toHaveLength(0);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("cancelling a large ingest leaves no documents and the cancelled state survives reopening", async ({ page, request }) => {
  test.setTimeout(300_000);
  const dir = mkdtempSync(join(tmpdir(), "insight-e2e-"));
  try {
    const file = join(dir, "cancel-me.csv");
    await writeDocumentsCsv(file, 160 * MIB, []);
    const projectId = await newProject(request, "E2E cancel ingest");
    await page.goto(`/#/projects/${projectId}/input`);
    await page.getByLabel("CSV file").setInputFiles(file);
    await page.getByRole("button", { name: "Upload and check" }).click();

    const card = page.locator("[data-ingest-id]").first();
    const cancel = card.getByRole("button", { name: "Cancel this import" });
    await expect(cancel).toBeVisible({ timeout: 120_000 });
    await cancel.click();
    await expect(card).toHaveAttribute("data-ingest-state", "CANCELLED", { timeout: 60_000 });
    await expect(card).toContainText("No documents were added from this file.");

    await page.reload();
    await expect(page.locator("[data-ingest-id]").first()).toHaveAttribute("data-ingest-state", "CANCELLED");
    expect((await (await request.get(`/api/projects/${projectId}/documents`)).json() as unknown[]).length).toBe(0);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
