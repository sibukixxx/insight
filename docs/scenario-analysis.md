# Scenario analysis (#66)

Scenario analysis adds prospective reasoning to a research run. It is not a forecaster: the engine never predicts, never generates probabilities and never picks a "most likely future". It records several possible futures that branch from one observed baseline and tracks how later evidence treats each branch.

```text
Observed baseline (asOf)
  -> ScenarioSet (versioned, append-only)
       S1 .. Sn: assumptions, mechanisms, frozen expectations, falsification conditions
  -> later observations (IndicatorObservation / AssumptionCheck)
  -> ScenarioEvaluation (append-only) with per-branch status and ScenarioDelta
```

## Semantics

| Concept | Meaning |
| --- | --- |
| `ScenarioSet` | Research question, `baseline.asOf` (required), branches, explicit `nonExhaustive` flag, shared evidence refs, declared relations. Curating creates a new `version`; older versions stay readable. |
| `Scenario` | One branch: `horizon` (required, after `asOf`), assumptions, mechanisms, expectations, disconfirming observations, affected dimensions, evidence refs, unresolved gaps, limitations. |
| `Assumption` | Typed: `OBSERVED_BASELINE`, `EXOGENOUS_ASSUMPTION`, `MODEL_ASSUMPTION`, `HUMAN_ASSUMPTION`, `POLICY_ASSUMPTION`. Only `OBSERVED_BASELINE` may cite `evidenceRefs` as observed fact; every other kind cites `sourceRefs`. |
| `ScenarioExpectation` | Frozen statement about a future observation: indicator, optional direction/range, `observationWindow`, required `falsificationCondition`, and `ExpectationBasis` provenance. Every scenario needs at least one expectation testable after `asOf`. A pre-observation basis with `observedDataAvailableAtCreation: true` is rejected as post-hoc leakage. |
| `ScenarioProbability` | Optional and caller-supplied only, with `basis` and `sourceRefs`. The engine never creates one. |
| `ScenarioRelation` | `MUTUALLY_EXCLUSIVE`, `OVERLAPPING`, `NESTED` or `CONFLICTING`. Scenarios need not be exclusive; declared relations must name existing branches. |
| `ScenarioStatus` | `UNTESTED`, `CONSISTENT_SO_FAR`, `WEAKENED`, `CONTRADICTED`, `INCONCLUSIVE`. Evidence state, never likelihood. |
| `ScenarioEvaluation` | Append-only evaluation of one set version. It carries forward earlier observations, so history is never rewritten. |
| `ScenarioDelta` | Strengthened / weakened / contradicted / unchanged branches, invalidated assumptions, fired falsification conditions, newly required evidence and an explanation. No ranking. |

### Evaluation rules

- A decisive outcome (`CONSISTENT`, `CONTRADICTS`) needs an `evidenceRef`.
- An observation at or before the set's `createdAt`, or outside the expectation's `observationWindow`, cannot test the branch. It is kept for audit and counted as `INCONCLUSIVE` with a reason.
- Any fired falsification condition makes the branch `CONTRADICTED`; an invalidated assumption makes it `WEAKENED`; otherwise at least one consistent observation gives `CONSISTENT_SO_FAR`.
- Every expectation without a decisive observation becomes a scenario-specific `DataRequirement` (`gapId: scenario:<scenarioId>:<expectationId>`).

### Deterministic scaffold

`scaffold` drafts one branch per hypothesis of an iteration that is not `CONTRADICTED`, plus one per competing hypothesis. Each falsification criterion becomes an expectation with `DERIVED_FROM_PRIOR_RUN` provenance over `[baseline.asOf, horizon.end]`. The caller supplies baseline and horizon. Hypotheses without a falsification criterion are skipped and reported, not given invented expectations. No model or API key is needed. Indicators are placeholders until a human names concrete measures; curate by submitting a new set version.

## API

Internal UI API:

- `GET /api/research-runs/{runId}` — each iteration gains `scenarios` (branch plus `status`, `kind: "SCENARIO"`, `scenarioSetId`, `setVersion`, `baselineAsOf`, `nonExhaustive`) and `scenarioDelta` when that iteration was evaluated. The stored iteration payload is unchanged.
- `GET /api/research-runs/{runId}/scenarios`
- `POST /api/research-runs/{runId}/scenario-sets` (ScenarioSet body; id/version/createdAt assigned)
- `POST /api/research-runs/{runId}/scenario-sets/scaffold` (`{iterationId?, baseline, horizon}`)
- `POST /api/research-runs/{runId}/scenario-sets/{setId}/evaluations` (`{iterationId?, observations, assumptionChecks}`)

Public Engine Contract v1 (`contracts/public-engine/v1/schema.json`): `getScenarios`, `createScenarioSet`, `scaffoldScenarioSet`, `evaluateScenarios` under `/api/public/v1/research-runs/{researchRunId}/…`, all idempotent. The Research Artifact gains an additive `scenarioAnalysis` field that appears only when the run has scenario sets. Fixture `13-scenarios.json` covers the contract.

## Fixture

`internal/domain/testdata/scenario_japan_eu.json` is the deterministic Japan → EU export fixture with five branches (FX pass-through, freight/tariff absorption, substitution, discretionary demand fall, category-specific demand). Golden tests (`go test -tags golden ./internal/goldenset/`) cover false certainty, post-hoc leakage, missing horizon and conflicting scenarios.

## Out of scope

Model-backed scenario generation, autonomous web research, FX or price prediction and commercial recommendation. Product UI lives downstream (formerly #67).
