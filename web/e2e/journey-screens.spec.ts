// The five journey screens (Input, Analysis, Findings, Evidence, Report) against
// the Home shell, on desktop and phone, ja and en, with the local scripted model
// (no paid LLM). It asserts what can be asserted (shared theme, no overflow,
// Import reachable on a phone, each quote listed once); how it *looks* is judged
// from the screenshots that SHOTS_DIR saves. See #165 / #131.
import { readFileSync } from "node:fs";
import { expect, test, type Page } from "@playwright/test";

type Meta = { scenario: string; locale: "ja" | "en"; mobile: boolean };
const dictionary = (locale: string): Record<string, string> => JSON.parse(readFileSync(`public/locales/${locale}.json`, "utf8"));

// Deep Ink of the Home hero and header (#151): the same shell must frame every screen.
const INK = "rgb(20, 33, 61)";

const shot = async (page: Page, name: string, fullPage = true) => {
  const dir = process.env.SHOTS_DIR;
  if (dir) await page.screenshot({ path: `${dir}/${test.info().project.name}-${name}.png`, fullPage });
};

const assertShell = async (page: Page) => {
  await expect(page.locator("header").first()).toHaveCSS("background-color", INK);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), "no horizontal scroll").toBe(true);
};

test("import → analysis → findings → evidence → report share the Home shell and list each quote once", async ({ page }) => {
  const { scenario, locale, mobile } = test.info().project.metadata as Meta;
  const dict = dictionary(locale);
  const t = (key: string) => dict[key] ?? key;

  await page.goto("/");
  const card = page.locator(`[data-scenario=${scenario}]`);
  // A shared database may already hold projects: then samples are shown on request.
  const reveal = page.getByRole("button", { name: t("home.samples.show") });
  await expect(card.or(reveal)).toBeVisible();
  if (await reveal.isVisible()) await reveal.click();
  await expect(card).toBeVisible();
  await assertShell(page);
  await shot(page, "01-home");
  await card.getByRole("button", { name: t("samples.try") }).click();
  await page.getByRole("button", { name: t("input.sample.preview") }).click();
  const preview = page.getByRole("region", { name: t("input.preview.title"), exact: true });
  await expect(preview).toBeVisible();
  await assertShell(page);

  // Reviewing a long preview: the column guide is folded and Import stays reachable on a phone.
  await expect(page.locator("details", { hasText: t("input.format.details") }).first()).not.toHaveAttribute("open", "");
  const importButton = preview.getByRole("button", { name: t("project.import") });
  await preview.evaluate((e) => e.scrollIntoView({ block: "start" }));
  if (mobile) await expect(importButton).toBeInViewport();
  await shot(page, "02-input-preview-viewport", false);
  await shot(page, "02-input-preview");
  await importButton.click();
  await page.getByRole("region", { name: t("nextStep.label"), exact: true }).getByRole("link", { name: new RegExp(t("nextStep.analysis.action")) }).click();

  await page.getByLabel(t("analysis.outputLocale")).selectOption(locale === "ja" ? "ja-JP" : "en-US");
  await assertShell(page);
  await shot(page, "03-analysis");
  await page.getByRole("button", { name: t("project.runAnalysis"), exact: true }).click();
  await expect(page.getByText(t("analysisPage.completedNext"))).toBeVisible({ timeout: 80_000 });
  await page.getByRole("link", { name: new RegExp(t("analysisPage.viewResults")) }).click();
  const findings = page.locator("main a[href^='#/insights/']");
  await expect(findings.first()).toBeVisible();
  await assertShell(page);
  await shot(page, "04-findings");
  await findings.first().click();
  await expect(page.getByRole("list", { name: t("evidenceMap.label") })).toBeVisible();
  await assertShell(page);

  // Every quote the page shows is shown once. Folded duplicates are inside a closed <details>.
  const quotes = page.locator("main button[aria-expanded]:visible");
  await expect(quotes.first()).toBeVisible();
  const shown = (await quotes.allInnerTexts()).map((s) => s.replace(/\s+/g, " ").trim());
  expect(shown.length).toBeGreaterThan(0);
  expect(new Set(shown).size, `duplicated quotes: ${shown.join(" | ")}`).toBe(shown.length);
  await page.locator("#evidence button[aria-expanded]").first().click();
  await expect(page.locator("#evidence mark").first()).toBeVisible();
  await shot(page, "05-evidence");

  await page.getByRole("link", { name: new RegExp(t("nav.backToProject")) }).click();
  const report = page.waitForEvent("download");
  await page.getByRole("link", { name: t("project.downloadReport") }).first().click();
  expect(readFileSync(await (await report).path(), "utf8").length).toBeGreaterThan(0);
  await assertShell(page);
  await shot(page, "06-report");
});
