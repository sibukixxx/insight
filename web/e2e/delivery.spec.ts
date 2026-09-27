// Delivery build, empty database, no model configured: the deterministic
// path from a fresh start to an exported report, plus error handling.
import { readFileSync } from "node:fs";
import { expect, test, type Page } from "@playwright/test";

const fixture = (name: string) => new URL(`./fixtures/${name}`, import.meta.url).pathname;
const xssTitle = "<img src=x onerror=\"window.__xss=1\">";

test.describe.configure({ mode: "serial" });

let projectHash = "";

async function projectId(page: Page): Promise<string> {
  const match = /#\/projects\/([^/?]+)/.exec(page.url());
  if (!match?.[1]) throw new Error(`not on a project page: ${page.url()}`);
  return decodeURIComponent(match[1]);
}

test("fresh start guides the user and states that the sample is not in this build", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1, name: "Insight Lab" })).toBeVisible();
  await expect(page.getByRole("list", { name: "How an analysis works" })).toBeVisible();
  await expect(page.getByText("No projects yet. Try the demo or create a project.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Try the demo" })).toBeDisabled();
  await expect(page.getByText("Production build (no demo data)")).toBeVisible();
});

test("create a project, download a template, preview and import an analysis CSV", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("Project name").fill("E2E deterministic study");
  await page.getByRole("button", { name: "Create" }).click();
  await expect(page).toHaveURL(/#\/projects\/[^/]+\/input$/);
  projectHash = `#/projects/${encodeURIComponent(await projectId(page))}`;

  const download = page.waitForEvent("download");
  await page.locator("[data-format=documents]").getByRole("link", { name: /Download template/ }).click();
  const template = await download;
  expect(template.suggestedFilename()).toBe("insight-lab-documents-template.csv");
  expect(readFileSync(await template.path(), "utf8")).toBe("id,source,title,content\n");

  await page.getByLabel("CSV file").setInputFiles(fixture("report.pdf"));
  await expect(page.getByText("report.pdf is not a supported file. Supported: .csv.", { exact: true })).toBeVisible();

  await page.getByText("Other input formats (specialized analysis)").click();
  await page.getByRole("radio", { name: "Corporate-event analysis CSV" }).check();
  await page.getByLabel("CSV file").setInputFiles(fixture("analysis.csv"));
  const preview = page.getByRole("region", { name: "Preview" });
  await expect(preview).toBeVisible();
  await expect(preview.getByText("Row 6: corporate_number is empty")).toBeVisible();
  await preview.getByText("Advanced: dataset profile").click();
  await expect(preview.getByRole("cell", { name: "corporate_number" })).toBeVisible();
  await expect(page.getByText("0 documents in this project")).toBeVisible();

  await preview.getByRole("button", { name: "Import analysis data" }).click();
  await expect(page.getByText(/Read 6 records; imported \d+ aggregate documents; skipped 1\./)).toBeVisible();
  await expect(page.getByText(/[1-9]\d* documents in this project/)).toBeVisible();
});

test("pasted text is stored and shown as text, never as markup", async ({ page }) => {
  await page.goto(`/${projectHash}/input`);
  await page.getByLabel("Source type").selectOption("web");
  await page.getByLabel("Title").fill(xssTitle);
  await page.getByLabel("Content").fill("<script>window.__xss=2</script> plain evidence text");
  await page.getByRole("button", { name: "Add document" }).click();
  await expect(page.getByText(xssTitle, { exact: true }).first()).toBeVisible();
  expect(await page.evaluate(() => (window as unknown as { __xss?: number }).__xss)).toBeUndefined();
  await expect(page.locator("main img")).toHaveCount(0);
});

test("the analysis explains the deterministic mode, shows progress and completes", async ({ page }) => {
  await page.goto(`/${projectHash}/analysis`);
  await expect(page.locator("[data-check=model]")).toContainText("No model is configured: the run is deterministic");
  await page.getByRole("button", { name: "Run analysis" }).click();
  await expect(page.getByText("The latest run is complete. Review its findings and evidence.")).toBeVisible({ timeout: 60_000 });
  await page.getByRole("link", { name: /View results/ }).click();
  await expect(page).toHaveURL(/\/findings\?run=/);
  await page.getByText("Advanced: run provenance").click();
  await expect(page.locator("[data-provenance=mode]")).toContainText("deterministic");
  await expect(page.locator("[data-provenance=model]")).toContainText("none (deterministic)");
});

test("the report of the selected run downloads", async ({ page }) => {
  await page.goto(`/${projectHash}`);
  const download = page.waitForEvent("download");
  await page.getByRole("link", { name: "Download report" }).first().click();
  const report = readFileSync(await (await download).path(), "utf8");
  expect(report.length).toBeGreaterThan(100);
  expect(report).toContain("#");
});

