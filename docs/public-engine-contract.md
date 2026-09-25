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
| getScenarios | `GET /research-runs/{researchRunId}/scenarios` | 200 `ScenarioAnalysis` (#66) |
| createScenarioSet | `POST /research-runs/{researchRunId}/scenario-sets` | 201 `ScenarioSetResult` (#66) |
| scaffoldScenarioSet | `POST /research-runs/{researchRunId}/scenario-sets/scaffold` | 201 `ScenarioSetResult` (#66) |
| evaluateScenarios | `POST /research-runs/{researchRunId}/scenario-sets/{scenarioSetId}/evaluations` | 201 `ScenarioEvaluationResult` (#66) |
| createDatasetProfile | `POST /subjects/{subjectId}/dataset-profiles` | 201 `DatasetProfile` (#92) |
| getDatasetProfile | `GET /subjects/{subjectId}/dataset-profiles/{profileId}` | 200 `DatasetProfile` (#92) |
| triage | `POST /subjects/{subjectId}/dataset-profiles/{profileId}/triage` | 201 `SelectionPlan` (#92) |
| listSelectionPlans | `GET /subjects/{subjectId}/dataset-profiles/{profileId}/selection-plans` | 200 `SelectionPlanList` (#92) |
| getSelectionPlan | `GET /selection-plans/{planId}` | 200 `SelectionPlan` (#92) |
| reviseSelectionPlan | `POST /selection-plans/{planId}/revisions` | 201 `SelectionPlan` (#92) |

Every request and response carries `contractVersion`. Mutating requests also carry an `idempotencyKey`. The operation names are the keys of the conformance fixtures and the method names of both SDKs; `internal/publicengine/conformance` maps them to the paths above.

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
| `MODEL_BINDING_UNAVAILABLE` | 422 |
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

## Question-conditioned analysis

`startAnalysis.researchQuestion` is an optional, domain-neutral semantic input.

- When present, the engine passes the question to every semantic LLM stage: observation selection, mismatch/pattern detection, hypothesis generation, evidence/counter-evidence retrieval, synthesis and dedupe.
- The prompt explicitly tells the model not to assume the question's premise is true and to retain evidence or alternative explanations that can falsify or reframe it.
- The question is part of the **input fingerprint**, not the execution fingerprint. Same evidence + different question is therefore an input change. `compareAnalyses.input.researchQuestion` exposes the field-level question change when present.
- When omitted, analysis remains open-ended discovery.
- A `createResearchRun.question` or appended iteration must match the analysis's recorded question when that analysis was question-conditioned. Analyses created before this field existed, or analyses with no question, remain compatible.
- The field is additive in Public Engine Contract v1. Legacy hypothesis field/stage names such as `latentNeed` / `need_hypothesis` remain wire-compatible; they no longer imply that the research domain is customer needs.

## Model-backed capability

`EngineInfo.modelBacked` reports whether analyses use a configured model (it follows live settings). A deterministic engine (`false`) analyzes evidence into observations but never forms hypotheses, so `createResearchRun` / `appendIteration` over its analyses fail with `ANALYSIS_HAS_NO_HYPOTHESES`. Consumers should check it before starting research work.

## Model bindings (#65 extension point)

Routing policy (which model for which stage, cost budgets, escalation) belongs to consumers. The engine exposes only a minimal, provider-neutral extension point:

- `EngineInfo.modelRouting` lists the pipeline `stages` a caller may bind and the `allowedModels` the operator configured (`-allowed-models` / `INSIGHT_LAB_ALLOWED_MODELS`; the configured `-model` is always allowed).
- `startAnalysis.modelBindings` maps stages to allowed models. Unknown stage → `INVALID_REQUEST`; a model the operator did not allow, or no model endpoint → `MODEL_BINDING_UNAVAILABLE` (422).
- Bindings are execution configuration: they are recorded per stage in `provenance.execution.llm.models`, change the execution fingerprint (so run comparison attributes the difference to execution), and never change research semantics.
- Conformance: `17-model-bindings` (requires an engine started with `-model scripted-model -allowed-models scripted-model-large`, see below).
- `EngineInfo.modelBacked` (#104) is true when analyses use a configured model and can form hypotheses. False means deterministic only: research runs are refused with `ANALYSIS_HAS_NO_HYPOTHESES`, so a consumer that needs research checks this flag before starting an analysis instead of failing afterwards.

## Limits

The request body is at most 16 MiB. A request carries at most 500 documents and 50 artifacts. Document content is limited to 200,000 characters and both analysis/research questions to 2,000 characters. Metadata is at most 32 string entries with keys matching `^[A-Za-z0-9._:-]{1,64}$` and values up to 1,024 characters. Document metadata keys starting with `public_` or `analytical_`, and the keys `dataset_hash` and `acquisition_manifest`, are reserved. The engine records that provenance itself, so a consumer cannot claim a file hash or acquisition manifest that the engine would then report as verified.

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

- `contracts/public-engine/v1/fixtures/` holds 17 fixtures. Each names its `engine` (`deterministic` or `model_backed`):
  1. generic public-data subject (model-backed)
  2. commerce-like opaque subject (model-backed)
  3. deterministic Analytical Artifact
  4. missing evidence to added evidence to a new iteration (model-backed)
  5. counter-evidence (model-backed)
  6. contract version mismatch
  7. idempotent requests
  8. same research results across LIGHT / STANDARD / HEAVY (`08-execution-profile-equivalence`, #91)
  9. raw/ref input path (`09-raw-artifact-input`, #90)
  10. run comparison (#83, model-backed)
  11. re-evaluation (#74, model-backed)
  12. longitudinal timeline (#71, model-backed)
  13. scenarios (#66, model-backed)
  14. data triage (#92)
  15. re-triage from research gaps (#92, model-backed)
  16. temporal operation pack (#73)
  17. model bindings (#65, model-backed; needs `-allowed-models scripted-model-large`)
- Fixtures 08 and 09 need an engine started with an input root containing `fixtures/data` and, for 08, a HEAVY adapter (`-input-root` and `-heavy-dir`).
- `internal/http/public_conformance_test.go` starts a deterministic engine and a model-backed engine. It runs every fixture over plain HTTP with `internal/publicengine/conformance`. SDK repositories run the same fixture files against a live engine or recorded responses.
- The model-backed engine in that test uses the scripted stand-in model from `internal/llm/scripted`, the same one `cmd/insight-scripted-llm` serves over HTTP for SDK repositories. Production code has no fake-model mode: the engine only ever talks to an OpenAI-compatible endpoint. Model-backed fixtures check contract behavior with that model. They do not measure the quality of a real model.

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

### Running model-backed fixtures outside this repository

`cmd/insight-scripted-llm` serves a deterministic, grounded, OpenAI-compatible `/chat/completions` endpoint (the same scripted model the in-repo conformance test uses). It is a test tool, never a model:

```sh
go run ./cmd/insight-scripted-llm -addr 127.0.0.1:8788 &
go run ./cmd/insight-lab -port 8787 -no-browser -db /tmp/insight-model.db -base-url http://127.0.0.1:8788 \
  -model scripted-model -allowed-models scripted-model-large -api-key scripted
```

Fixture 17 asserts `modelRouting.allowedModels[0] == "scripted-model-large"` and binds `scripted-model`, so both flags are required as written. Standalone SDK repositories point `INSIGHT_MODEL_BACKED_URL` at that engine to run fixtures whose `engine` is `model_backed`, and `INSIGHT_DETERMINISTIC_URL` at a second engine started with `-input-root <fixtures/data> -heavy-dir <dir>` and its own `-db`. This section is the canonical copy of the setup; the SDK READMEs repeat it and defer to it.
