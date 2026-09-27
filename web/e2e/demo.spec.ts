// Demo build with the local scripted model (no paid LLM): the complete
// first-run path without README, CLI or ID copying — sample → analysis →
// evidence → report. Tagged @mobile to run on a phone viewport as well.
import { readFileSync } from "node:fs";
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
  await page.getByLabel("Language of model-written text").selectOption("ja-JP");
  await page.getByRole("button", { name: "Run analysis" }).click();
  await expect(page.getByText("The latest run is complete. Review its findings and evidence.")).toBeVisible({ timeout: 80_000 });
  await page.getByRole("navigation", { name: "Project sections" }).getByRole("link", { name: "Run history" }).click();
  const comparison = page.getByRole("region", { name: "Comparison" });
  await expect(comparison).toBeVisible();
  await expect(comparison).toContainText("Differences between runs are not evidence of cause");
});
