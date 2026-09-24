# Public Engine Contract v1

`insight-lab.public-engine` v1 is the language-neutral, domain-neutral boundary that downstream consumers use instead of Insight's internal packages. It covers Issues #59, #76 and #77.

The single source of truth is [`contracts/public-engine/v1/schema.json`](../contracts/public-engine/v1/schema.json). The server wire types, the Go SDK and the TypeScript SDK are all checked against its `$defs` by drift tests.

```text
consumer ──> Go SDK / TypeScript SDK ──> HTTP/JSON /api/public/v1 ──> Insight engine
                  (thin, no logic)          (this contract)            (research semantics)
```

## Transport

HTTP/JSON under `/api/public/v1` on the Insight Lab server. Both SDKs use this transport and nothing else.

| Operation | Method and path | Success |
|---|---|---|
| getEngine | `GET /engine` | 200 `EngineInfo` |
| createSubject | `POST /subjects` | 201 new, 200 existing `Subject` |
| addEvidence | `POST /subjects/{subjectId}/evidence` | 200 `EvidenceReceipt` |
| startAnalysis | `POST /subjects/{subjectId}/analyses` | 202 `AnalysisRun` |
| getAnalysis | `GET /subjects/{subjectId}/analyses/{analysisId}` | 200 `AnalysisRun` |
| getAnalysisResults | `GET /subjects/{subjectId}/analyses/{analysisId}/results` | 200 `AnalysisResults` |
| createResearchRun | `POST /subjects/{subjectId}/research-runs` | 201 `ResearchResult` |
| appendIteration | `POST /research-runs/{researchRunId}/iterations` | 201 `ResearchResult` |
| getResearchRun | `GET /research-runs/{researchRunId}` | 200 `ResearchResult` |

Every request and response carries `contractVersion`. Mutating requests also carry an `idempotencyKey`.

## Semantics

- **Subject.** A subject is an opaque external reference `{namespace, id, type}` that owns evidence. Insight stores and echoes it and never branches on its namespace or type. A commerce-like consumer and a public-data consumer get the same generic research output.
- **Evidence.** A document is identified by its `externalRef` within its subject. An Analytical Artifact is identified by its `id`. Resending identical content returns `UNCHANGED`. Different content under the same identity is `IDENTITY_CONFLICT`. A request is all-or-nothing.
- **Analysis run.** An analysis run reads the subject's current evidence. It records the execution and input snapshots and fingerprints from #82, which the contract returns verbatim in `provenance`. Results are always scoped to one run.
- **Research run.** Research is built from one explicitly named, completed analysis run. `appendIteration` evaluates the same question on a newer completed run and records which added evidence targets which gap. Earlier iterations are never modified.
- **Result.** `ResearchResult.artifact` is the existing `insight-lab.research-artifact` v1 export, embedded verbatim rather than re-modelled. It carries hypotheses, supporting and counter evidence, gaps, data requirements, what cannot be concluded, readiness, the Insight Delta and provenance. It contains no consumer action such as buy, sell, launch or stop.

## Idempotency and identity

- The first successful response to an idempotency key is stored and replayed verbatim for every retry with the same request.
- The same key with a different request is `IDEMPOTENCY_CONFLICT`.
- Failed attempts are not stored, so they can be retried with the same key.
- Natural identities (subject reference, document `externalRef`, artifact `id`) are also idempotent across keys, as described above.
- Both SDKs generate a key when the caller omits one. Pass your own key and reuse it when you retry.

## Errors

Errors use `{"contractVersion":"1","error":{"code","message"}}`.

| Code | HTTP |
|---|---|
| `INVALID_REQUEST` | 400 |
| `UNSUPPORTED_CONTRACT_VERSION` | 400 |
| `NOT_FOUND` | 404 |
| `IDEMPOTENCY_CONFLICT` | 409 |
| `IDENTITY_CONFLICT` | 409 |
| `ANALYSIS_NOT_COMPLETED` | 409 |
| `ANALYSIS_HAS_NO_HYPOTHESES` | 409 |
| `MIXED_ANALYSIS_RUNS` | 409 |
| `INTERNAL` | 500 |

