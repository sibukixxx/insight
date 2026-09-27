# Persistence and state ownership

Issue #137, following [ADR 0001](adr/0001-standalone-first-portable-deployment.md). This document inventories what Insight persists today, who owns it, which guarantees the local implementation gives, and what any future external store would have to provide. It does not add a database driver: SQLite and the local filesystem remain the reference implementation.

## Ownership rule

One coordinator process owns one SQLite database and its data directory. `app.Open` takes an exclusive advisory lock on `<db>.lock` before opening the database; a second server or headless command on the same database fails with `ENGINE_IN_USE` (CLI exit code 5) instead of sharing it. The operating system releases the lock when the process exits, so a crash never leaves a stale lock.

This is enforced because the durable queues (#132, #133) are claimed from the database: a second process would claim the first one's queued analyses and ingests without the in-memory settings (API key) they were enqueued with. Before #133 a second process was already unsafe (it failed the other process's queued and running analyses on start); the lock makes that explicit.

The lock is advisory and local. It does **not** make SQLite on a network filesystem, or several hosts writing one data directory, safe; those topologies are unsupported.

## Inventory

### (a) Research and engine state — SQLite `insight.db`

| Tables | Written by | Notes |
|---|---|---|
| `projects`, `documents` | Reference API, Public Engine, importers, ingest | `documents.ingest_id` hides rows of a non-READY ingest from every read |
| `observations`, `patterns`, `pattern_observations`, `insights`, `insight_patterns`, `evidence` | Analysis pipeline | Owned by the analysis run that produced them |
| `research_runs`, `research_iterations`, `human_evaluations` | Research loop | Append-only iterations |
| `scenario_sets`, `scenario_evaluations` | Scenario analysis | Append-only evaluations |
| `dataset_profiles`, `selection_plans` | Data triage | Immutable plan versions |
| `public_subjects`, `public_idempotent_responses` | Public Engine Contract | Idempotent replay of accepted requests |
| `engine_state` | Migration 020 | Opaque identity of this database (#116); survives restarts, differs per database |

### (b) Coordinator state — SQLite and the HEAVY state directory

| State | Store | Claim / transition | Recovery on start |
|---|---|---|---|
| Analysis queue and lifecycle (#133) | `analyses` (`status`, `failure_code`, `cancel_requested_at`, `retry_of`) | Conditional `UPDATE … WHERE status = 'queued'`; terminal writes only while `running`, completion only without a cancel request | `running` → `failed`/`INTERRUPTED`; `queued` resumes only when the current settings reproduce the execution fingerprint, else `NEEDS_REQUEUE` |
| Ingest jobs (#132) | `ingest_jobs`, `ingest_row_keys` | Conditional claim `QUEUED → VALIDATING`; READY is one conditional row update | VALIDATING requeued from the staged file after its rows are discarded (FAILED when the file is gone) |
| HEAVY partitions (#93, #134) | `-heavy-dir/<jobId>.json` | Per-partition status, attempts and result; job ID derived from raw sha256 + spec | Succeeded partitions are reused; others rerun |

Idempotency lives with the state it protects: Public Engine replays (`public_idempotent_responses`), ingest identity (partial unique index over project, kind, file hash, manifest hash, request key), one retry per failed analysis (unique `retry_of`).

### (c) Immutable input and result artifacts — filesystem

| Artifact | Location | Identity |
|---|---|---|
| Raw artifacts referenced as `file:<relative path>` | `-input-root` (read-only to Insight) | sha256 + size measured by the engine at registration and on every read |
| Staged large-CSV uploads | `<ingest-dir>/<ingestId>/upload.csv` (default `ingest/` beside the database) | sha256 measured while receiving; deleted when the ingest finishes |
| Ingest error exports | `<ingest-dir>/<ingestId>/errors.csv` | Kept with the ingest |
| Prepared Analytical Artifacts | Stored as documents in `insight.db` | Deterministic artifact ID and artifact hash |
| HEAVY shard results | Inside the partition state (b) | Result sha256 checked against its WorkSpec before merging |

Large raw data is never stored in the state database, and no queue ever carries raw bytes: a WorkSpec carries a URI, byte ranges and hashes.

### (d) Queue transport — not a persistence concern

Delivery of work to workers belongs to the execution adapter (in-process claim, PROCESS stdin/stdout, a future broker in #135). A broker is not a job store: the coordinator's state above remains the record of what is queued, running and finished.

## Transaction boundaries

The pure-Go SQLite driver runs with one connection (`SetMaxOpenConns(1)`), so every transaction blocks all other database work while it is open. Writers therefore keep transactions bounded:

- ingest batches are one transaction per bounded batch of documents; discarding rows is chunked;
- analysis progress is a single-row update; terminal transitions are single conditional updates;
- migrations run one transaction per migration file at start-up.

## Backup and restore

Stop the process (or rely on the lock to know it is stopped), then copy the data directory: `insight.db` with its `-wal`/`-shm` files, `ingest/`, and `-heavy-dir` if used. `-input-root` is the operator's own data. Restoring onto another host keeps the `engine_state` identity, which is correct: it is the same database. Queued analyses on a restored database resume only if that host's settings reproduce their execution fingerprints.

## Capabilities and errors

| Situation | Behavior |
|---|---|
| Second process on the same database | `ENGINE_IN_USE`; CLI exit 5; server exits with the message |
| Unknown `-runtime` value, or `-runtime process` without `-heavy-dir` and `-input-root` | Start-up fails with the reason |
| HEAVY requested without `-heavy-dir` | `EXECUTION_PROFILE_UNAVAILABLE`; never downgraded |
| Raw reference without `-input-root` | `INPUT_SOURCE_UNAVAILABLE` |
| Ingest staging quota or disk full | HTTP 507 |
| Analysis queue beyond its bound | HTTP 503 with `Retry-After` |
| Queued analysis whose configuration changed across a restart | `failed` / `NEEDS_REQUEUE` |

## Requirements for a future external state store

An external or shared store is added only for a documented need for several coordinators (ADR 0001). Before one is accepted it must provide, and a conformance suite (#139) must test without a cloud account:

- transactional conditional updates equivalent to the claims and terminal guards above;
- leases with expiry and a fencing token checked on every terminal write, so a stale coordinator cannot finish a run another one reclaimed;
- the same idempotency keys and uniqueness rules (ingest identity, one retry per run, Public Engine replay);
- `engine_state` identity semantics;
- configuration selected explicitly (never inferred from the presence of a broker), with the connection string and credentials supplied by environment or secret injection, never in manifests, WorkSpecs, snapshots or telemetry.

PostgreSQL is a possible opt-in adapter under these rules, not a prerequisite, and no Core start-up path provisions a database.
