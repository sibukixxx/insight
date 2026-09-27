#!/usr/bin/env node
// e-Stat 統計ダッシュボード API から公的統計のスナップショットを取得し、
// レスポンスを一切加工せずに raw/ へ保存する。明示的に実行するときだけ使う:
//   node examples/demo-ja/official/fetch.mjs
// 実行するとレスポンス内の取得時刻などが変わるため、raw の sha256 も変わる。
// 更新後は必ず build.mjs を実行し、差分をレビューすること。
import { createHash } from "node:crypto";
import { mkdir, writeFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const rawDir = join(here, "raw");

const API = "https://dashboard.e-stat.go.jp/api/1.0/Json";
// 茨城県 水戸市・常陸太田市・つくば市（getRegionInfo ParentRegionCode=08000 で確認）
const REGIONS = "08201,08212,08220";

// name -> request URL. 順序は RETRIEVAL.json の出力順。
export const REQUESTS = [
  // 総人口（総数）: 国勢調査、市区町村、年次、2015〜2020
  ["population_census_2015_2020.json",
    `${API}/getData?Lang=JP&IndicatorCode=0201010000000010000&RegionCode=${REGIONS}&Cycle=3&IsSeasonalAdjustment=1&TimeFrom=2015CY00&TimeTo=2020CY00&MetaGetFlg=Y`],
  // 事業所数（民営）: 経済センサス（2021年は活動調査）
  ["establishments_private_2021.json",
    `${API}/getData?Lang=JP&IndicatorCode=0701010001000010012&RegionCode=${REGIONS}&Cycle=3&IsSeasonalAdjustment=1&TimeFrom=2021CY00&TimeTo=2021CY00&MetaGetFlg=Y`],
  // 事業所数（民営）（～2016年）: 別系列として API が分けている
  ["establishments_private_2016.json",
    `${API}/getData?Lang=JP&IndicatorCode=0701010001000010010&RegionCode=${REGIONS}&Cycle=3&IsSeasonalAdjustment=1&TimeFrom=2016CY00&TimeTo=2016CY00&MetaGetFlg=Y`],
  // 地域メタデータ（市区町村名・コード有効期間）
  ["regions_ibaraki.json", `${API}/getRegionInfo?Lang=JP&ParentRegionCode=08000`],
  // 統計調査メタデータ（作成機関など）
  ["statinfo_kokusei.json", `${API}/getStatInfo?Lang=JP&StatCode=00200521`],
  ["statinfo_ecensus.json", `${API}/getStatInfo?Lang=JP&StatCode=00200552`],
  // 調査期日を確認するための総務省統計局の調査概要ページ（Shift_JIS、無加工）
  ["stat_kokusei_2015_gaiyou.html", "https://www.stat.go.jp/data/kokusei/2015/gaiyou.html"],
  ["stat_kokusei_2020_gaiyou.html", "https://www.stat.go.jp/data/kokusei/2020/gaiyou.html"],
  ["stat_ecensus_2016_gaiyo.html", "https://www.stat.go.jp/data/e-census/2016/gaiyo.html"],
  ["stat_ecensus_2021_gaiyo.html", "https://www.stat.go.jp/data/e-census/2021/gaiyo.html"],
];

async function main() {
  await mkdir(rawDir, { recursive: true });
  const files = [];
  for (const [name, url] of REQUESTS) {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`${name}: HTTP ${res.status} for ${url}`);
    const body = Buffer.from(await res.arrayBuffer());
    const retrievedAt = new Date().toISOString();
    if (name.endsWith(".json")) {
      const status = Object.values(JSON.parse(body.toString("utf8")))[0]?.RESULT?.status;
      if (status !== "0") throw new Error(`${name}: API status ${status}`);
    }
    await writeFile(join(rawDir, name), body);
    files.push({
      name,
      url,
      retrievedAt,
      bytes: body.length,
      sha256: createHash("sha256").update(body).digest("hex"),
    });
    console.log(`saved raw/${name} (${body.length} bytes)`);
  }
  await writeFile(join(rawDir, "RETRIEVAL.json"), JSON.stringify({ files }, null, 2) + "\n");
  console.log("wrote raw/RETRIEVAL.json — run build.mjs next and review the diff");
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  await main();
}
