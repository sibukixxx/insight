# Analytical Artifact Contract

`insight-lab.analytical-artifact` v1 is the provider-neutral import/export contract for deterministic calculations produced outside Insight. The producer may be DuckDB, another SQL engine, a dataframe tool, BI export, or custom code; Insight does not embed or select that engine.

The JSON Schema is the language-neutral source of truth at `contracts/analytical-artifact/v1/schema.json`; the declarative temporal operation spec `temporal-operation.schema.json` lives beside it (see [temporal-evidence.md](temporal-evidence.md)). Inside this repository Go code uses `contracts/analytical`. External producers use the standalone SDKs — `insight-sdk-go/analytical` or the `analytical` module of `@sibukixxx/insight-sdk` — which pin a copy of this directory and submit artifacts through the Public Engine Contract (`addEvidence.analyticalArtifacts`). Consumers never import Insight internal packages. The Evidence boundary adapter remains internal.

## Trust boundary

An Analytical Artifact records calculation output, not an explanation:

- a result is never a cause, hypothesis, Claim, or Insight;
- the adapter emits `domain.EvidenceNeutral` and an Observation candidate;
- a later research iteration evaluates meaning, competing explanations, counter evidence, and causal limitations;
- producer-specific configuration stays in referenced parameters/specs and never becomes an Insight Core dependency.

V1 Research Artifacts and existing APIs remain unchanged.

## Domain neutrality and missing values

`externalSubject` is optional and opaque. Its namespace and ID let a producer correlate an artifact with its own object without leaking domain enums, workflow states, or requested actions into Insight Core.

Missing results set `missing: true` and omit `value`. Unknown is never coerced to zero, false, counter-evidence, or a producer Claim. Missing period, population, dataset hash, spec hash, or provenance is rejected rather than guessed.

## Reproducibility and identity

Every artifact carries producer/version, generation time, versioned SHA-256-addressed datasets, a SHA-256-addressed query/spec, parameters, filters, period, population, metric definitions, deterministic results, quality flags, computation metadata, and source provenance.

The artifact `id` is the idempotency key:

- same `id` plus same `artifactHash`: duplicate/no-op;
- same `id` plus different `artifactHash`: identity conflict;
- different `id`: distinct artifact.

`ReproducibilityKey` is derived from dataset IDs, versions and hashes, the spec hash, parameters, and filters. The external producer owns `artifactHash` and computes it from its canonical serialized artifact.

## Producer examples

An external analytical producer can:

1. checksum every input dataset;
2. store and checksum its versioned query or declarative spec;
3. execute with pinned parameters;
4. emit either fixture under `contracts/analytical-artifact/v1/fixtures/`;
5. validate and seal it (`analytical` in an SDK, or `contracts/analytical` inside this repository) and submit it through the Public Engine Contract (`addEvidence.analyticalArtifacts`); the internal Evidence/Observation adapter takes over from there.

`trade-time-series.json` demonstrates a SQL-style time series. `external-consumer.json` demonstrates a domain-neutral producer, opaque subject correlation, and an explicitly missing result.

No DuckDB dependency, connection, SQL execution, consumer action, or domain-specific enum is introduced into Insight.

## Compatibility

Consumers verify `artifactSchema` and `schemaVersion`. Unknown JSON fields are tolerated for additive evolution. Renaming or removing fields, changing semantics, or adding required fields requires a new schema version.

