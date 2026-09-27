#!/usr/bin/env node
// raw/ のスナップショットだけを読み、デモ用 Documents CSV と scenario.json を生成する。
// 完全オフライン・決定的（現在時刻・ネットワーク・ロケールに依存しない）。
//   node examples/demo-ja/official/build.mjs          # 生成して書き込む
//   node examples/demo-ja/official/build.mjs --check  # 書き込まずに比較し、差分があれば exit 1
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(here, "..", "..", "..");
const rawDir = join(here, "raw");
const rel = (p) => relative(repoRoot, p).split("\\").join("/");
const sha256 = (buf) => createHash("sha256").update(buf).digest("hex");
const arr = (x) => (x == null ? [] : Array.isArray(x) ? x : [x]);
const fail = (msg) => {
  throw new Error(msg);
};

const TRANSFORM_VERSION = "1";
const REGION_CODES = ["08201", "08212", "08220"];
const DASHBOARD_URL = "https://dashboard.e-stat.go.jp/";
const LICENSE = "公共データ利用規約（第1.0版）（PDL1.0）";
const LICENSE_URL = "https://dashboard.e-stat.go.jp/static/terms";
const ATTRIBUTION =
  "出典：統計ダッシュボード（https://dashboard.e-stat.go.jp/）。" +
  "統計ダッシュボード（https://dashboard.e-stat.go.jp/）のデータを加工して作成（Insight Lab デモ用に文章化）。" +
  "このサービスは、統計ダッシュボードのAPI機能を使用していますが、サービスの内容は国によって保証されたものではありません。";
const ROW_CREDIT = "出典：統計ダッシュボード（https://dashboard.e-stat.go.jp/）のデータを加工して作成。";

// ---------- raw snapshot loading & integrity ----------
const retrieval = JSON.parse(readFileSync(join(rawDir, "RETRIEVAL.json"), "utf8"));
const retrievalByName = new Map(retrieval.files.map((f) => [f.name, f]));
const rawBytes = new Map();
for (const f of retrieval.files) {
  const buf = readFileSync(join(rawDir, f.name));
  if (buf.length !== f.bytes) fail(`raw/${f.name}: ${buf.length} bytes, RETRIEVAL.json says ${f.bytes}`);
  if (sha256(buf) !== f.sha256) fail(`raw/${f.name}: sha256 does not match RETRIEVAL.json`);
  rawBytes.set(f.name, buf);
}
const raw = (name) => rawBytes.get(name) ?? fail(`raw/${name} is not listed in RETRIEVAL.json`);
const rawJSON = (name) => JSON.parse(raw(name).toString("utf8"));
// 統計局の概要ページ（Shift_JIS）をタグ除去した平文にする
const rawHTMLText = (name) =>
  new TextDecoder("shift_jis")
    .decode(raw(name))
    .replace(/<script[\s\S]*?<\/script>/g, " ")
    .replace(/<[^>]+>/g, " ")
    .replace(/&nbsp;/g, " ")
    .replace(/\s+/g, " ");
const requirePhrase = (name, phrase) => {
  if (!rawHTMLText(name).includes(phrase)) fail(`raw/${name} no longer contains 「${phrase}」`);
  return phrase;
};

// JST（UTC+9）の日付。取得時刻は RETRIEVAL.json の値から決定的に計算する。
const jstDate = (iso) => new Date(Date.parse(iso) + 9 * 3600 * 1000).toISOString().slice(0, 10);
const withCommas = (digits) => digits.replace(/\B(?=(\d{3})+(?!\d))/g, ",");

// ---------- regions ----------
const regionInfo = rawJSON("regions_ibaraki.json").GET_META_REGION_INF;
if (regionInfo.RESULT.status !== "0") fail("regions: API status not 0");
const prefObj = arr(regionInfo.METADATA_INF.CLASS_INF.CLASS_OBJ)[0];
const prefName = prefObj["@name"];
const regionName = new Map();
for (const c of arr(prefObj.CLASS)) {
  if (!REGION_CODES.includes(c["@regionCode"])) continue;
  if (c["@toDate"] !== "999912") fail(`region ${c["@regionCode"]} is not current in getRegionInfo`);
  if (c["@fromDate"] > "201510") fail(`region ${c["@regionCode"]} code starts after 2015-10`);
  regionName.set(c["@regionCode"], c["@name"]);
}
for (const code of REGION_CODES) if (!regionName.has(code)) fail(`region ${code} missing in getRegionInfo`);

