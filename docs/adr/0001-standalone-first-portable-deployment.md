# ADR 0001: Standalone-first portable deployment

- Status: Accepted
- Date: 2026-09-27
- Issue: #136 (parent #131)
- Related: #132 bounded ingestion, #133 durable analysis lifecycle, #134 portable WorkSpec / PROCESS runtime, #135 optional remote runtime, #137 persistence ports, #138 packaging, #139 deployment conformance, #116 engine-state identity

## Context

Insight is an open-source Research Engine that runs locally as one Go binary with SQLite. Large-input and long-running work (#131) raised the question of how Insight scales and where it can be deployed. Without a recorded decision it is easy to drift into assumptions that are not true of the code: that production needs PostgreSQL, that Docker Compose should start a database and a broker, or that "portable" means every workload runs in every serverless sandbox.

This ADR fixes what the default is, which kinds of state exist and who owns them, where optional adapters may plug in, and what Insight will not do.

## Decision

1. **Seven concerns, not one database.** Research/engine state, durable job/coordinator state, immutable input/result artifacts, job delivery, worker placement, observability and secrets are separate concerns. None of them implies a particular database, broker or vendor for another.
2. **Standalone is the default and needs nothing else.** The default OSS deployment is the local Go binary with SQLite and the local filesystem. No network database, broker, Docker, PostgreSQL, Redis, NATS, DuckDB or cloud account is required for any documented feature, including HEAVY. Docker is a packaging option (#138), not a runtime prerequisite.
3. **SQLite and one coordinator stay first-class.** The existing SQLite persistence and single-coordinator semantics are the reference implementation, not a stepping stone. PostgreSQL is **not** the standard production or default database and is not bundled by default Compose. An external/shared state-store adapter (#137) is evaluated only against a documented need for multiple coordinators. The choice of a broker never decides the engine's database.
4. **Placement is independent of profile.** RuntimeMode LOCAL / PROCESS / DISTRIBUTED says where work runs; ExecutionProfile LIGHT / STANDARD / HEAVY / AUTO says which resource strategy is used. A capability that was requested explicitly and is unavailable fails clearly; nothing downgrades silently.
5. **No provider types leak inward.** Provider-specific queue, database, object-store, runtime or credential types never appear in Research Core, the Analytical Artifact, WorkSpec, the Public Engine Contract or the standalone SDKs. Adapters state their guarantees (delivery, consistency, transactions) and negotiate capabilities explicitly. Insight does not claim universal exactly-once semantics; model calls in particular are not exactly-once.
6. **Portable means equivalent results, not identical hosting.** Cloudflare Workers and similar sandboxes may need an external Go worker for compute-heavy operations. Portability means the same public operations produce the same results, not that every workload runs in every environment.
7. **TechVit stays outside the OSS Core.** Customer/project management, managed ingestion and aggregation, policy, reports and delivery are TechVit's private responsibilities. Its reviewed Selection Plan → sealed Analytical Artifact numeric path is never moved into the OSS Core; it reaches Insight only through the Public Contract / SDK.

## Responsibility and ownership

```text
                       ┌──────────────────────────── one coordinator process ────────────────────────────┐
 Reference Web / CLI ─▶│ HTTP API · Public Engine Contract                                                 │
 SDK / TechVit ───────▶│   │                                                                               │
                       │   ├─ Research Core (semantics, validation, readiness)                            │
                       │   ├─ Analysis coordinator (analyses table = queue of record, #133)               │
                       │   ├─ Ingest coordinator (ingest_jobs, #132)                                       │
                       │   └─ HEAVY planner + Heavy Runtime (partition state, #134) ──▶ Dispatcher ─┐      │
                       │                                                                           │      │
                       │   SQLite insight.db  ·  ingest/ staging  ·  heavy-dir state                │      │
                       └───────────────────────────────────────────────────────────────────────────┼──────┘
                                                                                                   ▼
                                     LOCAL: in-process │ PROCESS: child `insight-lab worker` │ DISTRIBUTED: optional adapter (#135, not implemented)
                                                        (reads only -input-root; no secrets)
```

| Concern | Owner today | Default implementation | Optional adapter point |
|---|---|---|---|
| Research / engine state (projects, documents, analyses, research runs, engine-state identity #116) | Coordinator | SQLite `insight.db` | External state store only for a documented multi-coordinator need (#137) |
| Durable job / coordinator state | Coordinator | `analyses` rows (#133), `ingest_jobs` rows (#132), `-heavy-dir` partition JSON (#134) | Same store as research state; a broker is not a job store |
| Immutable input and result artifacts | Coordinator | `-input-root` raw files, `ingest/` staged uploads, prepared Analytical Artifacts stored as documents | Shared object/file store for remote workers (#135); referenced by URI + sha256, never carried by the broker |
| Job delivery | Coordinator | In-process claim of queued rows; PROCESS: WorkSpec on the child's stdin | Broker adapter carrying references and metadata only (#135) |
| Worker placement | Operator (`-runtime`) | LOCAL (default), PROCESS (opt-in) | DISTRIBUTED (#135) |
| Observability | Operator | Process logs on stdout/stderr; state of record via `GET` endpoints | Operator-chosen log/trace collection; no bundled telemetry backend |
| Secrets | Operator | Flags or environment (`INSIGHT_LAB_API_KEY`), held in memory only | Secret injection by the platform; never in the database, snapshots, WorkSpec, queue payloads, SSE or logs |

## Single- and multi-coordinator boundary

- **Supported:** exactly one coordinator process per SQLite database and data directory. It may use PROCESS workers on the same host, which read only the configured input root.
- **Not supported:** two coordinators sharing one SQLite file, SQLite on a shared network filesystem, or several hosts writing the same `ingest/` or `-heavy-dir`. These are multi-writer topologies SQLite and the local filesystem do not make safe; configuration validation that rejects them is tracked in #137.
- **Future:** multiple coordinators require a transactional shared job/state store with leases and fencing. That store is an explicit, configured adapter (#137); it is not implied by adding remote workers.

## Support matrix

"Verified" means covered by tests in this repository. Anything else is a design target, not a support claim.

| Mode | Status | What the operator supplies | Persistent state | Credentials | Known limits |
|---|---|---|---|---|---|
| Local binary (`insight-lab serve`) | Verified | A host with the binary | Data directory: `insight.db`, `ingest/`; optional `-heavy-dir` | Optional model API key (flag/env) | One coordinator per database |
| Local binary, PROCESS runtime | Verified (#134) | Same, plus `-runtime process -heavy-dir -input-root` | As above plus `-heavy-dir` | None passed to workers | Workers on the same host; per-partition process isolation, timeout and output limit |
| Single container (`compose.yaml`) | Verified by `make docker-smoke` (#138) | A container runtime and one named volume for the data directory | The volume | Optional model API key via env/secret | Image is built locally; no published registry image yet |
| On-premises | Verified as the local binary on the operator's host | Host, backups, access control, TLS termination if exposed | Data directory on local disk | As local binary | No built-in authentication (only a loopback-origin check for browsers); keep the default localhost bind or front it with the operator's authenticating proxy |
| BYO cloud compute (VM / container service on AWS, Google Cloud, Azure, others) | Not verified | Compute that runs the binary/container with a persistent local volume | That volume (not ephemeral container storage) | Platform secret injection | Same one-coordinator rule; serverless sandboxes without a persistent volume or long-running processes are unsuitable for the coordinator |
| Cloudflare Workers and similar sandboxes | Not supported for the coordinator | — | — | — | May host an external client or a future external worker; compute-heavy operations need a Go worker elsewhere (Decision 6) |
| Optional distributed workers | Not implemented (#135) | Broker and shared artifact store of the operator's choice | Coordinator state as above; artifacts in the shared store | Adapter credentials only; never LLM keys | Opt-in; an explicit distributed request without a broker fails |

## How the follow-up issues implement this decision

| Issue | Role in this decision | State |
|---|---|---|
| #132 | Large CSV ingestion owns its staging directory beside the database and its `ingest_jobs` rows; nothing becomes Evidence until READY | Backend merged (#142); UI pending |
| #133 | The `analyses` table is the queue of record; cancel, retry and restart recovery are durable, the API key is never persisted | Backend merged (#143); UI pending |
| #134 | Versioned WorkSpec with an allowlisted operation; LOCAL and PROCESS placement; workers get no secrets and read only the input root | Merged (#144) |
| #135 | DISTRIBUTED placement as an opt-in adapter: broker carries references, artifacts live in a shared store, at-least-once with fencing | Not started; depends on this ADR |
| #137 | Persistence ports: separates research state, coordinator state and artifact refs; rejects unsafe multi-writer configurations | One-coordinator lock and [persistence.md](../persistence.md) merged (#146); external store not needed yet |
| #138 | Packaging: single-container default and opt-in Compose profiles that never start a database or broker by default | Implemented ([deployment.md](../deployment.md)) |
| #139 | Deployment conformance: the support matrix above becomes test-backed per adapter | Not started |
| TechVit #66 / #67 | Managed job ↔ Core run correlation and distributed governance on the TechVit side, through the Public Contract only | Other repository |

## Consequences

- The standalone path stays the one every feature is built and tested against first; optional adapters are added behind explicit configuration and their own conformance results (#139).
- Scaling within one host uses PROCESS workers; scaling beyond one host needs #135, and multiple coordinators additionally need #137. Neither is implied by the other.
- Operators own backup and restore of the data directory; there is no managed database to rely on.
- Documentation must not claim provider support that has not been verified.

## Non-goals

Installing every cloud adapter, changing the Research Core model, requiring Kubernetes, replacing SQLite pre-emptively, and introducing a second analytics engine.
