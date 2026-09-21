# P0 Current-Reality Audit

Audited against `main` on 2026-09-18 before P0 implementation.

## Insight Lab

### EXISTING

- `Observation`
- `Expectation` / `ExpectationBasis`
- `SurprisingFact` (Mismatch / Surprise semantics)
- Primary and competing hypotheses via `HypothesisSetID` / `HypothesisRole`
- `Evidence` with support / counter / neutral types
- grounding and deterministic quality guardrails
- confidence as evidence quality, explicitly not causal probability
- candidate causal structure with exposure / outcome / confounder / mediator / collider / unknown
- validation / causal / identification status
- append-only research runs, research gaps and data requirements
- CSV import and `dataset` source type
- Markdown research report export
- corporate-event analysis CSV adapter

### REUSABLE

- Existing domain semantics and causal guardrails
- `SourceDataset`
- Research Question → Observation → Expectation → Surprise → competing hypotheses → Evidence/Counter Evidence → gaps/validation flow
- CSV as the provider-neutral external-data boundary
- Generic corporate-event analysis CSV contract

### MISSING FOR THIS P0

- A checked-in real public dataset fixture with complete provenance
- deterministic first-report arithmetic independent of LLM output
- public answer-first report format requested for LLMO/GEO use
- claim-level Evidence Ledger from public prose back to source rows/calculations
- one-analysis → Full Report / note source / SNS summary distribution outputs
- a reproducibility test for the first public evidence report

### DO NOT TOUCH

- Core Insight domain model and existing causal semantics
- Existing SQLite migrations/repositories
- LLM prompt/pipeline behavior unrelated to the P0 public report
- Existing confidence meaning

## ja-company-base

### EXISTING

- National Tax Agency Web API client
- update-date-range fetch
- `AnalysisRecord` projection
- `ASSIGNED` / `UPDATED` / `CHANGED` / `CLOSED` event classification
- CSV / JSONL `AnalysisWriter`
- `ja-company-export` CLI
- source provider/version/fetched-at metadata
- external-acquisition credential boundary

### REUSABLE

- `AnalysisRecord` CSV is already accepted by Insight Lab without a Go module dependency
- lifecycle-event wording is already conservative and does not equate registry events with founding/bankruptcy

### MISSING / EXTERNAL BOUNDARY

- A real NTA export cannot be produced in this execution environment without the user's `HOUJIN_APP_ID` (or a separately downloaded official bulk file).
- That limitation is not a reason to block P0 because an authentication-free official aggregate source is available for the first report.

### DO NOT TOUCH

- AppID handling and logging rules
- event semantics
- public API client contract

## First Research Question

**日本の会社は本当に減っているのか？ — 2021年と2024年の経済センサスを単純比較してよいのか**

The question remains falsifiable. The implementation does not assume a decline and records an inconclusive result when the population definitions do not support a like-for-like comparison.