// ---------- survey metadata ----------
function statInfo(name) {
  const s = rawJSON(name).GET_META_STAT_INFO;
  if (s.RESULT.status !== "0") fail(`${name}: API status not 0`);
  const c = arr(arr(s.METADATA_INF.CLASS_INF.CLASS_OBJ)[0].CLASS)[0];
  return { code: c["@code"], name: c["@name"], agency: c["@agency"], linkUrl: c["@linkUrl"] };
}
const kokusei = statInfo("statinfo_kokusei.json");
const ecensus = statInfo("statinfo_ecensus.json");

// ---------- getData series ----------
function series(name, indicatorCode, expectedTimes) {
  const s = rawJSON(name).GET_STATS;
  if (s.RESULT.status !== "0") fail(`${name}: API status not 0`);
  const sd = s.STATISTICAL_DATA;
  const classes = new Map(arr(sd.CLASS_INF.CLASS_OBJ).map((c) => [c["@id"], arr(c.CLASS)]));
  const ind = classes.get("indicator").find((c) => c["@code"] === indicatorCode) ?? fail(`${name}: indicator missing`);
  const unitByCode = new Map(classes.get("unit").map((u) => [u["@code"], u["$"]]));
  const statName = arr(sd.TABLE_INF.STAT_NAME).map((x) => ({ code: x["@code"], name: x["$"] }));
  const definition = arr(ind.TERM_DETAILS?.TERM_DETAIL).map((d) => String(d["$"]).replace(/^"|"$/g, ""));
  const annotations = arr(ind.INDI_ANNOTATIONS).filter((a) => a["@regionalRank"] === "4").map((a) => a["$"]);
  const values = new Map(); // `${region}|${year}` -> digits
  const units = new Set();
  for (const obj of arr(sd.DATA_INF.DATA_OBJ)) {
    const v = obj.VALUE;
    if (v["@indicator"] !== indicatorCode) fail(`${name}: unexpected indicator ${v["@indicator"]}`);
    if (v["@isProvisional"] !== "0") fail(`${name}: provisional value for ${v["@regionCode"]} ${v["@time"]}`);
    if (!/^\d+$/.test(v["$"])) fail(`${name}: non-integer value ${v["$"]}`);
    const year = v["@time"].replace(/CY00$/, "");
    values.set(`${v["@regionCode"]}|${year}`, v["$"]);
    units.add(unitByCode.get(v["@unit"]) ?? fail(`${name}: unknown unit code ${v["@unit"]}`));
  }
  if (units.size !== 1) fail(`${name}: expected exactly one unit`);
  for (const code of REGION_CODES)
    for (const y of expectedTimes) if (!values.has(`${code}|${y}`)) fail(`${name}: missing ${code} ${y}`);
  if (values.size !== REGION_CODES.length * expectedTimes.length) fail(`${name}: unexpected extra values`);
  // getData 側の地域名と getRegionInfo 側の地域名が一致することを確認
  for (const r of classes.get("regionCode"))
    if (regionName.get(r["@code"]) !== r["@name"]) fail(`${name}: region name mismatch for ${r["@code"]}`);
  const f = retrievalByName.get(name);
  return {
    file: name, indicatorCode, indicatorName: ind["@name"], unit: [...units][0], statName, definition,
    annotations, values, periods: expectedTimes, url: f.url, retrievedAt: f.retrievedAt, sha256: f.sha256,
  };
}

const pop = series("population_census_2015_2020.json", "0201010000000010000", ["2015", "2020"]);
const est21 = series("establishments_private_2021.json", "0701010001000010012", ["2021"]);
const est16 = series("establishments_private_2016.json", "0701010001000010010", ["2016"]);
if (pop.statName[0]?.code !== kokusei.code) fail("population series is not reported as 国勢調査");
if (est21.statName[0]?.code !== ecensus.code || est16.statName[0]?.code !== ecensus.code)
  fail("establishment series are not reported as 経済センサス");

