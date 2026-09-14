# ja-company-base Dogfooding

This workflow tests the boundary:

`ja-company-base AnalysisRecord CSV → deterministic aggregate Dataset Documents → Insight pipeline`

Insight Lab does not import or depend on the `ja-company-base` Go module. The integration contract is CSV only.

## What the adapter does

The adapter groups valid rows by:

- event month;
- prefecture and city names;
- `event_type`;
- source provider and version.

It creates one `dataset` Document per group. The content reports `record_count` and an explicit semantic warning: the count is the number of exported administrative records, not a count of startups, business commencements, policy effects, or causal impacts.

The adapter does not calculate an increase, select a baseline, or invent an expected value. Those questions require multiple periods and remain subject to the ordinary Observation grounding and causal guardrails.

## Reproducible run

### 1. Export administrative records

From `ja-company-base`:

```bash
HOUJIN_APP_ID=... go run ./cmd/ja-company-export \
  -from 2026-01-01 \
  -to 2026-06-30 \
  -prefecture 13 \
  -format csv \
  -output companies.csv
```

Use multiple months. A one-day or one-month export cannot establish a temporal mismatch.

### 2. Start Insight Lab

```bash
INSIGHT_LAB_API_KEY=... go run ./cmd/insight-lab
```

Create a project in the browser, then use **Import ja-company analysis CSV**. The equivalent HTTP request is:

```bash
curl -F file=@companies.csv \
  http://127.0.0.1:8787/api/projects/PROJECT_ID/documents/import/analysis
```

The response distinguishes source rows from generated aggregate Documents:

```json
{"recordsRead":120,"imported":8,"skipped":0,"errors":[]}
```

### 3. Run analysis and inspect the report

Run analysis from the project screen. The report should preserve this distinction:

```text
Observed: exported ASSIGNED record count differs between periods
Not established: startup count increased
Not established: a municipal policy caused the difference
Identification: NOT_IDENTIFIED
```

## Acceptance checks

- Dataset Documents show the same counts as an independent CSV group-by.
- `SourceType` is `dataset`, never a disguised interview or survey.
- An `ASSIGNED` count is not rewritten as “new businesses” or “startups.”
- At least three explanations are independently evaluated when meaningful.
- Supporting evidence for one hypothesis is not automatically copied to another.
- Association-only output remains `NOT_IDENTIFIED`.
- Missing periods, control regions, population, economic conditions, policy timing, and source-schema changes are visible as missing evidence when relevant.

## Current operational limits

- Real export requires the user's `HOUJIN_APP_ID`.
- Real LLM analysis requires `INSIGHT_LAB_API_KEY`.
- The first dogfood uses CSV. JSONL is deliberately deferred.
- The adapter trusts `event_type` classification supplied by the exporter and preserves its provider/version for audit. It does not reinterpret source events.
