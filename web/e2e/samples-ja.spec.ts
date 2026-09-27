// Demo build, Japanese UI, local scripted model (no paid LLM): a first-time
// user picks the synthetic Japanese shop sample, puts its CSV through the
// ordinary preview → import, runs an analysis and reaches evidence and the
// report without README, CLI or copying IDs. SHOTS_DIR saves screenshots.
import { readFileSync } from "node:fs";
import { expect, test, type Page } from "@playwright/test";

const shot = async (page: Page, name: string) => {
  const dir = process.env.SHOTS_DIR;
  if (dir) await page.screenshot({ path: `${dir}/${test.info().project.name}-${name}.png`, fullPage: true });
};

test("サンプル選択 → CSV 取り込み → 分析 → 根拠 → レポート", async ({ page }) => {
  await page.goto("/");
  const gallery = page.getByRole("region", { name: "何を調べてみますか？" });
  await expect(gallery.getByRole("listitem")).toHaveCount(3);
  await expect(gallery.locator("[data-scenario=ja-shop-records]")).toContainText("架空データ");
  await expect(gallery.locator("[data-scenario=ja-official-population]")).toContainText("実データ（公的統計）");
  await expect(gallery.locator("[data-scenario=ja-population-establishments]")).toContainText("混合（公的統計＋架空メモ）");
  await shot(page, "01-home");

  // The download is the exact CSV the importer will receive.
  const download = page.waitForEvent("download");
  await gallery.locator("[data-scenario=ja-official-population]").getByRole("link", { name: "使用データ（CSV）をダウンロード" }).click();
  const csv = readFileSync(await (await download).path(), "utf8");
  expect(csv.startsWith("id,source,title,content\n")).toBe(true);
  expect(csv).toContain("総人口は241,656人");

  await gallery.locator("[data-scenario=ja-shop-records]").getByRole("button", { name: "この例で試す" }).click();
  await expect(page).toHaveURL(/#\/projects\/demo-scenario-ja-shop-records\/input$/);
  await expect(page.getByRole("region", { name: "架空店舗の記録から、変化と足りない情報を調べる" })).toContainText("AI モデル接続済み");
  await page.getByRole("button", { name: "この CSV をプレビュー" }).click();
  const preview = page.getByRole("region", { name: "プレビュー", exact: true });
  await expect(preview).toContainText("9");
  await shot(page, "02-input-preview");
  await preview.getByRole("button", { name: "インポート" }).click();
  const next = page.getByRole("region", { name: "次のステップ", exact: true });
  await next.getByRole("link", { name: /分析へ進む/ }).click();

  await expect(page.getByText("サンプルプロジェクト：")).toBeVisible();
  await page.getByText("詳細設定（任意）").click();
  await page.getByLabel("モデルが書くテキストの言語").selectOption("ja-JP");
  await shot(page, "03-analysis");
  await page.getByRole("button", { name: "分析を実行" }).click();
  await expect(page.getByText("最新の実行が完了しました。発見と根拠を確認しましょう。")).toBeVisible({ timeout: 80_000 });
  await page.getByRole("link", { name: /結果を見る/ }).click();
  const findings = page.locator("main a[href^='#/insights/']");
  await expect(findings.first()).toBeVisible();
  await shot(page, "04-findings");
  await findings.first().click();
  await expect(page.getByRole("list", { name: "根拠のつながり" })).toBeVisible();
  const quote = page.locator("#evidence button[aria-expanded]").first();
  await quote.scrollIntoViewIfNeeded();
  await quote.click();
  // The quote is the imported Japanese source text, unchanged.
  await expect(page.locator("#evidence mark").first()).toContainText("架空データ");
  await shot(page, "05-evidence");

  await page.getByRole("link", { name: /プロジェクトへ戻る/ }).click();
  const report = page.waitForEvent("download");
  await page.getByRole("link", { name: "レポートをダウンロード" }).first().click();
  const markdown = readFileSync(await (await report).path(), "utf8");
  expect(markdown).toContain("架空データ");
  await shot(page, "06-report");
});

test("分析済みのサンプルへ戻り、スマートフォン幅でも横スクロールしない @mobile", async ({ page }) => {
  await page.goto("/");
  const card = page.locator("[data-scenario=ja-shop-records]");
  await expect(card.getByRole("button", { name: "この例で試す" })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await shot(page, "01-home");
  await card.getByRole("link", { name: /前回実行した分析結果を見る/ }).click();
  await expect(page).toHaveURL(/findings\?run=/);
  await expect(page.getByText("サンプルプロジェクト：")).toBeVisible();
  await expect(page.locator("main a[href^='#/insights/']").first()).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await shot(page, "04-findings");
  await page.getByRole("link", { name: "出典と注意事項" }).click();
  await page.getByText("出典・取得・変換の記録").click();
  await expect(page.getByText("すべて架空データであり、実在の店舗の売上・評判を表さない。")).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await shot(page, "02-input-provenance");
});
