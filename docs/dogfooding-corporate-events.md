# Corporate-event CSV Dogfooding

This workflow tests the boundary:

`external corporate-event CSV → deterministic aggregate Dataset Documents → Insight pipeline`

Insight Lab does not depend on any source-specific exporter repository. The integration contract is a normalized corporate-event CSV.

## What the adapter does

The adapter groups valid rows by:

- event month;
- prefecture and city names;
- `event_type`;
- source provider and version.

It creates one `dataset` Document per group. The content reports `record_count` and an explicit semantic warning: the count is the number of exported administrative records, not a count of startups, business commencements, policy effects, or causal impacts.

The CSV adapter itself does not calculate an increase, select a baseline, or invent an expected value; it only aggregates rows. Given multiple periods, the pipeline's deterministic dataset pre-analysis (Issue #16, see below) computes delta, rate of change, baseline delta and share of series total from `record_count` — with no model involved and no LLM key required. Everything past that (is the change meaningful, what caused it) remains subject to the ordinary Observation grounding and causal guardrails.

## Deterministic pre-analysis (no LLM required)

Running analysis on a project of `dataset` Documents alone — no interview/review/survey text — works even with **no LLM configured on the Settings page**. The pipeline:

1. turns every dataset Document with a numeric `record_count` into a grounded Observation (the quote is the document's own first sentence, verified exactly like a model-proposed quote would be);
2. groups Documents into series by event type, location, provider and (if an acquisition manifest is attached) unit/population scope, and computes delta, rate of change, baseline delta and share of series total for every consecutive period pair;
3. persists the result and stops — no hypotheses, no narrative, because generating those requires a model.

The analysis metrics record `provenance.mode: "deterministic"` in this case, and record `"model_backed"` with the model name and a prompt fingerprint once an LLM is configured. A project with no dataset Documents and no LLM configured fails with a message naming both remedies.

### Attaching an acquisition manifest

Both import endpoints accept an optional `manifest` multipart field: a JSON [acquisition manifest](../internal/service/manifest.go) recording where the file came from (source, dataset id, retrieval method/time, geography, period, unit, population scope, schema id/version, recipe reference). The importer computes the file's SHA-256 and attaches both the hash and the manifest to every Document it creates, and rejects a manifest containing anything that looks like a credential (`api_key`, `token`, `appId`, …) before importing any row.

```bash
curl -F file=@companies.csv \
  -F manifest='{"sourceName":"external-registry-export","datasetId":"export-2026-01","retrievalMethod":"user_provided","retrievedAt":"2026-03-01T00:00:00Z","schemaId":"corporate-event-analysis-csv","unit":"administrative records"}' \
  http://127.0.0.1:8787/api/projects/PROJECT_ID/documents/import/analysis
```

Datasets in the same project whose manifests declare a different unit, population scope, period granularity or schema version surface as `compatibilityWarnings` in the run's provenance and in the `## Run provenance` section of the exported report — a warning, not a silent rejection, since harmonizing populations (Issue #12) is legitimate work that must stay visible.

## Reproducible run

### 1. Prepare administrative-event records

Use any external exporter or adapter that produces the required CSV contract. For example:

```bash
external-exporter --from 2026-01-01 --to 2026-06-30 --format csv --output companies.csv
```

Use multiple months. A one-day or one-month export cannot establish a temporal mismatch.

### 2. Start Insight Lab

```bash
INSIGHT_LAB_API_KEY=... go run ./cmd/insight-lab
```

`INSIGHT_LAB_API_KEY` is only required for hypothesis generation and report narrative; the deterministic pre-analysis below runs without it. Create a project in the browser, then use **Import analysis CSV**. The equivalent HTTP request is:

```bash
curl -F file=@companies.csv \
  http://127.0.0.1:8787/api/projects/PROJECT_ID/documents/import/analysis
```

The response distinguishes source rows from generated aggregate Documents:

```json
{"recordsRead":120,"imported":8,"skipped":0,"errors":[],"fileHash":"…sha256…"}
```

### 3. Run analysis and inspect the report

Run analysis from the project screen. With an LLM configured, the report should preserve this distinction:

```text
Observed: exported ASSIGNED record count differs between periods
Not established: startup count increased
Not established: a municipal policy caused the difference
Identification: NOT_IDENTIFIED
```

Without an LLM configured, the report's `## Run provenance` section instead shows `mode: deterministic`, the dataset file hashes, and the computed delta/rate/baseline/share for each period pair — no hypotheses or narrative.

## Acceptance checks

- Dataset Documents show the same counts as an independent CSV group-by.
- `SourceType` is `dataset`, never a disguised interview or survey.
- An `ASSIGNED` count is not rewritten as “new businesses” or “startups.”
- At least three explanations are independently evaluated when meaningful.
- Supporting evidence for one hypothesis is not automatically copied to another.
- Association-only output remains `NOT_IDENTIFIED`.
- Missing periods, control regions, population, economic conditions, policy timing, and source-schema changes are visible as missing evidence when relevant.
- A project of only dataset Documents analyzes successfully with no LLM configured, and its report states `mode: deterministic`.
- Datasets with mismatched unit, population scope, period granularity or schema version surface as `compatibilityWarnings` rather than being silently combined.

## Current operational limits

- Real exports may require credentials in the external acquisition layer; those credentials never belong in Insight Lab metadata or source control.
- Deterministic pre-analysis (counts, period comparisons, provenance) needs no LLM key at all. Hypothesis generation, evidence retrieval and report narrative still require `INSIGHT_LAB_API_KEY`.
- The first dogfood uses CSV. JSONL is deliberately deferred.
- The adapter trusts `event_type` classification supplied by the exporter and preserves its provider/version for audit. It does not reinterpret source events.
- The acquisition manifest is supplied by whoever exports the file; Insight Lab does not fetch external statistical APIs or registries itself and cannot verify a manifest's claims beyond rejecting embedded credentials and internal contradictions.
