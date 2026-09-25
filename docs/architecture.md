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

## Five independent axes

| Axis | Values | Meaning |
|---|---|---|
| AnalysisMode | Discovery / Dataset Analysis / Research Review | How input is interpreted |
| ReasoningProfile | GENERAL_RESEARCH (default) / CUSTOMER_INSIGHT | Which research vocabulary is layered on the shared epistemic rules ([reasoning profiles](reasoning-profiles.md)) |
| ResearchStage | DISCOVERY / EXPLORATORY / VALIDATION / SYNTHESIS | Where the research is in its lifecycle |
| ExecutionMode | deterministic / model-backed | Whether a model is used where defined |
| ExecutionProfile | LIGHT / STANDARD / HEAVY / AUTO | Resource/runtime strategy |

They never substitute for each other. ExecutionProfile must not change hypothesis or readiness semantics; conformance fixture `08-execution-profile-equivalence` checks this. ReasoningProfile is always selected explicitly, never inferred from input; it changes prompts and the execution fingerprint, never the input fingerprint or the grounding/evidence/causal guardrails (`18-reasoning-profiles`).

## Quickstart — direct engine use

```sh
make build
./bin/insight-lab -no-browser                    # UI + /api + /api/public/v1 on 127.0.0.1:8787
./bin/insight-lab -input-root ./data -heavy-dir ./heavy   # enable raw references and HEAVY
```

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