// 調査期日は API にないため、統計局の調査概要ページ（raw 保存）に文言があることを確認して使う
const CENSUS_DATE = {
  2015: { phrase: requirePhrase("stat_kokusei_2015_gaiyou.html", "平成27年10月1日午前零時現在"), date: "2015年10月1日" },
  2020: { phrase: requirePhrase("stat_kokusei_2020_gaiyou.html", "令和２年10月1日午前零時現在"), date: "2020年10月1日" },
};
const ECENSUS_DATE = {
  2016: { phrase: requirePhrase("stat_ecensus_2016_gaiyo.html", "平成28年6月1日"), date: "2016年6月1日" },
  2021: { phrase: requirePhrase("stat_ecensus_2021_gaiyo.html", "令和3年6月1日"), date: "2021年6月1日" },
};
const ECENSUS_2016_EXCLUDES_PUBLIC = requirePhrase("stat_ecensus_2016_gaiyo.html", "国・地方公共団体の事業所");
const ECENSUS_2021_PUBLIC_SURVEY = requirePhrase("stat_ecensus_2021_gaiyo.html", "国及び地方公共団体の事業所に対する調査");
void ECENSUS_2016_EXCLUDES_PUBLIC;
void ECENSUS_2021_PUBLIC_SURVEY;

const popDefinition = (pop.definition[0] ?? fail("population definition missing")).split("。")[0] + "。";
const popAnnotation = pop.annotations[0] ?? fail("population annotation missing");
const estDefinition = est21.definition[0] ?? fail("establishment definition missing");
if (est16.definition[0] !== estDefinition) fail("2016/2021 establishment definitions differ in metadata");
const est21Annotation = est21.annotations[0] ?? fail("2021 annotation missing");
const est16Annotation = est16.annotations[0] ?? fail("2016 annotation missing");

// ---------- row builders ----------
const popRow = (code, year) => {
  const name = regionName.get(code);
  const v = withCommas(pop.values.get(`${code}|${year}`));
  return {
    id: `pop-${code}-${year}`,
    source: "dataset",
    title: `${name}の総人口（国勢調査 ${year}年）`,
    content:
      `公的統計（${kokusei.agency}『${kokusei.name}』、e-Stat統計ダッシュボード 系列${pop.indicatorCode}「${pop.indicatorName}」）。` +
      `${prefName}${name}（地域コード${code}）の${CENSUS_DATE[year].date}時点の総人口は${v}${pop.unit}。` +
      `取得日${jstDate(pop.retrievedAt)}（日本時間）。この値は人口の観測値であり、変化の理由や将来予測は含まない。${ROW_CREDIT}`,
  };
};

const estRow = (s, code, year, annotation, continuityNote) => {
  const name = regionName.get(code);
  const v = withCommas(s.values.get(`${code}|${year}`));
  return {
    id: `est-${code}-${year}`,
    source: "dataset",
    title: `${name}の事業所数（民営）（経済センサス ${year}年）`,
    content:
      `公的統計（${ecensus.agency}『${ecensus.name}』、e-Stat統計ダッシュボード 系列${s.indicatorCode}「${s.indicatorName}」、系列注記「${annotation}」）。` +
      `${prefName}${name}（地域コード${code}）の${year}年の事業所数（民営）は${v}${s.unit}。` +
      `この年の経済センサスの調査期日は${ECENSUS_DATE[year].date}（統計局の調査概要より）。` +
      `民営事業所とは国及び地方公共団体の事業所を除く事業所で、事業所は「${estDefinition.split("。")[0]}」と定義される。` +
      `人口（国勢調査 2020年10月1日時点）とは調査年・時点・対象が異なるため、同一時点の比率（人口あたり事業所数など）として組み合わせてはいけない。` +
      `${continuityNote}取得日${jstDate(s.retrievedAt)}（日本時間）。${ROW_CREDIT}`,
  };
};