The SDKs add `UNAVAILABLE` for an unreachable engine or a non-contract response. They raise `UNSUPPORTED_CONTRACT_VERSION` themselves when a response uses a version they do not speak. Internal error details are never returned.

## Versioning and compatibility

- The server accepts only versions listed in `EngineInfo.supportedContractVersions`. Any other version fails explicitly.
- Adding an optional field is additive and does not change the version. Receivers ignore unknown fields.
- Renaming or removing a field, changing its meaning, or adding a required field needs a new contract version.
- The embedded research artifact keeps its own `artifactSchema` and `schemaVersion`. Its legacy `analysisMode` key means execution mode. The contract itself uses `executionMode`.
- Go SDK tags follow `sdk/go/vX.Y.Z`. The npm package declares supported contract versions in `insightContractVersions`.

## Limits

The request body is at most 16 MiB. A request carries at most 500 documents and 50 artifacts. Document content is limited to 200,000 characters and a question to 2,000 characters. Metadata is at most 32 string entries with keys matching `^[A-Za-z0-9._:-]{1,64}$` and values up to 1,024 characters. Document metadata keys starting with `public_` or `analytical_` are reserved.

## Decisions

### Packaging (#76, #77)

- **Go SDK: module `github.com/sibukixxx/insight/sdk/go`, nested in this repository.** It has no dependencies and cannot import Insight's `internal/*` packages because it is a different module. The core module path is `insight-lab`, which cannot be fetched with `go get`, so a nested module with a hosted path is the only way to make the SDK installable without renaming the core. Split into its own repository only if its release cadence diverges from the contract.
- **TypeScript SDK: package `@sibukixxx/insight-sdk` in `sdk/node`.** Types are generated from the schema by `scripts/generate-types.mjs`. The source uses erasable TypeScript only, so Node 22.18+ runs it directly and `tsc` builds `dist/` for publishing. The package is not published yet.
- **Why stay in the core repository.** Keeping both SDKs next to the schema lets one change update the contract, the server and both SDKs, with the drift checks failing on any mismatch. The constraint that matters is independent versioning without semantic drift, not repository count.

### Single source of truth and drift checks

- `internal/publicengine/drift_test.go` checks server wire types against the schema.
- `sdk/go/drift_test.go` checks the Go SDK against the schema.
- `sdk/node/test/drift.test.ts` fails when `src/contract.gen.ts` is stale or a fixture uses an unknown operation or error code.

### Conformance

- `contracts/public-engine/v1/fixtures/` holds the seven #59 scenarios:
  1. generic public-data subject
  2. commerce-like opaque subject
  3. deterministic Analytical Artifact
  4. missing evidence to added evidence to a new iteration
  5. counter-evidence
  6. contract version mismatch
  7. idempotent requests
- `internal/http/public_conformance_test.go` starts a deterministic engine and a model-backed engine. It runs every fixture through the Go SDK, then runs the Node SDK suite against the same engines.
- The model-backed engine in that test uses a scripted stand-in model defined only in the test. Production code has no fake-model mode. Fixtures 01, 02, 04 and 05 check contract behavior with that model. They do not measure the quality of a real model.

### Analytical Artifact boundary (#69)

- An artifact is stored as one `dataset` document: one line per result statement, with the artifact and its identity in metadata.
- Deterministic pre-analysis grounds each result statement and turns it into an observation, without a model (rule `analytical-artifact/v1`).
- A result stays a calculation output to interpret. It is never a hypothesis, claim or insight, and no evidence rows are created from it.
- A deterministic run over artifacts has no hypotheses, so research on it returns `ANALYSIS_HAS_NO_HYPOTHESES`. Research needs a model-backed run.

### Re-evaluation (#74) scope

- `appendIteration` covers the evaluation step of #74. It evaluates a named completed run in a new append-only iteration, links added evidence to gaps, and records the Insight Delta.
- The rest of #74 is out of scope here: evidence-delta detection, partial re-evaluation, affected-scope tracking and scheduler invocation.

## Non-goals

- Domain-specific operations such as EvaluateProduct.
- Consumer actions or recommendations.
- Scheduler or orchestration.
- Authentication for multi-tenant deployment. The server binds to localhost by default and rejects cross-origin browser requests.
- Streaming progress. Both SDKs poll.
