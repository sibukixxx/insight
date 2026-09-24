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
| listAnalyses | `GET /subjects/{subjectId}/analyses` | 200 `AnalysisList` |
| compareAnalyses | `GET /subjects/{subjectId}/analyses/{analysisId}/compare/{otherAnalysisId}` | 200 `RunComparisonResult` |
| listResearchRuns | `GET /subjects/{subjectId}/research-runs` | 200 `ResearchRunList` |
| reEvaluate | `POST /research-runs/{researchRunId}/re-evaluations` | 201 new iteration, 200 otherwise `ReEvaluationResult` |
| getResearchTimeline | `GET /research-runs/{researchRunId}/timeline` | 200 `ResearchTimeline` (#71) |
| applyTemporalOperation | `POST /temporal-operations` | 200 `TemporalOperationResult` (#73, stateless) |

Every request and response carries `contractVersion`. Mutating requests also carry an `idempotencyKey`.

## Semantics

- **Subject.** A subject is an opaque external reference `{namespace, id, type}` that owns evidence. Insight stores and echoes it and never branches on its namespace or type. A commerce-like consumer and a public-data consumer get the same generic research output.
- **Evidence.** A document is identified by its `externalRef` within its subject. An Analytical Artifact is identified by its `id`. Resending identical content returns `UNCHANGED`. Different content under the same identity is `IDENTITY_CONFLICT`. A request is all-or-nothing.
- **Analysis run.** An analysis run reads the subject's current evidence. It records the execution and input snapshots and fingerprints from #82, which the contract returns verbatim in `provenance`. Results are always scoped to one run.
- **Research run.** Research is built from one explicitly named, completed analysis run. `appendIteration` evaluates the same question on a newer completed run and records which added evidence targets which gap. Earlier iterations are never modified.
- **Run comparison (#83).** `compareAnalyses` compares two runs of one subject. Each of the input and execution axes is `SAME`, `CHANGED` or `UNKNOWN`; a run without a recorded snapshot or fingerprint is `UNKNOWN`, never `SAME`. The attribution is `SAME_CONFIGURATION`, `EXECUTION_CHANGE`, `INPUT_CHANGE`, `CONFOUNDED` or `ATTRIBUTION_UNAVAILABLE`. The result also carries a field-level execution diff, the input document and dataset diff, numeric metric deltas, repeat groups of runs with identical fingerprints (with metric ranges; a single run has unknown variation), and insight matches with their method (`EXACT_EVIDENCE_SPANS`, `EVIDENCE_SPAN_OVERLAP`, `HYPOTHESIS_COMPARISON_KEY`) and score. It never names a winner or ranks runs, and its explanation always starts with "Differences between runs are not evidence of cause". When an appended iteration's run used a different or unrecorded execution than the previous iteration's run, its Insight Delta explanation records that the delta is confounded with an instrument change.
- **Re-evaluation (#74).** `reEvaluate` is called by an external scheduler or a person after new evidence arrived and a new analysis run completed. It carries a `correlationKey`, the `previousIterationId` it is based on, a trigger (`MANUAL` or `SCHEDULED`), the added/removed/changed evidence refs and optional affected gap IDs. The engine appends one new iteration with an audit record (affected gaps and hypotheses, input and execution fingerprints before and after, scope) and returns `NEW_ITERATION`. A known, unchanged input fingerprint returns `NO_EVIDENCE_CHANGE` and appends nothing. The same correlation key with the same analysis returns `ALREADY_EVALUATED`; with another analysis it is `IDEMPOTENCY_CONFLICT`. A previous iteration that is not the latest is `STALE_ITERATION`. Earlier iterations are never modified. Partial re-evaluation is not yet proven safe, so the scope is always `FULL`.
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
| `STALE_ITERATION` | 409 |
| `EXECUTION_PROFILE_UNAVAILABLE` | 422 |
| `INPUT_SOURCE_UNAVAILABLE` | 422 |
| `INPUT_VERIFICATION_FAILED` | 400 |
| `INTERNAL` | 500 |

The SDKs add `UNAVAILABLE` for an unreachable engine or a non-contract response. They raise `UNSUPPORTED_CONTRACT_VERSION` themselves when a response uses a version they do not speak. Internal error details are never returned.

## Versioning and compatibility

- The server accepts only versions listed in `EngineInfo.supportedContractVersions`. Any other version fails explicitly.
- Adding an optional field is additive and does not change the version. Receivers ignore unknown fields.
- Renaming or removing a field, changing its meaning, or adding a required field needs a new contract version.
- The embedded research artifact keeps its own `artifactSchema` and `schemaVersion`. Its legacy `analysisMode` key means execution mode. The contract itself uses `executionMode`.
- Standalone SDKs pin the contract version and record the upstream insight commit of the vendored schema/fixtures. They declare the contract versions they speak.

## Limits

The request body is at most 16 MiB. A request carries at most 500 documents and 50 artifacts. Document content is limited to 200,000 characters and a question to 2,000 characters. Metadata is at most 32 string entries with keys matching `^[A-Za-z0-9._:-]{1,64}$` and values up to 1,024 characters. Document metadata keys starting with `public_` or `analytical_`, and the keys `dataset_hash` and `acquisition_manifest`, are reserved. The engine records that provenance itself, so a consumer cannot claim a file hash or acquisition manifest that the engine would then report as verified.

## Decisions

### Packaging (#59, #94)

- This repository owns the contract, schema, fixtures and server. It contains **no SDK implementation** and never depends on an SDK repository.
- **Go SDK:** standalone repository [`sibukixxx/insight-sdk-go`](https://github.com/sibukixxx/insight-sdk-go), module `github.com/sibukixxx/insight-sdk-go`.
- **TypeScript SDK:** standalone repository [`sibukixxx/insight-sdk-js`](https://github.com/sibukixxx/insight-sdk-js).
- Both started as the v0 implementation from PR #87 (`feat/public-engine-sdk`) and were extracted rather than rewritten. SDKs are optional: the engine is fully usable through this HTTP contract alone.

### Single source of truth and drift checks

- `internal/publicengine/drift_test.go` checks server wire types against the schema.
- Each SDK repository runs its own drift check against its vendored copy of `schema.json`.

### Conformance

- `contracts/public-engine/v1/fixtures/` holds the #59 scenarios:
  1. generic public-data subject
  2. commerce-like opaque subject
  3. deterministic Analytical Artifact
  4. missing evidence to added evidence to a new iteration
  5. counter-evidence
  6. contract version mismatch
  7. idempotent requests
  10. run comparison (#83)
  11. re-evaluation (#74)
  8. same research results across LIGHT / STANDARD / HEAVY (`08-execution-profile-equivalence`)
  9. raw/ref input path (`09-raw-artifact-input`)
- Fixtures 08 and 09 need an engine started with an input root containing `fixtures/data` and, for 08, a HEAVY adapter (`-input-root` and `-heavy-dir`).
- `internal/http/public_conformance_test.go` starts a deterministic engine and a model-backed engine. It runs every fixture over plain HTTP with `internal/publicengine/conformance`. SDK repositories run the same fixture files against a live engine or recorded responses.
- The model-backed engine in that test uses a scripted stand-in model defined only in the test. Production code has no fake-model mode. Fixtures 01, 02, 04 and 05 check contract behavior with that model. They do not measure the quality of a real model.

### Analytical Artifact boundary (#69)

- An artifact is stored as one `dataset` document: one line per result statement, with the artifact and its identity in metadata.
- Deterministic pre-analysis grounds each result statement and turns it into an observation, without a model (rule `analytical-artifact/v1`).
- A result stays a calculation output to interpret. It is never a hypothesis, claim or insight, and no evidence rows are created from it.
- A deterministic run over artifacts has no hypotheses, so research on it returns `ANALYSIS_HAS_NO_HYPOTHESES`. Research needs a model-backed run.

### InputSource / RawArtifact (#90)

- Inputs have three kinds, advertised in `EngineInfo.inputSourceKinds`: inline documents (`documents`), prepared Analytical Artifacts (`analyticalArtifacts`) and references to raw bytes (`inputSources` with kind `RAW_ARTIFACT`).
- A raw reference names `uri` and `mediaType`, optionally `name`, `version`, `sizeBytes`, `sha256`. The size and hash are claims. The engine streams the bytes once when the reference is added, measures sha256 and size itself and records only its own measurement. A claim that does not match is `INPUT_VERIFICATION_FAILED`. The receipt says `verifiedBy: "engine"`.
- URIs are read through resolvers configured on the engine. Core ships one: `file:<relative path>` under the directory given by `-input-root` (`INSIGHT_LAB_INPUT_ROOT`). Absolute paths, `..` and unknown schemes are `INPUT_SOURCE_UNAVAILABLE`, and host paths are never echoed. Object stores or other sources are adapters outside Core.
- Raw bytes are never stored in SQLite and never loaded whole into memory. The stored document holds the verified reference in reserved `public_raw_*` metadata; a caller cannot set those keys. Its text is a descriptor and is never analyzed.
- With `preparation` (kind `csv-aggregate/v1`: metrics, dimension columns, a period column or a fixed period, and a population), an analysis on STANDARD or HEAVY prepares the bytes into an Analytical Artifact before the pipeline runs. Memory grows with the number of groups, not rows. Sums are exact rationals, so partition order cannot change a value. Missing values stay missing, never zero. The bytes are hashed again during preparation; if they changed since registration the run fails.
- The prepared artifact id is derived from the raw sha256 and the spec, so preparing again (after a restart, or on another profile) finds the existing artifact instead of creating a second one. Its document links back with `public_prepared_from`.
- Without `preparation` a reference is recorded for provenance only (`preparation: "NOT_REQUESTED"`).
- The input snapshot fingerprints the reference document, including the verified hash, so a changed raw artifact changes `inputFingerprint`.

### ExecutionProfile (#91, #93)

- `StartAnalysisRequest.executionProfile` is `LIGHT`, `STANDARD`, `HEAVY` or `AUTO` (default). It is a resource/runtime strategy, separate from `semanticAnalysisMode` (how input is read), research stage and `executionMode` (deterministic or model-backed).
- LIGHT runs in-process on inline documents and prepared artifacts only; it refuses raw references that still need preparation. STANDARD additionally streams and prepares raw references in-process. HEAVY prepares them through the Heavy Execution Adapter: partitions with persistent job identity, per-partition progress, retry and cancellation, resumable after a restart. LIGHT never depends on that adapter.
- AUTO resolves deterministically from input shape (document count, inline bytes, raw bytes to prepare). It never downgrades: if the chosen profile is not available, the request fails with `EXECUTION_PROFILE_UNAVAILABLE`. Explicit HEAVY without a configured adapter fails the same way.
- `AnalysisRun.executionProfile` reports `requested`, `resolved`, `reason` and `strategyVersion`. The same record is in `provenance.execution`. It is deliberately not part of the execution fingerprint, because a profile changes how inputs are prepared and executed, never what the results mean.
- All profiles run the same canonical pipeline over the same prepared input. Fixture 08 checks that STANDARD, HEAVY and LIGHT (given the equivalent prepared artifact) produce identical observations.
- `EngineInfo.executionProfiles` lists each profile with `available`. HEAVY is available only when the engine is started with `-heavy-dir` (and `-input-root`). The local adapter persists job state as JSON in that directory; distributed runtimes implement the same `execution.Runtime` port.

### Re-evaluation (#74) scope

- `reEvaluate` records evidence changes, affected scope and fingerprints and appends one audited iteration; `appendIteration` remains the plain evaluation step.
- Out of scope: built-in scheduling, notifications, automatic publishing and partial re-evaluation. Scenario expectation evaluation will attach to the same record once Scenario semantics (#66) exist.

## Non-goals

- Domain-specific operations such as EvaluateProduct.
- Consumer actions or recommendations.
- Scheduler or orchestration.
- Authentication for multi-tenant deployment. The server binds to localhost by default and rejects cross-origin browser requests.
- Streaming progress. Both SDKs poll.

## Model bindings (#65 extension point)

Routing policy (which model for which stage, cost budgets, escalation) belongs to consumers. The engine exposes only a minimal, provider-neutral extension point:

- `EngineInfo.modelRouting` lists the pipeline `stages` a caller may bind and the `allowedModels` the operator configured (`-allowed-models` / `INSIGHT_LAB_ALLOWED_MODELS`; the configured `-model` is always allowed).
- `startAnalysis.modelBindings` maps stages to allowed models. Unknown stage → `INVALID_REQUEST`; a model the operator did not allow, or no model endpoint → `MODEL_BINDING_UNAVAILABLE` (422).
- Bindings are execution configuration: they are recorded per stage in `provenance.execution.llm.models`, change the execution fingerprint (so run comparison attributes the difference to execution), and never change research semantics.
- Conformance: `17-model-bindings`.