const pop1Rows = [];
for (const code of REGION_CODES) for (const y of pop.periods) pop1Rows.push(popRow(code, y));
pop1Rows.push({
  id: "def-population-definition",
  source: "document",
  title: "定義メモ（公式メタデータより）：総人口の定義",
  content:
    `公式メタデータの要約。統計ダッシュボードの系列「${pop.indicatorName}」（${pop.indicatorCode}）では、総人口を「${popDefinition}」と定義している。` +
    `市区町村の値は系列注記により「${popAnnotation.replace(/。$/, "")}」。市区町村・年次の値の調査名は『${pop.statName[0].name}』と表示される。` +
    `この定義は人口の数え方を示すだけで、人口が変化した理由は含まない。`,
});
pop1Rows.push({
  id: "def-population-timing",
  source: "document",
  title: "定義メモ（公式メタデータより）：調査時点と周期",
  content:
    `公式メタデータの要約。統計局の調査概要によると、国勢調査は${CENSUS_DATE[2015].phrase}（2015年）と${CENSUS_DATE[2020].phrase}（2020年）によって行われた。` +
    `このデモで取得した値は${pop.periods.join("年と")}年の2時点だけで、その間の各年の人口や月々の変化は含まれない。` +
    `2時点の差は変化の大きさを示すが、いつ・なぜ変化したかは示さない。`,
});

const mixRows = [];
for (const code of REGION_CODES) mixRows.push(popRow(code, "2020"));
for (const code of REGION_CODES) {
  mixRows.push(
    estRow(est16, code, "2016", est16Annotation,
      "2016年の値は2021年とは別の系列として登録されているため、増減を比べる前に対象範囲と定義の連続性を確認する必要がある。"),
  );
  mixRows.push(
    estRow(est21, code, "2021", est21Annotation,
      "2016年の値は別系列（～2016年）として登録されているため、そのまま増減として比べてはいけない。"),
  );
}
const [c1, c2, c3] = REGION_CODES.map((c) => regionName.get(c));
const syntheticRows = [
  {
    id: "memo-report-01",
    source: "report",
    title: "架空の確認メモ：複数資料を合わせる前に",
    content:
      `架空データ。社内の分析担当が書いた想定の確認メモ。${c1}・${c2}・${c3}の人口は国勢調査、事業所数は経済センサスから取っており、` +
      `調査年も調査時点も定義も違う。地域コードの対応、民営のみかどうかといった集計範囲、調査対象の違いを確認するまでは、` +
      `人口と事業所数を直接比べたり割り算したりしない。`,
  },
  {
    id: "memo-interview-01",
    source: "interview",
    title: "架空の出店担当者メモ",
    content:
      `架空データ。架空の出店担当者は「人口が一番多い地域を優先したい」と話した。` +
      `ただし家賃、商圏、競合店、交通の便、住民の購買力はまだ取得しておらず、人口の多さだけで出店先を決める根拠はそろっていない。`,
  },
  {
    id: "memo-support-01",
    source: "support",
    title: "架空の問い合わせ対応：欠損値の扱い",
    content:
      `架空データ。候補地域の家賃情報のうち${c2}の分はまだ取得できていない。` +
      `表で空欄になっているのは未取得という意味で、家賃が0円という意味ではない。欠損のまま平均や順位を出さないこと。`,
  },
];
for (const r of syntheticRows) mixRows.push(r);

