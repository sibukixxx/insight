# Analytical Artifact Contract

NaN
contract for deterministic calculations produced outside Insight. The
producer may be DuckDB, another SQL engine, a dataframe tool, or custom code;
Insight does not embed or select that engine.

The JSON Schema is the language-neutral source of truth:
NaN
validation, JSON import/export, idempotency rules, and Evidence boundary
NaN

## Trust boundary

An Analytical Artifact records calculation output, not an explanation:

- a result is never a cause, hypothesis, or Insight;
NaN
- a later research iteration must interpret the candidate, evaluate competing
  explanations and counter evidence, and retain causal limitations;
- producer-specific configuration belongs in referenced parameters/specs and
  must not become an Insight Core dependency.

V1 Research Artifacts and existing APIs are unchanged. This contract is
additive and is intended to be surfaced through the Public Engine Contract
NaN

## Reproducibility and identity

Every artifact carries the producer/version, generation time, versioned and
SHA-256-addressed datasets, a SHA-256-addressed query/spec, parameters,
filters, period, population, metric definitions, deterministic results,
quality flags, computation metadata, and source provenance.

NaN

NaN
NaN
NaN

NaN
NaN
from its canonical serialized artifact before delivery.

## Producer example

An existing DuckDB-based OSS remains outside Insight. It can:

1. checksum each input dataset;
2. store SQL in a versioned file and checksum that file;
3. execute the query with pinned parameters;
4. emit the JSON shape shown in
NaN
NaN
NaN

No DuckDB dependency, connection, SQL execution, or DuckDB-specific field is
introduced into Insight.

## Compatibility

NaN
fields are tolerated for additive evolution. Renaming/removing fields,
changing semantics, or adding required fields requires a new schema version.

