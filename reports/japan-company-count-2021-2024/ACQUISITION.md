# Acquisition record — 2024 harmonized reference table (P1)

This file documents how the two P1 rows in `normalized.csv` were obtained so the
harmonized comparison is reproducible from a public download, without an LLM and
without any authenticated API.

## What was needed

Issue #12 asks for a 2024 enterprise count whose population matches the 2021
Economic Census for Business Activity (企業等数 3,684,049; all industries excluding
public service; individual proprietorships with no employees included).

The 2024 Economic Census Basic Survey A (甲調査) excludes individual proprietorship
establishments with no employees, so its published top line (2,549,827) is not that
population. The Statistics Bureau publishes 参考表 (reference tables) "for comparison
with past results" that add the missing segment back.

## Source

| Item | Value |
|---|---|
| Statistic | 令和6年経済センサス‐基礎調査 甲調査（民営事業所） 参考表 2024年6月 |
| Publisher | 総務省統計局, distributed via e-Stat |
| Table list | https://www.e-stat.go.jp/stat-search/files?page=1&layout=datalist&lid=000001473659 |
| Table used | 企業等に関する集計 第2表「企業産業(大分類)、経営組織(3区分)別企業等数、事業所数、従業者数及び売上（収入）金額(雇用者のいない個人経営の企業を含む)－全国、都道府県、市区町村」 |
| Dataset page | https://www.e-stat.go.jp/stat-search/files?layout=dataset&stat_infid=000040389376 |
| File | `/stat-search/file-download?statInfId=000040389376&fileKind=0` (Excel, about 5.9 MB, one sheet `e2_002`, 149,120 rows) |
| Usage notes | 利用上の注意（参考表） PDF: `/stat-search/file-download?statInfId=000040389310&fileKind=2` |
| e-Stat publication date | 2025-12-24 |
| Retrieved | 2026-09-19 |
| SHA-256 of the Excel file as retrieved | `3281f2f0800ffb3d9c4dddcba4561354b0f32577458e15ae171ec9aeb5c10b9e` |
| SHA-256 of the usage-notes PDF as retrieved | `74c2f3525bc73d6d088d6d9e78f9d6892b0795cb2dff8190f4043e099fc4b2b1` |

The Excel file itself is not committed (size, and e-Stat terms ask for citation of the
source rather than redistribution). Anyone can re-download it from the URL above and
verify the checksum. If e-Stat republishes the file the checksum will change; record
the new value here together with the new retrieval date rather than editing the old row.

## Cells read

Sheet `e2_002`. Rows 1–8 are titles, footnotes and headers. Values are read from the
first data rows, all with 地域識別コード `a`, 地域区分 `00000_全国`, 企業産業大分類
`AR_全産業（S_公務を除く）`:

| Row | 経営組織 | 企業等数 | Used as |
|---:|---|---:|---|
| 9 | `0_総数` | 3,352,019 | `enterprise_equivalents_harmonized`, 2024 |
| 10 | `1_会社企業` | 1,700,278 | `company_enterprises_reference`, 2024 |
| 11 | `2_会社以外の法人` | 252,223 | not normalized (context only) |
| 12 | `3_個人` | 1,399,518 | not normalized (context only) |

Consistency check performed at acquisition: 1,700,278 + 252,223 + 1,399,518 = 3,352,019.

## Footnotes that define comparability

Transcribed from rows 4–5 of the sheet:

> 1) 参考表の数値は、当調査が調査対象としていない雇用者のいない個人経営の企業・事業所について、令和３年経済センサス‐活動調査で得られた数値を含めて集計した参考値である。
> 2) 参考表の作成方法、その他留意事項については、利用上の注意を参照。

From the usage-notes PDF:

> 令和６年経済センサス‐基礎調査の甲調査の調査対象範囲には、雇用者のいない個人経営の事業所（以下、「雇なし個人」という。）が調査対象に含まれていない。
> このため、過去の結果との比較に資することを目的として、令和３年経済センサス‐活動調査において得られた雇なし個人の数値※と令和６年経済センサス‐基礎調査の雇なし個人以外の数値を合わせて、雇なし個人を含めた参考表を作成した。
> ※なお、日本標準産業分類第14 回改定に基づく組替えは反映

Consequences recorded in `normalized.csv`:

- `population_definition_id` of the 2024 harmonized row equals the 2021 row
  (`JP_all_industries_excl_public_incl_no_employee_individual`), so the pair is no
  longer `NOT_COMPARABLE`.
- `harmonization_method` is `carry_forward_2021_no_employee_individual`, not
  `observed`, so the generator labels the pair `HARMONIZED_REFERENCE`, never
  `COMPARABLE`. The no-employee individual segment has no 2024 measurement.

## Known discrepancy left open

The reference table's 会社企業 count (1,700,278) differs from 結果の概要 表II-1
(1,764,656) by 64,378. The retrieved documents do not state why. Both values are kept
as separate metrics with different `population_definition_id`s; neither is treated as
a harmonized company series. Resolving this requires the 集計事項一覧（参考表）
(`/stat-search/file-download?statInfId=000040389313&fileKind=4`) and the table notes,
which is listed as a next step in the report.

## Not done here

- Prefecture and municipality rows of the same sheet (follow-up in #12).
- 参考表「事業所の活動状態に関する集計」(存続・新設・廃業), useful as flow evidence.
- Any registry-level export; it is out of scope for this report and, per the BYO-Evidence boundary, remains an external adapter/acquisition responsibility.
