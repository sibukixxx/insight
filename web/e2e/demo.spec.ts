// Demo build with the local scripted model (no paid LLM): the complete
// first-run path without README, CLI or ID copying — sample → analysis →
// evidence → report. Tagged @mobile to run on a phone viewport as well.
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { expect, test } from "@playwright/test";

test("empty database → sample → analysis → evidence → report @mobile", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText("Demo build")).toBeVisible();
  await page.getByRole("button", { name: "Try the demo" }).click();
  await expect(page).toHaveURL(/#\/projects\/[^/?]+$/);

  const next = page.getByRole("region", { name: "Next step", exact: true });
  await expect(next).toContainText("Evidence is ready");
  await next.getByRole("link", { name: /Go to analysis/ }).click();

  await expect(page.locator("[data-check=model]")).toContainText("Model configured: scripted-model");
  await page.getByRole("button", { name: "Run analysis" }).click();
  await expect(page.getByText("The latest run is complete. Review its findings and evidence.")).toBeVisible({ timeout: 80_000 });
  await page.getByRole("link", { name: /View results/ }).click();

  const findings = page.locator("main a[href^='#/insights/']");
  await expect(findings.first()).toBeVisible();
  const title = (await findings.first().locator("div").first().textContent()) ?? "";
  await findings.first().click();

  await expect(page.getByRole("heading", { level: 1, name: title })).toBeVisible();
  await expect(page.getByRole("list", { name: "Evidence chain" })).toBeVisible();
  const quote = page.locator("#evidence button[aria-expanded]").first();
  await quote.scrollIntoViewIfNeeded();
  await quote.click();
  await expect(page.locator("#evidence mark").first()).toBeVisible();

  await page.getByRole("link", { name: /Back to project/ }).click();
  await expect(page).toHaveURL(/\/findings/);
  const download = page.waitForEvent("download");
  await page.getByRole("link", { name: "Download report" }).first().click();
  const report = readFileSync(await (await download).path(), "utf8");
  expect(report).toContain(title);
});

test("run history compares two runs without ranking them", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name === "demo-mobile", "desktop only");
  await page.goto("/");
  await page.locator("main a[href^='#/projects/']").first().click();
  await page.getByRole("navigation", { name: "Project sections" }).getByRole("link", { name: "Analysis" }).click();
  await page.getByText("Advanced settings (optional)").click();
  await page.getByLabel("Language of model-written text").selectOption("ja-JP");
  await page.getByRole("button", { name: "Run analysis" }).click();
  await expect(page.getByText("The latest run is complete. Review its findings and evidence.")).toBeVisible({ timeout: 80_000 });
  await page.getByRole("navigation", { name: "Project sections" }).getByRole("link", { name: "Run history" }).click();
  const comparison = page.getByRole("region", { name: "Comparison" });
  await expect(comparison).toBeVisible();
  await expect(comparison).toContainText("Differences between runs are not evidence of cause");
});

test("evaluation, patterns and the research publication review work on a completed run", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name === "demo-mobile", "desktop only");
  await page.goto("/");
  await page.locator("main a[href^='#/projects/']").first().click();
  const sections = page.getByRole("navigation", { name: "Project sections" });

  await sections.getByRole("link", { name: "Evaluation" }).click();
  await expect(page.getByText("Evidence Coverage", { exact: true })).toBeVisible();
  await expect(page.getByText(/observation candidates were verified against source text/)).toBeVisible();

  await sections.getByRole("link", { name: "Traces and patterns" }).click();
  const quote = page.locator("main article button[aria-expanded]").first();
  await expect(quote).toBeVisible();
  await quote.click();
  await expect(page.locator("main article mark").first()).toBeVisible();

  await sections.getByRole("link", { name: "Research publications" }).click();
  await page.getByLabel("Research question").fill("Did the program change the outcome?");
  await page.getByRole("button", { name: "Create from latest completed analysis" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Did the program change the outcome?" })).toBeVisible();
  const state = page.getByRole("region", { name: "Publication state" });
  await expect(state.locator("h2 code")).toHaveText(/^[A-Z_]+$/);
  await page.getByLabel("Contribution").selectOption("REPLICATION");
  await page.getByLabel("I completed the human review of this output").check();
  await page.getByRole("button", { name: "Save review and assess readiness" }).click();
  await expect(state.locator("h2 code")).toHaveText(/^[A-Z_]+$/);
  await expect(page.getByRole("alert")).toHaveCount(0);
  await page.getByRole("link", { name: /Back to research publications/ }).click();
  await expect(page.getByRole("link", { name: "Did the program change the outcome?" })).toBeVisible();
});

