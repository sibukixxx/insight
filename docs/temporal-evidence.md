# Temporal evidence and Observation Delta

Temporal data extends Analytical Artifact v1 (#69) and the existing Observation
and Evidence. It does not introduce another observation or run aggregate.
AnalysisID remains the ownership boundary from #82; no migrations or backfill
from #81/#82 are repeated.

## Input contract

Opt in by adding `metrics[].version` and `results[].temporal`:

```json
{
  "observedAt": "2024-12-31T23:59:59Z",
  "origin": "observed",
  "geography": "JP→EU",
  "valueBasis": "nominal"
}
```

- `origin`: observed or derived. Neither asserts a cause.
- `valueBasis`: nominal, real or not_applicable.
- `period`: event window, with inclusive start/end and explicit calendar basis.
  Supported bases are calendar-year (YYYY), calendar-month (YYYY-MM), and
  calendar-day (YYYY-MM-DD). A result may inherit the artifact's basis.
- `observedAt`: measurement time, distinct from provenance.retrievedAt
  (acquisition) and artifact.generatedAt (analysis/production).
- Metric definition includes version, unit, description and aggregation.
- Population, geography, dimensions, parameters and filters define scope.
- Missing and null remain unknown; they never become zero.

Legacy v1 artifacts without these optional additions import unchanged. Legacy
Observation/Evidence JSON omits temporal metadata. Migration 010 adds nullable
JSON columns; it leaves existing Analysis identity and rows untouched.

Shared leaf types were moved to internal/analytical/model and aliased from
the existing analytical package, preserving the public Go contract names.

## Producing and comparing observations

External Go consumers use `insight-lab/contracts/analytical`:

```go
candidates, err := analytical.ToCandidatesForAnalysis(artifact, analysisID)
// Handle err. analysisID must identify an existing Analysis owned by the caller.
previous := candidates[0].Observation
current := candidates[1].Observation
delta := analytical.CompareObservations(previous, current)
```

Candidates are neutral Evidence, never Insights. The adapter binds the supplied
AnalysisID and derives deterministic candidate IDs scoped to it. It cannot look
up or authorize runs; the host application must verify ownership before storing
them and persist the corresponding source Document.

Temporal metadata carries an immutable copy of the original artifact ID/hash,
result index, dataset versions/hashes, query/spec reference/hash, source records,
quality flags, definitions, values and measurement times. Existing SQLite
Observation and Evidence repositories retain that projection on every read path.

`CompareObservationSeries` compares adjacent observations in the supplied order,
supporting three or more windows and repeated runs without choosing the newest
Analysis. The caller must select the same series explicitly. Temporal dimensions
belong in period: a changing dimension called "year" is not silently ignored.

## Delta semantics

The result includes previous/current Observation and Analysis references,
baseline/comparison temporal evidence, changed dimensions, data-definition
changes, source changes, validity, magnitude, direction, warnings and limitations.

- Absolute delta = current − previous, in the metric unit.
- Relative delta = (current − previous) / abs(previous), a ratio, not a percent.
- A zero baseline permits absolute delta but omits relative delta with a warning.
- Arithmetic uses finite IEEE-754 float64 values; overflow suppresses the affected
  delta. It is descriptive analysis, not a financial decimal ledger.
- Different metric definitions/versions, units, populations, geography, dimensions,
  value basis, origin, parameters, filters or query/spec suppress arithmetic.
- Periods must be ordered, non-overlapping, and have equal basis and calendar
  window counts. Calendar months need not have equal day counts.
- Missing ownership, missing/non-numeric values and invalid provenance also
  suppress arithmetic. `valid=false` never carries a magnitude or direction.
- Dataset/source revisions retain both histories and emit a review warning.
  Validity checks structural comparability, not source accuracy or causal validity.
- Quality flags stay attached to both endpoints and emit a review warning.
- A nominal change is not an inflation/FX-adjusted real change.
- Change, anomaly and correlation are not causal effects. No automatic insight,
  causal status or promotion transition occurs.

## Temporal Analytics Pack (#73)

Declarative, versioned operations turn temporal results into a **derived**
Analytical Artifact. Spec: `contracts/analytical-artifact/v1/temporal-operation.schema.json`
(`insight-lab.temporal-operation` v1). Go: `analytical.ApplyTemporalOperation`
in `insight-lab/contracts/analytical`. HTTP: stateless
`POST /api/public/v1/temporal-operations` (nothing is stored).

Operations: `yoy`, `mom`, `qoq`, `period_over_period`, `cagr`, `rolling_mean`,
`rolling_delta`, `share`, `unit_price`, `indexed_baseline`, `lagged_comparison`,
`before_after`, `cohort_comparison`, `control_comparison`, `anomaly_candidate`,
`change_point_candidate`.

- Output results are `origin: derived`, carry a versioned metric per operation
  component, and keep the source geography and value basis (ratios become
  `not_applicable`).
- The derivation is a pure function of (source artifact, spec): it inherits the
  source `generatedAt`, its ID derives from the source hash and spec hash, and
  `spec.hash` is the SHA-256 of the spec. An external producer (for example a
  DuckDB job) implementing the same spec yields the same reproducibility key.
- Provenance keeps the source datasets and appends `artifact:<source id>` and
  `temporal-operation:<op>@1:sha256:<spec hash>` transformation references.
- Unreported windows inside a series become explicit gaps. Missing and gap
  inputs produce missing outputs with a quality flag; unknown never becomes 0.
- A selection that matches more than one series is rejected rather than merged.
- Each operation adds `LIMITATION` quality flags. Anomaly and change-point
  results are candidates. Lag correlation is association only. No output uses
  causal wording; a test scans every operation's JSON for it.

Derived artifacts enter research like any Analytical Artifact: submit them with
`addEvidence`, and they become neutral Observation candidates.

See [longitudinal-research.md](longitudinal-research.md) for how temporal
evidence feeds repeated research iterations and the timeline.

## Acceptance checks

`make test` covers the synthetic Japan→EU 2022–2024 fixture (100, 115, 121),
deterministic deltas (15 and 6), incompatible comparisons, provenance, missingness,
zero baseline, calendar periods, run-scoped identity, legacy artifacts/JSON and
SQLite persistence. `make vet` covers both normal and demo builds.

Fixture: contracts/analytical-artifact/v1/fixtures/temporal-japan-eu.json.
Its example.invalid sources and placeholder hashes are synthetic contract data,
not verified trade statistics. There is no UI or application-specific behavior.

Synthetic trade+FX fixture for the pack:
contracts/analytical-artifact/v1/fixtures/trade-fx-temporal.json (monthly,
with a missing window, an unreported window and a spike; not real statistics).
Conformance: contracts/public-engine/v1/fixtures/16-temporal-operation-pack.json.
