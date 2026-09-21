# Insight Lab

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.25%2B-00ADD8.svg)](go.mod)

**Bring your own evidence. Insight Lab helps turn observations into auditable hypotheses, competing explanations, research gaps, and decision-ready handoffs — without pretending that correlation proves causation.**

[日本語](README.ja.md) · [Documentation](docs/README.md) · [Causal reasoning semantics](docs/causal-reasoning.md) · [Project scope](docs/project-scope.md) · [Contributing](CONTRIBUTING.md)

## What Insight Lab is

Insight Lab is an open-source, local-first **evidence reasoning engine**.

It is designed for research where the input already exists in some form:

- customer interviews, reviews, support logs, sales notes, surveys, and other raw evidence;
- structured datasets such as CSV exports, BI data, operational metrics, public-data exports, and `ja-company-base` output;
- existing research or analysis artifacts such as internal studies, consulting reports, market research, or AI-generated analysis.

Insight Lab is not intended to be a general-purpose research chatbot. It does not autonomously search the web, authenticate to external data services, or acquire missing evidence on its own.

The core reasoning path is:

```text
Provided Evidence
  ↓
Observation / Claim
  ↓
Expectation + provenance
  ↓
Mismatch / surprise
  ↓
Primary + competing hypotheses
  ↓
Supporting / counter / neutral evidence
  ↓
Research gaps / DataRequirements
  ↓
Validation + identification status
  ↓
Decision readiness / human handoff
```

When evidence is insufficient, Insight Lab should say what is missing rather than manufacture certainty.

## Product boundary: BYO Evidence

Insight Lab follows a **Bring Your Own Evidence** boundary.

Inside the OSS core:

- ingestion through generic document / dataset boundaries;
- deterministic dataset pre-analysis where possible;
- grounded observations and claims;
- expectation / mismatch reasoning;
- competing hypotheses;
- supporting, counter, and neutral evidence;
- research gaps and provider-neutral `DataRequirement` values;
- append-only research iterations;
- validation, identification, and decision-readiness states;
- Markdown reports and versioned JSON Research Artifacts.

Outside the OSS core:

- autonomous web search;
- authenticated retrieval from e-Stat, registries, Drive, CRM, or other external systems;
- customer-specific credential management;
- continuous external monitoring;
- autonomous evidence-acquisition agents;
- pricing, proposals, estimates, or customer-specific commercial recommendations.

A downstream/private orchestration layer may receive `DataRequirement` values, acquire additional evidence, and feed that evidence back into Insight Lab for a new research iteration.

See [BYO-Evidence boundary](docs/byo-evidence-boundary.md).

## Analysis modes

The target architecture distinguishes **how an input should be interpreted** from **where the research currently is in its lifecycle**.

### 1. Discovery

For minimally interpreted primary evidence:

- interviews;
- reviews;
- support and sales logs;
- survey free text;
- customer feedback.

Typical flow:

```text
Raw Evidence
→ Observation
→ Pattern / latent need
→ Hypothesis
→ Evidence / counter-evidence
→ Research gap
```

### 2. Dataset Analysis

For structured data:

- CSV / BI exports;
- operational metrics;
- e-Stat or municipality exports;
- `ja-company-base` analysis exports.

Typical flow:

```text
Structured Dataset
→ schema / unit / period / population checks
→ deterministic aggregation / delta / baseline
→ candidate observations
→ mismatch
→ competing hypotheses
→ evidence / counter-evidence
→ research gap
```

Numeric source-of-truth calculations should remain deterministic. Model-backed interpretation is optional and must not silently recalculate authoritative values.

### 3. Research Review

For already-interpreted material:

- internal analysis;
- research-firm reports;
- consultant reports;
- BI narratives;
- human memos;
- AI / ChatGPT / Claude analysis.

The important rule is:

> An external claim is not automatically an Observation or primary evidence.

Research Review should decompose artifacts into claims, evidence references, assumptions, methods, counter-evidence, and missing evidence before those claims influence stronger conclusions.

**Current status:** first-class semantic Analysis Mode is implemented for `DISCOVERY`, `DATASET_ANALYSIS`, and `RESEARCH_REVIEW`. Research Review preserves imported claims as claims rather than silently upgrading them into observations or primary evidence. All modes converge on the same evidence-reasoning core; source-specific acquisition remains outside Insight Lab.

