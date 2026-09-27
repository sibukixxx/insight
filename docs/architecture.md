# Architecture map

One canonical picture of how Insight is used. If another document disagrees with this one, this one wins and the other should be fixed (#95).

## Canonical architecture

```text
User / App / downstream product (e.g. TechVit Insight)
          ↓
 standalone SDK (optional)        insight-sdk-go · insight-sdk-js
          ↓
 Public Engine Contract           contracts/public-engine/v1 · /api/public/v1
          ↓
      Insight OSS                 Research semantics, one engine
          │
          ├ LIGHT
          ├ STANDARD
          ├ HEAVY    (with a configured Heavy Runtime adapter)
          └ AUTO     (deterministic choice with a recorded reason)
```

- **Insight OSS** owns Research semantics: Observation, Expectation, Mismatch, competing hypotheses, supporting/counter evidence, ResearchGap / DataRequirement, validation/identification, readiness, Insight Delta, temporal/longitudinal/scenario semantics, run identity and comparison.
- **Public Engine Contract** is the language-neutral boundary (schema, fixtures, HTTP/JSON). See [public-engine-contract.md](public-engine-contract.md).
- **SDKs** are thin clients in separate repositories. They are convenient, never required: everything is reachable over HTTP/JSON.
- The engine never depends on an SDK or on any consumer repository.

## Deployment

Insight is standalone-first: the default is one Go binary with SQLite and the local filesystem, and no documented feature (including HEAVY) needs a network database, broker, Docker or cloud account. External PostgreSQL, brokers and object stores are **optional** adapters, never prerequisites, and one coordinator process owns one SQLite database. See [ADR 0001](adr/0001-standalone-first-portable-deployment.md) for state ownership, the single/multi-coordinator boundary and the support matrix.

## Input path

```text
Raw / File / Stream / Dataset / Analytical Artifact / Evidence
                         ↓
               InputSource / preparation          (#90, #92)
                         ↓
                 canonical research input
                         ↓
                   Research Engine                  (same semantics for every profile)
                         ↓
 ResearchRun / Observation / Hypotheses / Gaps / Insight Artifact
```

- `inputSources` on `addEvidence` accept `RAW_ARTIFACT` references. The engine measures sha256 and size itself; consumer-claimed hashes are only compared, never recorded as verified.
- Raw references are prepared by a declarative spec (`csv-aggregate/v1`) into an Analytical Artifact. Missing stays missing, never zero.
- Large input support does **not** mean loading arbitrary GB into memory: STANDARD streams each raw artifact and prepares several of them with bounded concurrency (results never depend on the bound), HEAVY partitions work through the Heavy Runtime port.
- For many-column datasets, [data triage](data-triage.md) produces an auditable Selection Plan before preparation. No column is silently dropped.

### Large CSV ingestion (#132)

The synchronous import and preview endpoints stay as they are for small files (the preview keeps its explicit 32 MiB limit and reports `scope: EXHAUSTIVE`). Files beyond that go through a separate, durable ingest that is independent of any Analysis Run:

```text
POST /api/projects/{id}/ingests  (raw text/csv, or multipart: kind, manifest, then file)
  → streamed to <ingest-dir>/tmp, sha256 + size measured, text sniffed, quota checked
  → immutable staged file <ingest-dir>/<ingestId>/upload.csv, job QUEUED
  → one worker: VALIDATING — same importers as the small path, bounded batches
  → READY (documents become Evidence) | FAILED | CANCELLED (rows discarded)
```

- **Atomicity.** Documents are written in bounded transactions with `documents.ingest_id` set and are hidden from every document read until the job is READY; READY is a single conditional row update. FAILED / CANCELLED and interrupted jobs discard their rows in bounded chunks, so the single SQLite connection is never held for a whole file and partial ingestion never looks like success.
- **Identity.** (project, kind, file sha256, manifest hash, optional `Idempotency-Key`) — the same bytes submitted again resolve to the live or READY receipt (HTTP 200) instead of importing Evidence twice. A FAILED or CANCELLED ingest does not block re-uploading.
- **Restart.** On start, VALIDATING jobs are requeued from their staged file after discarding their rows (or FAILED when the file is gone); leftovers of finished jobs, temp files and staging directories without a job are removed.
- **Limits.** `-ingest-max-bytes` (2 GiB), `-ingest-max-staged-bytes` (8 GiB quota over queued/running ingests → 507), `-ingest-max-rows` (10M), an 8 MiB physical-line bound and 100k analysis groups. Memory is one batch plus capped error examples; the full rejected-row list is `GET …/ingests/{ingestId}/errors.csv`.
- **Row checks.** Invalid source, empty content, malformed CSV, invalid UTF-8 and duplicate `id` within the file are rejected per row; a file with no importable row is FAILED. Missing values are never turned into zero.
- **Retention.** The staged upload is deleted when the job finishes; the error export stays with the job. The receipt previews the first documents as `scope: SAMPLE`.
- **Standalone.** The ingest directory defaults to `ingest/` beside the database (`-ingest-dir` / `INSIGHT_LAB_INGEST_DIR` override it); no extra service is needed. `GET /api/health` reports `capabilities.largeIngest`. Only the Go-supported CSV kinds (`documents`, `analysis`) are accepted; raw-reference registration stays with `RAW_ARTIFACT`.

### Analysis lifecycle (#133)

The `analyses` table is the queue of record. Workers claim the oldest `queued` row with a conditional update, so nothing about a waiting run lives only in memory except the API key it was enqueued with.

| Lifecycle | Stored status / failure code | Meaning |
|---|---|---|
| QUEUED | `queued` | Waiting; admission is bounded (`DefaultMaxQueuedAnalyses` = 256, beyond it `503` + `Retry-After`) |
| RUNNING / CANCEL_REQUESTED | `running` (+ `cancel_requested_at`) | Claimed by a worker |
| SUCCEEDED | `completed` | Results recorded |
| FAILED | `failed` / `ERROR` | The run itself failed |
| CANCELLED | `failed` / `CANCELLED` | `POST /api/analysis/{id}/cancel` |
| INTERRUPTED | `failed` / `INTERRUPTED` or `NEEDS_REQUEUE` | Stopped by a restart |

- **Legacy compatibility.** `status` keeps the four values `queued / running / completed / failed`; the Reference API adds `lifecycle`, `failureCode`, `cancelRequestedAt` and `retryOf`. The Public Engine Contract v1 is unchanged: a cancelled or interrupted run is `failed` with its reason in `error`.
- **Terminal correctness.** Every terminal write is conditional on the row still being `running`; completion additionally requires that no cancel was requested. A run cancelled while its pipeline finished is recorded as CANCELLED, never as a success, and no run is finished twice.
- **Cancel.** A queued run is cancelled at once. A running run records the request and its context is cancelled, which reaches model calls and stops the Heavy Runtime from starting or retrying partitions (a partition already scanning finishes its read). A finished run answers `409`.
- **Restart.** A run that was `running` is marked INTERRUPTED and is never resumed in place: model-backed stages may already have been called, and model calls are not exactly-once. A `queued` run resumes only when the current settings reproduce its recorded execution fingerprint (engine build, prompts, provider and models); otherwise it fails with `NEEDS_REQUEUE`. The API key is never persisted. On graceful shutdown a running run is left for this recovery rather than recorded as a failure.
- **Retry.** `POST /api/analysis/{id}/retry` enqueues a failed run again as a new analysis with the same request (label, note, semantic mode, question, reasoning profile, output locale, requested execution profile, model bindings) under the current settings. A failed run has at most one retry (`202` new, `200` existing). HEAVY preparation keeps its deterministic job ID, so a retry resumes partitions the Heavy Runtime already finished.
- **SSE.** `GET /api/analysis/{id}/events` opens with a `status` event, or the terminal event of a finished run, read after subscribing; `GET /api/analysis/{id}` stays the state of record.

## Five independent axes

| Axis | Values | Meaning |
|---|---|---|
| AnalysisMode | Discovery / Dataset Analysis / Research Review | How input is interpreted |
| ReasoningProfile | GENERAL_RESEARCH / CUSTOMER_INSIGHT | Which explicit reasoning/synthesis specialization the caller selected; GENERAL_RESEARCH is the default |
| ResearchStage | DISCOVERY / EXPLORATORY / VALIDATION / SYNTHESIS | Where the research is in its lifecycle |
| ExecutionMode | deterministic / model-backed | Whether a model is used where defined |
| ExecutionProfile | LIGHT / STANDARD / HEAVY / AUTO | Resource/runtime strategy |

They never substitute for each other. ReasoningProfile is explicit and is never inferred from source type, namespace or wording. It changes semantic/execution configuration, not evidence identity. ExecutionProfile must not change hypothesis or readiness semantics; conformance fixture `08-execution-profile-equivalence` checks this. Reasoning-profile separation is covered by fixture `18-reasoning-profiles`.

## Quickstart — direct engine use

```sh
make build
./bin/insight-lab serve -no-browser              # UI + /api + /api/public/v1 on 127.0.0.1:8787
./bin/insight-lab serve -no-web                  # API only, no Reference Web
./bin/insight-lab serve -input-root ./data -heavy-dir ./heavy   # enable raw references and HEAVY
./bin/insight-lab engine                         # headless: engine capabilities as JSON
```

Headless commands (`subject`, `evidence`, `analysis`, `research`, `status`) run the same engine in-process without HTTP or a browser; see the README "Headless research flow".

```sh
# create a subject over the public contract
curl -s -X POST http://127.0.0.1:8787/api/public/v1/subjects \
  -H 'Content-Type: application/json' \
  -d '{"contractVersion":"1","idempotencyKey":"k1","subject":{"namespace":"my-app","id":"item-1"}}'
```

## Quickstart — SDK consumer

```go
client := insight.NewClient("http://127.0.0.1:8787") // github.com/sibukixxx/insight-sdk-go
```

```ts
const client = new InsightClient({ baseUrl: "http://127.0.0.1:8787" }); // @sibukixxx/insight-sdk
```

## SDK status (#94)

- The SDK implementation started as v0 in PR #87 and was **extracted, not rewritten**, into [`insight-sdk-go`](https://github.com/sibukixxx/insight-sdk-go) and [`insight-sdk-js`](https://github.com/sibukixxx/insight-sdk-js).
- Each SDK pins a copy of `contracts/public-engine/v1` with its upstream revision recorded.
- #90 / #91 additions (InputSource, ExecutionProfile) are additive; they are not a reason to restart SDK work.

## OSS / downstream boundary

- Insight OSS supports every execution profile generically. A downstream managed product (for example TechVit Insight) is a consumer, not the owner of HEAVY semantics.
- Downstream products own customer/auth/storage, managed jobs, connectors, cost policy and product UI. Standalone users can implement their own adapters and runtime against the same contract.
- Nothing in the engine requires a downstream product.
