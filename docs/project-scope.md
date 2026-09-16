# Project Scope

Insight Lab is an open-source, evidence-grounded research and diagnostic engine.

Its purpose is to make analytical reasoning inspectable: observations should be grounded, hypotheses should remain hypotheses, counter-evidence should be visible, and uncertainty should be explicit.

## In scope

The public project may contain reusable mechanics for:

- document and structured-data ingestion through generic boundaries;
- observation extraction and grounding;
- pattern, expectation, mismatch, and surprise detection;
- abductive hypothesis generation;
- primary and competing hypothesis management;
- supporting, counter, and neutral evidence;
- falsification criteria and missing-evidence tracking;
- candidate causal structures and variable roles;
- validation, causal, and identification states;
- deterministic quality guardrails;
- auditable reports and evaluation tooling;
- synthetic fixtures and Golden Dataset-style regression evaluation.

## Out of scope

Insight Lab is not intended to become:

- a generic chatbot or generic RAG product;
- a system that treats LLM output as causal proof;
- a statistical causal-effect estimation package;
- an automatic true-DAG discovery engine;
- a generic BI/dashboard platform;
- a sales proposal generator;
- a pricing or estimating engine;
- a customer-specific commercial recommendation system;
- a private decision layer.

Statistical estimators such as DiD, RDD, IV, propensity-score methods, causal forests, or DML may be discussed as candidate validation designs. Implementing them is a separate decision and should not be implied by the current causal-reasoning pipeline.

## Public/private boundary

A useful rule is:

```text
PUBLIC OSS
Data
→ Observation
→ Surprise
→ Hypotheses
→ Evidence / Counter Evidence
→ Missing Evidence
→ Validation / Identification
→ Insight Candidate

PRIVATE OR DOWNSTREAM APPLICATION
Assessment
→ Recommendation
→ Architecture / Intervention
→ Estimate / Pricing
→ Proposal / Commercial Decision
```

Downstream applications are free to consume Insight Lab outputs. They should not require Insight Lab itself to embed customer-specific business judgment.

## External data boundary

Insight Lab should remain source-agnostic.

```text
external source
    ↓
adapter / exporter
    ↓
generic CSV or supported ingestion format
    ↓
Insight Lab
```

A source-specific collector may know about a government API, company registry, survey platform, or other schema. Insight Lab's core domain should not.

## Design test

Before adding a feature, ask:

> Would this feature still make sense for a researcher analyzing a completely different domain and dataset?

If yes, it may belong in Insight Lab.

If it primarily answers “what should this particular business sell, build, charge, or propose?”, it belongs downstream.