// The scripted model answers in milliseconds, so a run cannot be cancelled "while running" by
// timing alone. This test points the engine at a gate that holds the first model call open
// (relaying to the scripted model afterwards), which keeps the run genuinely running.
test("a running analysis can be cancelled, is shown as cancelled, and its retry keeps the chosen output language", async ({ page, request }, testInfo) => {
  test.skip(testInfo.project.name === "demo-mobile", "desktop only");
  const scripted = "http://127.0.0.1:8813";
  let openGate!: () => void;
  const gate = new Promise<void>((resolve) => { openGate = resolve; });
  const held = createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on("data", (c: Buffer) => chunks.push(c));
    req.on("end", () => {
      void gate.then(async () => {
        const answer = await fetch(`${scripted}${req.url ?? ""}`, { method: "POST", headers: { "content-type": "application/json" }, body: Buffer.concat(chunks) });
        res.writeHead(answer.status, { "content-type": "application/json" }).end(await answer.text());
      }).catch(() => res.destroy());
    });
  });
  await new Promise<void>((resolve) => held.listen(0, "127.0.0.1", resolve));
  const heldUrl = `http://127.0.0.1:${(held.address() as AddressInfo).port}`;
  const original = await (await request.get("/api/settings")).json() as { baseUrl: string; model: string };

  try {
    const project = await (await request.post("/api/projects", { data: { name: "E2E cancel and retry" } })).json() as { id: string };
    const passages = [
      "Visitors say the queue at the counter is too long at lunchtime, and several left without buying.",
      "One shop added a second counter in April; its waiting complaints dropped, while a neighbouring shop saw no change.",
      "Weekend inquiries mostly ask about opening hours, not about products.",
    ];
    for (const [n, content] of passages.entries()) {
      await request.post(`/api/projects/${project.id}/documents`, { data: { source: "document", title: `Note ${n + 1}`, content } });
    }
    await request.put("/api/settings", { data: { baseUrl: heldUrl, model: original.model, apiKey: "" } });
    await page.goto(`/#/projects/${project.id}/analysis`);

    // The output language is chosen in the main start control, not under advanced settings.
    await page.getByLabel("Language of model-written text").selectOption("ja-JP");
    await page.getByRole("button", { name: "Run analysis" }).click();
    await expect(page.locator("#analysis-panel [role=progressbar]")).toBeVisible();
    await page.getByRole("button", { name: "Cancel this run" }).click();

    await expect(page.getByText(/Cancelled\. This run was stopped before it finished/)).toBeVisible({ timeout: 60_000 });
    const cancelled = (await (await request.get(`/api/projects/${project.id}/analyses`)).json()) as { id: string; lifecycle: string; outputLocale: string }[];
    expect(cancelled).toHaveLength(1);
    expect(cancelled[0]).toMatchObject({ lifecycle: "CANCELLED", outputLocale: "ja-JP" });
    await expect(page.getByText("Failed:")).toHaveCount(0);

    // Back on the real scripted model: retry runs to completion with the recorded language.
    openGate();
    await request.put("/api/settings", { data: { baseUrl: original.baseUrl, model: original.model, apiKey: "" } });
    await page.getByRole("button", { name: "Retry with the same settings" }).click();
    await expect(page.getByText("The latest run is complete. Review its findings and evidence.")).toBeVisible({ timeout: 80_000 });
    const runs = (await (await request.get(`/api/projects/${project.id}/analyses`)).json()) as { id: string; status: string; retryOf?: string; outputLocale: string }[];
    expect(runs).toHaveLength(2);
    expect(runs[0]).toMatchObject({ status: "completed", retryOf: cancelled[0]?.id, outputLocale: "ja-JP" });
  } finally {
    openGate();
    await request.put("/api/settings", { data: { baseUrl: original.baseUrl, model: original.model, apiKey: "" } });
    held.close();
  }
});
