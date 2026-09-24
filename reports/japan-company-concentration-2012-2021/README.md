# Public Evidence Report — Japan company count and Tokyo concentration (2012→2021)

Research Question: **日本の企業数は2012年から2021年に減ったのか。減少の一方で東京への集中は強まったのか？**

The research question and source transcription originate from PR #10 (Go Public Evidence Report Factory P0). That PR was closed without merging because it would have added a second report factory next to the Python one from #11. The data now uses the same Python pipeline contract as `reports/japan-company-count-2021-2024`.

## Pipeline

```text
中小企業庁「2025年版中小企業白書 付属統計資料 6表」 (published table)
  ↓ manual transcription (2026-09-18, PR #10)
normalized.csv   (24 rows: 全国 + 5 prefectures × 2012/2014/2016/2021)
  ↓ deterministic Python calculation (Decimal, half-up) + comparability labelling
report.md
  ├─ evidence-ledger.json   (claim → source rows → calculation → comparability → counter evidence)
  ├─ note-draft.md
  └─ sns-summary.md
```

The published counts are third-party official statistics. This report's primary analysis is the deterministic delta, rate, and national-share calculation, the competing-explanation framing, and the evidence ledger. Do not describe it as Insight Lab measuring the original counts.

## Result in one line

National enterprises −488,275 (−12.6%). Tokyo −23,518 (−5.3%). Tokyo's national share went from 11.57% to 12.55% (+0.98pp). Tokyo did not grow. It declined more slowly than the country as a whole.

## Comparability

Every row carries the same `population_definition_id` (`sme_white_paper_2025_table6_private_non_primary`) because the publisher compiles all four years into one table. The derived label is therefore `COMPARABLE` **within that table**. It does not independently verify that the underlying census programs are equivalent across years. That caveat stays in `known_limitations` and in the report's "この分析では分からないこと" section.

## Source verification status

The values were transcribed from the published table on 2026-09-18. On 2026-09-24 an automated re-fetch of the source page returned HTTP 403, so the transcription has **not** been re-verified against the source since then. Re-check the 24 values and the publisher's terms before publishing.

## Reproduce

From the repository root:

```bash
python3 scripts/company_concentration_report.py \
  --input reports/japan-company-concentration-2012-2021/normalized.csv \
  --out-dir reports/japan-company-concentration-2012-2021/generated

python3 scripts/test_company_concentration_report.py
```

The generator uses only the Python standard library and does not need an LLM API key. The tests check that regenerating produces byte-identical output, so the checked-in `generated/` files cannot drift from the code.

## Files

| File | Purpose |
|---|---|
| `normalized.csv` | source-of-truth rows with provenance and population definition |
| `generated/` | outputs regenerated from `normalized.csv`; do not hand-edit |
