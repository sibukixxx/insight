# Insight Lab

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.25%2B-00ADD8.svg)](go.mod)

Open-source, local-first research and evidence reasoning engine.

Insight Lab accepts supplied evidence, records analysis and research history, and produces auditable observations, hypotheses, gaps, timelines, and scenario evaluations. It can run deterministically or with an OpenAI-compatible model.

[日本語](README.ja.md) · [Documentation](docs/README.md) · [Architecture](docs/architecture.md) · [Public Engine Contract](docs/public-engine-contract.md)


<!-- role-boundary:v1 -->
## Role and boundaries

**Role:** domain-neutral research and evidence-reasoning engine. Insight turns supplied evidence into auditable observations, hypotheses, counter-evidence, research gaps, iterations, deltas, timelines, and scenarios.

### Owns

- research semantics and persisted research history
- Evidence / Observation / Hypothesis / ResearchGap / DataRequirement semantics
- re-evaluation, temporal comparison, scenario and provenance contracts
- the language-neutral Public Engine Contract

### Does not own

- a consumer's business decision or workflow state
- privileged-action authorization or execution
- provider credentials or external side effects
- CRM, billing, commerce, marketing, or other domain policy
- autonomous source acquisition as a hidden dependency

### Integration

```text
domain / data producer
        ↓ evidence / analytical artifact
      Insight
        ↓ research result / gap / delta
domain-owned decision
```

Consumers should integrate through the versioned Public Engine Contract or the thin standalone SDKs. Insight Core must remain usable without any specific private application or vertical.
## Quick start

Requirements:

- Go 1.25+
- SQLite is embedded; no external database is required

Build and start the reference server:

```sh
make build
./bin/insight-lab serve -no-browser
```

Default endpoint:

```text
http://127.0.0.1:8787
```

API only:

```sh
./bin/insight-lab serve -no-web
```

Show engine capabilities without starting HTTP:

```sh
./bin/insight-lab engine
```

The headless CLI also provides `subject`, `evidence`, `analysis`, `research`, and `status` commands.

For model-backed analysis, configure an OpenAI-compatible endpoint:

```sh
./bin/insight-lab serve \
  -base-url https://example.invalid/v1 \
  -model your-model \
  -api-key "$API_KEY"
```

Without a model, deterministic ingestion, validation, temporal operations, and supported analytical processing remain available. Model-generated hypotheses require a configured model.

## What it does

The core research flow is:

```text
Evidence
  ↓
Observation / Claim
  ↓
Expectation / Mismatch
  ↓
Primary + competing hypotheses
  ↓
Supporting / counter / neutral evidence
  ↓
ResearchGap / DataRequirement
  ↓
ResearchIteration
  ↓
Re-evaluation / Timeline / Scenario
```

Research state is persisted by the engine. Re-running analysis does not overwrite previous runs.

## Inputs

| Input | Boundary |
| --- | --- |
| Text evidence | Documents or Document CSV |
| Structured observations | Dataset Documents |
| Deterministic external analysis | Analytical Artifact v1 |
| Large/raw files | `RAW_ARTIFACT` references with supported preparation specs |

Large raw datasets are not stored in SQLite. Normalize or aggregate them externally, or use a supported raw-artifact preparation path, then pass the resulting evidence into Insight.

Insight does not directly provide generic XLSX/PDF/Parquet ingestion, arbitrary SQL connectivity, web crawling, or SaaS connectors.

See [BYO-Evidence boundary](docs/byo-evidence-boundary.md) and [Analytical Artifact contract](docs/analytical-artifact-contract.md).

## Research model

Insight keeps five independent axes:

| Axis | Values |
| --- | --- |
| AnalysisMode | Discovery / Dataset Analysis / Research Review |
| ReasoningProfile | `GENERAL_RESEARCH` / `CUSTOMER_INSIGHT` |
| ResearchStage | `DISCOVERY` / `EXPLORATORY` / `VALIDATION` / `SYNTHESIS` |
| ExecutionMode | deterministic / model-backed |
| ExecutionProfile | `LIGHT` / `STANDARD` / `HEAVY` / `AUTO` |

`GENERAL_RESEARCH` is the default reasoning profile. Profiles are explicit and are not inferred from source type, namespace, or document wording.

See [Architecture](docs/architecture.md) and [Causal reasoning semantics](docs/causal-reasoning.md).

## Persistence

SQLite is the default persistent store.

The Core owns research state, including:

- subjects and analyses;
- observations and evidence;
- insights and hypotheses;
- research runs and append-only research iterations;
- temporal evidence;
- scenario sets and evaluations;
- human research evaluations.

Analysis input and execution snapshots are recorded so runs can be compared without treating configuration changes as evidence changes.

Derived views such as longitudinal timelines and observation deltas are rebuilt from persisted state where possible instead of being maintained as independent sources of truth.

Specify a database path with:

```sh
./bin/insight-lab serve -db ./insight.db
```

If `-db` is omitted, Insight uses the operating system's application data directory.

## Public API and SDKs

The language-neutral API is:

```text
/api/public/v1
```

The canonical schema and conformance fixtures live in:

```text
contracts/public-engine/v1
```

Optional standalone SDKs:

- [insight-sdk-go](https://github.com/sibukixxx/insight-sdk-go)
- [insight-sdk-js](https://github.com/sibukixxx/insight-sdk-js)

SDKs are thin clients. Insight Core does not depend on them.

## Execution profiles

- `LIGHT` — small/local workloads.
- `STANDARD` — bounded streaming and concurrent preparation.
- `HEAVY` — delegates heavy execution through a configured Heavy Runtime adapter.
- `AUTO` — deterministic profile selection with the resolved profile recorded.

Enable local raw-file references or a Heavy Runtime directory with:

```sh
./bin/insight-lab serve -input-root ./data -heavy-dir ./heavy
```

See [Architecture](docs/architecture.md).

## Operational responsibility

Insight Lab is self-hosted software. The operator is responsible for:

- deployment and access control;
- database backup and restore;
- model credentials and provider configuration;
- external/raw data storage;
- retention and privacy policy;
- monitoring and availability.

Managed products may provide these functions around Insight Core, but they are not part of the OSS research semantics.

## Non-goals

Insight Core is not:

- an autonomous web-research agent;
- a general-purpose data warehouse;
- a causal-effect estimator;
- a forecasting engine that selects a most-likely future;
- a managed multi-tenant SaaS control plane.

Unknown, insufficient evidence, and not-identified are valid research outcomes.

## Build and test

```sh
make build
make test
make vet
```

Golden evaluation:

```sh
make test-golden
```

## Documentation

- [Documentation index](docs/README.md)
- [Architecture](docs/architecture.md)
- [Public Engine Contract v1](docs/public-engine-contract.md)
- [Research loop](docs/research-loop.md)
- [Temporal evidence](docs/temporal-evidence.md)
- [Longitudinal research](docs/longitudinal-research.md)
- [Scenario analysis](docs/scenario-analysis.md)
- [Project scope](docs/project-scope.md)
- [Project status](docs/project-status.md)

## License

Apache License 2.0. See [LICENSE](LICENSE).
