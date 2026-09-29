// #158: question-first exploration in the browser, Japanese UI.
//   @model    demo server + local scripted model (no paid LLM): a sentence alone
//             → project → model-backed, unverified candidates → revisit.
//   @nomodel  delivery server (no model): the theme is saved, exploration is
//             refused with a reason and a settings link, and nothing is faked.
import { expect, test, type Page } from "@playwright/test";

const QUESTION = "地方都市で人口が減っているのに店舗が増える理由は？";

async function noOverflow(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
}

async function startFromQuestion(page: Page) {
  await page.goto("/");
  await page.getByLabel("調べたいこと").fill(QUESTION);
  await page.getByRole("button", { name: "この問いで調べ始める" }).click();
  await expect(page).toHaveURL(/#\/projects\/[^/]+\/analysis$/);
  const projectId = /projects\/([^/]+)\/analysis/.exec(page.url())?.[1];
  if (!projectId) throw new Error(`no project in ${page.url()}`);
  return projectId;
}

test("@model 文章だけで探索 → 未検証の候補・誤りとなる条件・必要データ → 再訪 → 後から資料追加", async ({ page }) => {
  const projectId = await startFromQuestion(page);
  // The theme was saved with the project and prefills the question field.
  await expect(page.locator("#research-question")).toHaveValue(QUESTION);
  await noOverflow(page);
  await page.getByText("詳細設定（任意）").click();
  await page.getByLabel("モデルが書くテキストの言語").selectOption("ja-JP");
  await page.getByRole("button", { name: "仮説を探索する（資料なし）" }).click();
  await expect(page.getByText("最新の実行が完了しました。発見と根拠を確認しましょう。")).toBeVisible({ timeout: 80_000 });
  await page.getByRole("link", { name: /結果を見る/ }).click();

  const result = page.getByRole("region", { name: "仮説の候補" });
  await expect(result).toContainText("未検証・資料は使っていません");
  await expect(result.locator("[data-candidate]")).toHaveCount(2);
  await expect(result.getByRole("heading", { name: "誤りとなる条件" }).first()).toBeVisible();
  await expect(result.getByRole("heading", { name: "確かめるために必要なデータ" }).first()).toBeVisible();
  await expect(page.getByRole("region", { name: "制約・不明点" })).toContainText("資料が与えられていません");
  await expect(page.locator("#exploration-result")).toHaveAttribute("data-verified", "false");
  await noOverflow(page);

  // No evidence of any kind was invented for the exploration.
  expect(await (await page.request.get(`/api/projects/${projectId}/documents`)).json()).toEqual([]);
  expect(await (await page.request.get(`/api/projects/${projectId}/insights`)).json()).toEqual([]);

  // Revisiting shows the same saved result.
  await page.reload();
  await expect(page.getByRole("region", { name: "仮説の候補" })).toContainText("未検証・資料は使っていません");

  // Evidence is added later to the same investigation; the earlier candidates stay reachable.
  await page.getByRole("link", { name: /資料を追加して検証する/ }).click();
  await expect(page).toHaveURL(new RegExp(`#/projects/${projectId}/input$`));
  await page.request.post(`/api/projects/${projectId}/documents`, { data: { source: "web", content: "店舗数は2015年から2020年にかけて増えた。" } });
  await page.goto(`/#/projects/${projectId}/analysis`);
  await expect(page.locator("#earlier-exploration")).toContainText("未検証の候補が2件");
  await expect(page.getByRole("button", { name: "分析を実行" })).toBeVisible();
  await expect(page.getByRole("button", { name: "仮説を探索する（資料なし）" })).toHaveCount(0);
});

test("@nomodel モデル未設定でも問いは保存でき、探索は理由と設定への導線を示して実行しない", async ({ page }) => {
  const projectId = await startFromQuestion(page);
  await expect(page.locator("#research-question")).toHaveValue(QUESTION);
  await expect(page.getByText(/問いからの探索にはモデルの接続が必要です/)).toBeVisible();
  await expect(page.getByRole("link", { name: "設定を開く" }).first()).toHaveAttribute("href", "#/settings");
  await expect(page.getByRole("button", { name: "仮説を探索する（資料なし）" })).toBeDisabled();
  // Saved, not run: no analysis exists and the saved theme survives a reload.
  expect(await (await page.request.get(`/api/projects/${projectId}/analyses`)).json()).toEqual([]);
  await page.reload();
  await expect(page.locator("#research-question")).toHaveValue(QUESTION);
  // The server refuses too, with a reason, if the client is bypassed.
  const refused = await page.request.post(`/api/projects/${projectId}/analysis`, { data: { exploratory: true, researchQuestion: QUESTION } });
  expect(refused.status()).toBe(409);
  expect(await refused.json()).toMatchObject({ error: expect.stringContaining("model") });
});