// ---------- serialisation ----------
const csvField = (s) => (/[",\r\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s);
const SOURCES = new Set(["interview", "review", "support", "sales", "survey", "job_posting", "social_post",
  "dataset", "document", "report", "paper", "web", "record", "other"]);
function toCSV(rows) {
  const ids = new Set();
  for (const r of rows) {
    if (!/^[A-Za-z0-9._-]+$/.test(r.id)) fail(`non-ASCII id ${r.id}`);
    if (ids.has(r.id)) fail(`duplicate id ${r.id}`);
    ids.add(r.id);
    if (!SOURCES.has(r.source)) fail(`bad source ${r.source}`);
  }
  return ["id,source,title,content", ...rows.map((r) => [r.id, r.source, r.title, r.content].map(csvField).join(","))]
    .join("\n") + "\n";
}

const officialSource = (s, survey, publisher, periods) => ({
  kind: "official",
  publisher,
  survey,
  dataset: s.indicatorName,
  provider: "e-Stat 統計ダッシュボード API",
  indicatorCode: s.indicatorCode,
  regions: REGION_CODES.map((code) => ({ code, name: regionName.get(code) })),
  periods,
  unit: s.unit,
  url: DASHBOARD_URL,
  apiRequest: s.url,
  retrievedAt: s.retrievedAt,
  rawFile: rel(join(rawDir, s.file)),
  rawSha256: s.sha256,
  license: LICENSE,
  licenseUrl: LICENSE_URL,
  attribution: ATTRIBUTION,
});

const transform = (description) => ({ script: rel(join(here, "build.mjs")), version: TRANSFORM_VERSION, description });

function scenario(id, dataKind, csv, rows, sources, transformDescription, limitations) {
  return {
    id,
    dataKind,
    importKind: "documents",
    inputFile: "input.csv",
    inputSha256: sha256(Buffer.from(csv, "utf8")),
    rows: rows.length,
    sources,
    transform: transform(transformDescription),
    limitations,
  };
}

const outputs = new Map();
const scenarioDir = (id) => join(repoRoot, "internal", "sampledata", "scenarios", id);

{
  const csv = toCSV(pop1Rows);
  const sc = scenario("ja-official-population", "official", csv, pop1Rows,
    [officialSource(pop, kokusei.name, kokusei.agency, pop.periods)],
    "統計ダッシュボード API の保存済みレスポンス（茨城県3市の国勢調査総人口 2015・2020年）を1値1行の文章に変換し、公式メタデータの定義メモ2行を加えた。",
    [
      "国勢調査の総人口は各回10月1日午前零時現在の時点値で、取得したのは2015年と2020年の2時点だけ。途中の年の推移は含まれない。",
      "人口の増減は観測値であり、転入・転出や出生・死亡など変化の理由はこのデータからは分からない。",
      `市区町村の値には${popAnnotation.replace(/。$/, "")}（系列注記）。`,
      "地域コード08201・08212・08220は統計ダッシュボードの地域メタデータ上2015年以前から変わっていないが、境界変更の有無そのものは確認していない。",
      "取得日時点のスナップショットであり、その後の公表値の訂正は反映されない。",
    ]);
  outputs.set(join(scenarioDir(sc.id), "input.csv"), csv);
  outputs.set(join(scenarioDir(sc.id), "scenario.json"), JSON.stringify(sc, null, 2) + "\n");
}
{
  const csv = toCSV(mixRows);
  const sc = scenario("ja-population-establishments", "mixed", csv, mixRows,
    [
      officialSource(pop, kokusei.name, kokusei.agency, ["2020"]),
      officialSource(est16, ecensus.name, ecensus.agency, est16.periods),
      officialSource(est21, ecensus.name, ecensus.agency, est21.periods),
      {
        kind: "synthetic",
        description: "Insight demo 用に作成した架空メモ（3件）。実在の人物・企業・調査ではない。",
        rows: syntheticRows.map((r) => r.id),
      },
    ],
    "統計ダッシュボード API の保存済みレスポンス（茨城県3市の国勢調査総人口 2020年、経済センサス事業所数（民営）2016・2021年）を1値1行の文章に変換し、架空メモ3行を加えた。",
    [
      "人口（国勢調査 2020年10月1日時点）と事業所数（経済センサス 2016年6月1日・2021年6月1日時点）は調査年・時点・対象が異なり、同一時点の比率として組み合わせられない。",
      "事業所数は民営事業所のみで、国及び地方公共団体の事業所を含まない。",
      "2016年と2021年の事業所数は統計ダッシュボード上で別系列として登録されており、増減として比べる前に対象範囲と定義の連続性を確認する必要がある。",
      "家賃・商圏・競合・交通・購買力は取得しておらず、人口と事業所数だけでは候補地域の優劣は判断できない。",
      "架空メモ3件は実在の人物・企業・調査ではなく、公的統計の値を含まない。未取得の欄は0を意味しない。",
    ]);
  outputs.set(join(scenarioDir(sc.id), "input.csv"), csv);
  outputs.set(join(scenarioDir(sc.id), "scenario.json"), JSON.stringify(sc, null, 2) + "\n");
}

// ---------- write or check ----------
const check = process.argv.includes("--check");
let differs = 0;
for (const [path, content] of [...outputs].sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))) {
  if (check) {
    const current = existsSync(path) ? readFileSync(path) : null;
    if (!current || !current.equals(Buffer.from(content, "utf8"))) {
      console.error(`DIFFERS: ${rel(path)}`);
      differs++;
    } else {
      console.log(`ok: ${rel(path)}`);
    }
  } else {
    mkdirSync(dirname(path), { recursive: true });
    writeFileSync(path, content);
    console.log(`wrote ${rel(path)}`);
  }
}
if (check && differs) {
  console.error(`${differs} file(s) differ from a fresh offline build — run build.mjs and review.`);
  process.exit(1);
}
