# Public Evidence Report Factory P0

Status: implemented on `feat/public-evidence-report-factory-p0`.

## Goal

Turn a small, inspectable public-data snapshot into reproducible evidence artifacts without making an LLM the numeric source of truth.

Pipeline:

```text
public source
  -> source_extract.csv + source_metadata.json
  -> deterministic normalization / arithmetic
  -> analysis.json
  -> evidence-ledger.json
  -> public report
  -> note / SNS derivative source
  -> Insight Lab dataset CSV for hypothesis exploration
```

## Current-reality audit

### EXISTING

| Area | Existing implementation |
| --- | --- |
| Observation | `internal/domain/analysis.go` + observation repositories |
| Expectation / Surprise | `domain.Insight.Expectation`, `SurprisingFact`, `ExpectationBasis` |
| Competing hypotheses | `HypothesisSetID`, `HypothesisRole`, `CompetingHypotheses` |
| Evidence / Counter Evidence | `domain.Evidence` with support / counter / neutral types |
| Grounding | `internal/service/grounding.go` and source offsets |
| Confidence | application-calculated evidence-quality score |
| Validation / Identification | explicit validation, causal, and identification statuses |
| Confounders / causal structure | candidate causal variables and CONFOUNDER role |
| Research loop | append-only `ResearchRun` / `ResearchIteration`, gaps, data requirements |
| CSV ingestion | generic `id,source,title,content` importer |
| Public dataset source type | `SourceDataset` |
| Markdown report | project / research Markdown export |
| ja-company-base boundary | deterministic NTA AnalysisRecord CSV adapter already implemented |

### REUSABLE

- Existing domain semantics and causal guardrails.
- Generic `dataset` document source.
- Research-gap and what-we-cannot-conclude fields.
- `ja-company-base` CSV contract for administrative-event analysis.
- Existing report/export conventions.

### MISSING before this P0

- A committed real public-data source snapshot for a public evidence report.
- A deterministic enterprise-stock calculation path independent of LLM/API credentials.
- Claim-level Evidence Ledger mapping prose -> arithmetic -> source.
- Answer-first public report format and derivative note/SNS material.
- One-command regeneration of those artifacts.

### DO NOT TOUCH

- Do not rename or duplicate Observation, Evidence, Counter Evidence, Expectation, Surprise, or ResearchRun concepts.
- Do not weaken `NOT_IDENTIFIED` / causal guardrails.
- Do not reinterpret NTA `ASSIGNED` events as startups or business commencements.
- Do not commit `HOUJIN_APP_ID`, `INSIGHT_LAB_API_KEY`, or other secrets.
- Do not make the LLM responsible for numeric aggregation or provenance.

## First research question

> 日本の企業数は2012年から2021年に減ったのか。減少の一方で東京への集中は強まったのか？

This question was selected instead of forcing the National Tax Agency delta API into a stock-count question.

`ja-company-base` correctly exposes administrative lifecycle/update records. Those records are useful for event analysis, but a count of update records is not an authoritative point-in-time enterprise stock. Using that feed to answer whether the stock of enterprises fell would create a semantic error.

For P0, the stock comparison therefore uses the Small and Medium Enterprise Agency's published prefectural enterprise-count table. The NTA full-record download remains a next-validation source for a separate, corporation-number-based definition.

## Real source

- Publisher: 中小企業庁
- Table: 2025年版中小企業白書 付属統計資料 6表
- Subject: 都道府県別規模別企業数（民営、非一次産業、2012年、2014年、2016年、2021年）
- URL: https://www.chusho.meti.go.jp/pamflet/hakusyo/2025/chusho/f6.html
- Retrieval date recorded in metadata: 2026-09-18

The P0 source extract is deliberately small: national totals plus five large prefectures. It is sufficient for the first falsifiable question while keeping the source-to-claim path reviewable.

## Deterministic result

2012 -> 2021:

- Japan: 3,863,530 -> 3,375,255, delta -488,275 (-12.6%).
- Tokyo: 447,113 -> 423,595, delta -23,518 (-5.3%).
- Tokyo share of the national total: 11.57% -> 12.55%, +0.98 percentage points.

The mismatch is important: Tokyo's absolute count also fell, while its relative share rose because its decline was smaller than the national decline.

These are descriptive arithmetic results. They do not identify relocation, policy, population, industry mix, openings, or closures as causal mechanisms.

## Generated artifacts

Running:

```bash
go run ./cmd/public-evidence-report
```

writes:

- `generated/normalized.csv` — normalized source rows.
- `generated/analysis.json` — deterministic metrics.
- `generated/evidence-ledger.json` — claim -> source -> calculation -> counter evidence -> limitations.
- `generated/report.md` — full public report.
- `generated/note-draft-source.md` — derivative editorial source, not a fabricated independent study.
- `generated/sns-summary.md` — short summary source.
- `generated/insight-import.csv` — existing Insight Lab generic CSV contract with `source=dataset`.

## Deterministic vs LLM boundary

Deterministic:

- parsing and schema validation
- sorting and normalization
- deltas and percentage changes
- share calculation
- source metadata
- evidence-ledger arithmetic
- rendered numeric claims

Optional model-backed Insight Lab stage:

- hypothesis generation
- alternative explanations
- question refinement
- limitation discovery assistance
- research-loop exploration

The generated public numeric report stays authoritative for the numbers even after `insight-import.csv` is analyzed with a model.

## Reproducibility

```bash
go test ./internal/publicreport
go run ./cmd/public-evidence-report
```

The unit test checks the national delta, Tokyo delta, Tokyo share movement, report sections, and ledger output.

Real NTA event ingestion continues to require the user's local `HOUJIN_APP_ID`. Model-backed Insight Lab analysis continues to require `INSIGHT_LAB_API_KEY`. Neither credential is required to regenerate this P0 public report.

## TechVit publication compatibility

`www.techvit.me` already has an Astro `writing` content schema with optional LLMO/GEO-oriented fields including:

- `question`
- `shortAnswer`
- `primaryData`
- `reviewedAt`
- `decisionCriteria`

The full report therefore fits the existing `writing` collection without a new CMS or page type. Publication should be a separate reviewed change: copy the generated report into a writing entry, add frontmatter, and link back to source/methodology. P0 does not auto-publish.

## Acceptance check

- [x] Real public data is committed as a reviewable source extract.
- [x] At least one full report is generated.
- [x] Numeric claims trace to source rows and explicit calculations.
- [x] Evidence and counter-evidence are separated.
- [x] Limitations are explicit.
- [x] Outputs are generated from the same inputs by one command.
- [x] LLM is not the numeric source of truth.
- [x] No secrets or customer/private data are included.
- [x] TechVit publication path was verified against the existing content schema.
- [ ] Model-backed Insight Lab re-analysis of `insight-import.csv` requires a configured provider and is intentionally outside the no-secret deterministic P0 run.

## Next validation

1. Expand from the five-prefecture extract to all 47 prefectures and compute distribution / concentration metrics.
2. Compare a current NTA all-record snapshot under the separate 'corporate-number entities' definition.
3. Add business births, closures, relocations, population, employment, and industry mix to test competing explanations.
4. Verify survey-definition comparability in the detailed source methodology before external publication.
