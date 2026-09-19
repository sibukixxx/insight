# Company Count Public Evidence P0 — source snapshot

This directory contains the source-side inputs for the first Public Evidence Report.

## Research question

> 日本の企業数は2012年から2021年に減ったのか。減少の一方で東京への集中は強まったのか？

## Source boundary

- `source_extract.csv` is a deliberately small transcription of the published table needed for P0.
- `source_metadata.json` records publisher, URL, retrieval date, coverage, unit, terms note, and known limitations.
- It is **not** an NTA corporate-number snapshot and must not be described as a count of incorporated entities only.
- It is **not** synthetic demo data.
- Values are inputs; all deltas, rates, shares, and narrative numbers are generated deterministically by Go code.

Primary source:

- 中小企業庁「2025年版中小企業白書 付属統計資料 6表 都道府県別規模別企業数（民営、非一次産業、2012年、2014年、2016年、2021年）」
- https://www.chusho.meti.go.jp/pamflet/hakusyo/2025/chusho/f6.html

## Reproduce

From the repository root:

```bash
go run ./cmd/public-evidence-report
```

Generated artifacts are written to `generated/`.

For an independent arithmetic check:

```bash
go test ./internal/publicreport
```

## Insight Lab handoff

`generated/insight-import.csv` has the existing generic `id,source,title,content` contract and uses `source=dataset`. It can be imported into an Insight Lab project for hypothesis generation and research-loop exploration.

The deterministic report remains authoritative for numeric claims. LLM output must not replace its counts or calculations.