**Analysis Mode vs. Execution Mode:** Analysis Mode describes how an input should be interpreted. `service.ExecutionMode` (`deterministic` / `model_backed`) only records whether a model participated in the run. The two concepts are orthogonal and are exported separately. The legacy Research Artifact v1 JSON key `analysisMode` remains the execution-mode field for compatibility, while semantic mode is exported separately.

## Research stage is separate from analysis mode

Analysis Mode answers:

> How should this input be read?

Research Stage answers:

> How mature is the current knowledge?

The research lifecycle is modeled separately:

```text
DISCOVERY
→ EXPLORATORY
→ VALIDATION
→ SYNTHESIS
```

A hypothesis generated after looking at a dataset is exploratory. It must not be presented as if it had been fixed before the data was observed.

Current `main` includes Research Stage plus first-class `Expectation` entities with provenance, explicit freeze-for-validation, guarded `EXPLORATORY → VALIDATION` transitions, cross-iteration carry-forward, and versioned Research Artifact export. That lifecycle was completed in [#31](https://github.com/sibukixxx/insight/issues/31).

## Current capabilities

Current `main` includes:

- local-first projects backed by SQLite;
- text and CSV ingestion through generic boundaries;
- source-backed observation grounding;
- deterministic dataset pre-analysis that can run without an LLM;
- acquisition-manifest and dataset-hash provenance;
- unit / population / period compatibility warnings;
- OpenAI-compatible model-backed interpretation;
- primary and competing hypotheses;
- supporting, counter, and neutral evidence;
- explicit causal / validation / identification states;
- append-only `ResearchRun` / `ResearchIteration` history;
- semantic Analysis Mode (`DISCOVERY` / `DATASET_ANALYSIS` / `RESEARCH_REVIEW`) and imported Research Claims;
- first-class `Expectation` provenance, freeze-for-validation, and cross-iteration lineage;
- persisted independent-validation evidence provenance;
- prioritized research gaps and next-data requirements;
- explicit `DataRequirement.gapId` → added-evidence linkage;
- generic Insight Semantics v2: Connection, candidate Mechanism, and Generalization / boundary conditions;
- iteration input snapshots and Insight Delta showing what changed between research iterations;
- decision-readiness and stopping reasons;
- human override and human handoff;
- Markdown research reports;
- versioned JSON Research Artifact export at
  `GET /api/research-runs/{runID}/artifact.json`, including Research Stage, semantic mode, Expectations, Claims, validation evidence, gap linkage, Insight Delta, and promotion state;
- persisted approved-artifact snapshots for reviewed publication-ready output;
- deterministic quality guardrails;
- Shared Eval / Golden evaluation infrastructure, including association-only, population-mismatch, real Open Data reproducibility, new-evidence re-analysis, inconclusive, competing-hypothesis, promotion, and human-review cases.

The publication-promotion workflow is implemented through domain/service/usecase/HTTP/report layers. Human review is required before `PUBLICATION_READY`; publication is never automatic. `approved-artifact.json` returns the persisted reviewed snapshot rather than regenerating the artifact from later state.

## Causal claims: intentionally conservative

Insight Lab is **not a causal-effect estimator**.

Model-generated prose, evidence counts, or an application confidence score cannot promote a hypothesis into a proven causal claim.

Observational causal hypotheses remain `NOT_IDENTIFIED` unless an appropriate external research design supplies stronger evidence. The system may suggest control groups, pre/post comparisons, natural experiments, Difference-in-Differences, RDD, or IV as candidate validation designs; it must not imply that those analyses have already been executed.

See [Causal reasoning semantics](docs/causal-reasoning.md).

## Research loop

Insight Lab does not stop at a one-shot report.

```text
Evidence
→ analysis
→ ResearchGap / DataRequirement
→ external acquisition
→ additional evidence
→ new ResearchIteration
→ re-analysis
→ stop / continue
```

Only evidence acquisition leaves the OSS boundary. Research history and re-analysis remain inside Insight Lab.

Additional evidence can be linked explicitly to the `DataRequirement.gapId` it addresses. The linkage is preserved in append-only iteration history and the Research Artifact, so a later reader can see which missing-evidence requirement an acquired item was intended to resolve.

A run may stop because evidence converged, important uncertainty remains unresolved, no feasible source exists, evidence conflicts, or a human chooses to stop. Unresolved gaps remain visible after stopping.

See [Research Loop dogfooding](docs/research-loop.md).

## Quick start

### Requirements

- Go 1.25+
- An OpenAI-compatible API only when model-backed interpretation is required

### Run the fictional demo

```bash
make build-demo
./bin/insight-lab-demo --demo
```

Open `http://127.0.0.1:8787`.

Configure the API base URL, model, and API key from Settings, or pass `--base-url`, `--model`, and `--api-key`.

### Analyze primary text evidence

1. Create a project.
2. Paste text or import CSV with `id,source,title,content` columns.
3. Run analysis.
4. Inspect observations, hypotheses, evidence, counter-evidence, missing evidence, warnings, and identification status.
5. Create or continue a Research Run when iterative investigation is needed.

### Analyze a structured dataset

External data should be acquired outside Insight Lab, normalized, and then imported with provenance.

```text
external source
  ↓
adapter / human / private acquisition
  ↓
normalized dataset + acquisition manifest
  ↓
Insight Lab
```

The deterministic pre-analysis path can complete without an LLM when the imported dataset supports it. Model-backed hypothesis or narrative generation requires a configured provider.

For a reproducible example, see [ja-company-base dogfooding](docs/dogfooding-ja-company.md).

### Export a machine-readable Research Artifact

```bash
curl -o artifact.json \
  http://127.0.0.1:8787/api/research-runs/<runID>/artifact.json
```

Downstream systems should consume this versioned artifact rather than parse `report.md`.

## Current roadmap

The current phase is deliberately narrower than the earlier feature-expansion roadmap:

1. **Real-data dogfooding / Public Evidence Reports** — run real public evidence through the complete Research Loop, follow at least one `ResearchGap` with additional evidence, exercise validation provenance / Insight Delta / Promotion review, and publish from an approved artifact where the evidence supports publication. See [#60](https://github.com/sibukixxx/insight/issues/60).
2. **Stable Public Engine contract** — define the narrow, versioned boundary that future Go and Node.js SDK repositories can depend on without exposing `internal/*` implementation details. See [#59](https://github.com/sibukixxx/insight/issues/59).
3. **Failure-driven expansion** — add core features only when dogfooding or a real consumer exposes a generic contract, research-semantics, correctness, or performance gap.

The current small interactive UI remains sufficient for this phase. A large bulk-ingestion workspace is not an active roadmap item; if real workloads demonstrate that need, split the work into measured engine concerns such as streaming / bounded-memory ingestion and downstream UI/orchestration concerns.

The Go and Node.js SDKs do **not** exist yet. Insight Lab remains the source of truth for research semantics and machine-readable contracts; #59 stabilizes that boundary before separate SDK repositories are created.
## Build, test, and evaluate

```bash
make build
make build-demo
make vet
make test
```

Golden tests:

```bash
go test -tags=golden ./...
```

Real-model evaluation:

```bash
INSIGHT_LAB_API_KEY=sk-... \
INSIGHT_LAB_MODEL=<model> \
make eval-demo
```

## What confidence means

The application score is an evidence-quality / coverage signal derived from deterministic factors such as grounding, coverage, source diversity, frequency, and counter-evidence.

**It is not the probability that a claim is true. It is not causal probability.**

Likewise, provider confidence from any semantic classifier must only describe that bounded classification decision, not the truth of the underlying hypothesis.

## Project scope

Insight Lab ends around evidence-grounded research artifacts, research gaps, validation / identification state, and decision-ready handoff.

It intentionally does not contain:

- commercial assessment;
- proposal generation;
- pricing or estimates;
- customer-specific architecture recommendations;
- customer-specific business decisions.

Those belong in downstream applications.

See [Project scope](docs/project-scope.md).

## Documentation

Start with the [documentation index](docs/README.md).

Key documents:

- [Project scope](docs/project-scope.md)
- [BYO-Evidence boundary](docs/byo-evidence-boundary.md)
- [Current project status](docs/project-status.md)
- [Research Loop](docs/research-loop.md)
- [Causal reasoning semantics](docs/causal-reasoning.md)
- [Detailed design](docs/detailed-design.md) (historical v1)
- [Evaluation](docs/evaluation/README.md)
- [Contributing](CONTRIBUTING.md)
- [Security](SECURITY.md)

## Privacy

Project data is stored locally. Text required for model-backed analysis is sent to the AI provider you configure. Review that provider's data-handling policy before processing confidential, regulated, or personal information.

Never commit API keys or other secrets to the repository.

## Contributing

Issues and pull requests are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) before making substantial changes, especially changes to causal semantics, evidence boundaries, research stages, or quality guardrails.

## License

Copyright 2026 Yuichi Takada.

Licensed under the [Apache License 2.0](LICENSE). Third-party dependencies retain their own licenses; see `go.mod` and `go.sum`.