test("deep links, back/forward and the locale switch keep the route", async ({ page }) => {
  await page.goto(`/${projectHash}`);
  const sections = page.getByRole("navigation", { name: "Project sections" });
  await sections.getByRole("link", { name: "Traces and patterns" }).click();
  await expect(page).toHaveURL(/\/patterns\?run=/);
  const patternsUrl = page.url();
  await sections.getByRole("link", { name: "Run history" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Run history" })).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(patternsUrl);
  await page.goForward();
  await expect(page.getByRole("heading", { level: 1, name: "Run history" })).toBeVisible();

  await page.goto(patternsUrl);
  await expect(page.getByRole("heading", { name: /Expectation \/ baseline mismatches/ })).toBeVisible();
  await page.locator("#locale-select-header").selectOption("ja");
  await expect(page).toHaveURL(patternsUrl);
  const jaSections = page.getByRole("navigation", { name: "プロジェクトのセクション" });
  await expect(jaSections.getByRole("link", { name: "痕跡とパターン" })).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("lang", "ja");
  await page.reload();
  await expect(jaSections.getByRole("link", { name: "痕跡とパターン" })).toBeVisible();
  await page.locator("#locale-select-header").selectOption("en");

  await page.goto(`/${projectHash}?run=does-not-exist`);
  await expect(page.getByText("The selected run was not found in this project; showing the latest completed run.")).toBeVisible();
});

test("without a model and without datasets text evidence is blocked before starting", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("Project name").fill("E2E text only");
  await page.getByRole("button", { name: "Create" }).click();
  await page.getByLabel("Content").fill("An interview note that only a model could analyze.");
  await page.getByRole("button", { name: "Add document" }).click();
  await page.getByRole("link", { name: /Go to analysis/ }).click();
  await expect(page.locator("[data-check=model]")).toHaveAttribute("data-level", "blocked");
  await expect(page.locator("#analysis-start-help")).toContainText("No model is configured and there are no dataset documents");
  await expect(page.getByRole("button", { name: "Run analysis" })).toBeDisabled();
  const runs = await page.request.get(`/api/projects/${await projectId(page)}/analyses`);
  expect(await runs.json()).toEqual([]);
  await page.getByRole("link", { name: "Open settings" }).click();
  await expect(page).toHaveURL(/#\/settings$/);
});

test("settings show the unconfigured model and a failing connection test explains why", async ({ page }) => {
  await page.goto("/#/settings");
  await expect(page.getByText("No model is configured. Enter a base URL and model to enable model-backed analysis.")).toBeVisible();
  await page.getByRole("button", { name: "Test connection" }).click();
  await expect(page.getByText("Connection test failed: base URL and model are required")).toBeVisible();
});

test("an API error is shown with a way back", async ({ page }) => {
  await page.goto("/#/projects/missing-project");
  await expect(page.getByRole("alert")).toContainText("project not found");
  await page.getByRole("link", { name: /Back/ }).first().click();
  await expect(page.getByRole("heading", { level: 1, name: "Insight Lab" })).toBeVisible();
});


test("a real server-side failure stays visible and retry preserves run settings", async ({ page }) => {
  // Source=dataset is not sufficient to produce countable observations.
  const p = await (await page.request.post("/api/projects", { data: { name: "E2E server failure" } })).json() as { id: string };
  await page.request.post(`/api/projects/${p.id}/documents`, { data: { source: "dataset", content: "Uncountable dataset note." } });
  await page.goto(`/#/projects/${p.id}/analysis`);
  await page.getByText("Advanced settings (optional)").click();
  await page.locator("#research-question").fill("What changed?");
  await page.locator("#reasoning-profile").selectOption("CUSTOMER_INSIGHT");
  await page.locator("#output-locale").selectOption("ja-JP");
  await page.getByRole("button", { name: "Run analysis" }).click();
  await expect(page.getByRole("alert").filter({ hasText: "the LLM is not configured" }).first()).toBeVisible();
  await page.getByRole("button", { name: "Retry with the same settings" }).click();
  await expect.poll(async () => {
    const runs = await (await page.request.get(`/api/projects/${p.id}/analyses`)).json() as { status: string }[];
    return runs.filter((r) => r.status === "failed").length;
  }).toBe(2);
  const runs = await (await page.request.get(`/api/projects/${p.id}/analyses`)).json() as { researchQuestion: string; reasoningProfile: string; outputLocale: string }[];
  for (const run of runs) expect(run).toMatchObject({ researchQuestion: "What changed?", reasoningProfile: "CUSTOMER_INSIGHT", outputLocale: "ja-JP" });
});

test("an active API snapshot prevents a second start", async ({ page }) => {
  const p = await (await page.request.post("/api/projects", { data: { name: "E2E active run" } })).json() as { id: string };
  await page.request.post(`/api/projects/${p.id}/documents`, { data: { source: "dataset", content: "Dataset evidence." } });
  // Hold a running snapshot to avoid timing-dependent tests on a fast local run.
  const run = { id: "active-fixture", projectId: p.id, status: "running", progress: 10, createdAt: "2026-01-01T00:00:00Z" };
  await page.route(`**/api/projects/${p.id}/analyses`, (route) => route.fulfill({ json: [run] }));
  await page.route("**/api/analysis/active-fixture", (route) => route.fulfill({ json: run }));
  await page.route("**/api/analysis/active-fixture/events", (route) => route.fulfill({ contentType: "text/event-stream", body: 'event: progress\ndata: {"progress":10,"step":"running"}\n\n' }));
  let starts = 0;
  page.on("request", (req) => { if (req.method() === "POST" && req.url().endsWith(`/api/projects/${p.id}/analysis`)) starts++; });
  await page.goto(`/#/projects/${p.id}/analysis`);
  await expect(page.locator("[data-readiness]")).toHaveAttribute("data-readiness", "running");
  await expect(page.locator("#run-analysis")).toBeDisabled();
  await page.locator("#run-analysis").evaluate((button) => button.closest("form")?.requestSubmit());
  expect(starts).toBe(0);
  await expect(page.locator("#analysis-start-help")).toContainText("still in progress");
});
