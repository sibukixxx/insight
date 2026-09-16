# Insight Lab

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.25%2B-00ADD8.svg)](go.mod)

**Evidence-grounded research for turning observations into auditable hypotheses — without pretending that correlation proves causation.**

[日本語](README.ja.md) · [Documentation](docs/README.md) · [Causal reasoning semantics](docs/causal-reasoning.md) · [Contributing](CONTRIBUTING.md)

## Why Insight Lab?

LLMs are good at producing plausible explanations. Plausible is not the same as supported.

Insight Lab keeps the reasoning trail visible:

```text
Data
  ↓
Observation
  ↓
Expectation
  ↓
Expectation mismatch / surprise
  ↓
Primary + competing hypotheses
  ↓
Supporting evidence / counter-evidence
  ↓
Missing evidence / falsification criteria
  ↓
Validation + identification status
```

The goal is not to make an AI sound certain. The goal is to make it easier for a human researcher to inspect **what was observed, what was inferred, what challenges the inference, and what remains unknown**.

## Current capabilities

- Local-first projects backed by SQLite.
- Text and CSV ingestion through a generic document boundary.
- Grounded observations tied to source material.
- Expectation/mismatch detection and abductive hypothesis generation.
- Supporting, counter, and neutral evidence with counter-search coverage.
- Independently evaluated competing hypotheses grouped by `HypothesisSetID`.
- Candidate causal structures with exposure, outcome, confounder, mediator, collider, and unknown roles.
- Explicit causal, validation, and identification states.
- Deterministic quality guardrails in application code.
- Markdown report export with an auditable evidence trail.
- OpenAI-compatible model configuration.
- Evaluation tooling for repeatable real-model dogfooding.

## Causal claims: intentionally conservative

Insight Lab is **not a causal-effect estimator**.

Model-generated prose, evidence counts, and the application's evidence-quality score cannot promote a hypothesis to a proven causal claim. The current pipeline keeps causal hypotheses `NOT_IDENTIFIED` unless an appropriate external research design supplies stronger evidence.

The system may suggest a control group, pre/post comparison, natural experiment, Difference-in-Differences, RDD, or IV as a **candidate validation design**. It does not claim that such a design has been executed.

See [Evidence-Grounded Causal Reasoning Semantics](docs/causal-reasoning.md) for the implementation contract.

For append-only investigation history, structured research gaps, next-data requirements, and human novelty evaluation, see [Research Loop dogfooding](docs/research-loop.md).

## Quick start

### Requirements

- Go 1.25+
- An OpenAI-compatible API for model-backed analysis

### Run the fictional demo

```bash
make build-demo
./bin/insight-lab-demo --demo
```

Open `http://127.0.0.1:8787`.

Configure the API base URL, model, and API key from Settings, or pass `--base-url`, `--model`, and `--api-key`.

### Analyze your own data

1. Create a project.
2. Paste text or import CSV with `id,source,title,content` columns.
3. Run the analysis.
4. Inspect observations, competing hypotheses, evidence, counter-evidence, missing evidence, quality warnings, and identification status.
5. Export the result as Markdown.

```bash
curl -o report.md http://127.0.0.1:8787/api/projects/<projectID>/report.md
```

Input may be written in any language supported by the configured model. Generated analysis is instructed to follow the input language.

## Build, test, and evaluate

```bash
make build
make build-demo
make vet
make test
```

To run the repeatable real-model evaluation flow:

```bash
INSIGHT_LAB_API_KEY=sk-... \
INSIGHT_LAB_MODEL=<model> \
make eval-demo
```

Evaluation output is stored under `docs/evaluation/` for inspection and comparison.

## What the score means

The application-calculated score measures evidence quality factors such as grounding, coverage, source diversity, frequency, and counter-evidence.

**It is not the probability that a claim is true, and it is not a probability of causality.**

## Project scope

Insight Lab is an OSS research/diagnostic engine. Its public boundary ends around evidence-grounded insight candidates and validation state.

It intentionally does not contain commercial assessment, proposal generation, pricing, architecture recommendations, estimates, or private decision logic. See [Project scope](docs/project-scope.md).

## External structured data

Insight Lab stays source-agnostic. External datasets should enter through adapters and the generic ingestion boundary rather than becoming domain-specific dependencies.

For example:

```text
public dataset / external collector
        ↓
CSV (JSONL adapter planned)
        ↓
Insight Lab ingestion
        ↓
evidence-grounded analysis
```

## Documentation

Start with the [documentation index](docs/README.md).

Key documents:

- [Project scope](docs/project-scope.md)
- [Current project status](docs/project-status.md)
- [Causal reasoning semantics](docs/causal-reasoning.md)
- [Causal inference learning notes](docs/learning-notes-causal-inference.md)
- [Detailed design](docs/detailed-design.md)
- [Evaluation](docs/evaluation/README.md)
- [Contributing](CONTRIBUTING.md)
- [Security](SECURITY.md)
- [Code of Conduct](CODE_OF_CONDUCT.md)

## Privacy

Project data is stored locally. Text required for model-backed analysis is sent to the AI provider you configure. Review that provider's data-handling policy before processing confidential, regulated, or personal information.

Never commit API keys or other secrets to the repository.

## Contributing

Issues and pull requests are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) before making substantial changes, especially changes to causal semantics or quality guardrails.

## License

Copyright 2026 Yuichi Takada.

Licensed under the [Apache License 2.0](LICENSE). Third-party dependencies retain their own licenses; see `go.mod` and `go.sum` for the dependency set.
