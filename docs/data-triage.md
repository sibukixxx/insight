# AI-assisted Data Triage (#92)

Data Triage decides **what deterministic processing should look at** for a research question when a dataset has many variables. It never computes evidence, never deletes source data and never decides research outcomes.

```text
Raw / dataset bytes
  ↓  deterministic Dataset Profile (bounded metadata only)
  ↓  triage: DETERMINISTIC (default) or MODEL
  ↓  Selection Plan v1 → human revisions v2, v3 … (immutable versions)
  ↓  deterministic processing (outside triage)
  ↓  Analytical Artifact / Evidence → Research Engine
  ↓  ResearchGap / DataRequirement → re-triage (next plan version)
```

## Dataset Profile

`POST /api/public/v1/subjects/{subjectId}/dataset-profiles` profiles CSV sent inline (`csv`, at most 8 MiB) or an existing evidence document of the subject (`documentId`).

Per column: `name`, inferred `type` (`INTEGER`, `NUMBER`, `BOOLEAN`, `DATE`, `STRING`, `EMPTY`), `nonNullCount`, `nullCount` (empty, `NA`, `null`, `N/A`), `distinctCount` (capped at 10,000 with `distinctCapped`), `min`/`max`, and at most 5 sample values of at most 64 characters. The profile also records `rowCount`, `contentSha256` of the profiled bytes and `profilerVersion`.

The profile is deterministic: identical bytes give an identical `profileFingerprint`. When `dataset.sha256` is sent it must match the profiled bytes. Raw rows are never stored or returned.

## Selection Plan

`POST .../dataset-profiles/{profileId}/triage` creates the next plan version.

Every profiled column appears **exactly once**, in profile order, as a `VariableDecision`:

| bucket | meaning |
|---|---|
| `INCLUDE` | deterministic processing should compute with it |
| `DEFER` | kept for later; not dropped |
| `NEEDS_REVIEW` | definition or relevance unclear — UNKNOWN is preserved |
| `EXCLUDE` | a **human** chose not to process it; source data is untouched |

Each decision has a `rationale`, an optional `role` (`METRIC`, `DIMENSION`, `PERIOD`, `IDENTIFIER`), optional candidate `classifications` (`POSSIBLE_OUTCOME`, `POSSIBLE_EXPOSURE`, `POSSIBLE_CONFOUNDER`, `POSSIBLE_MEDIATOR`, `POSSIBLE_COLLIDER`, `COMPARISON_CANDIDATE`, `DEFINITION_UNKNOWN`) and `linkedGapIds`.

Classifications are hypotheses about a variable. Semantic relevance is not causal evidence and correlation is not causation.

### Normalization invariant

Every proposer, deterministic, model or human, passes through the same rules:

- a column that is not in the profile is rejected (`INVALID_REQUEST`)
- a column the proposer omitted becomes `NEEDS_REVIEW` with `DEFINITION_UNKNOWN`
- the first placement of a duplicated column wins
- an automated `EXCLUDE` is downgraded to `DEFER`; only `HUMAN` revisions may exclude

### Proposers

- `DETERMINISTIC` (default, no model needed): period-like columns and low-cardinality categories are included; numeric columns are included when their name matches the question or a gap and deferred otherwise; row-unique text is deferred as an identifier; empty columns need review. It never assigns causal classifications.
- `MODEL`: sends only the bounded profile, question, hypothesis IDs and gaps to the configured model with a strict JSON schema. The model cannot exclude and its output is normalized as above. Without a configured model the request fails with `INVALID_REQUEST`.

`proposer` records `kind`, `actor` (for humans) and `model`.

### Versions, revisions and re-triage

- Plans are immutable. Each new plan is `version + 1` with `parentPlanId`.
- `POST /api/public/v1/selection-plans/{planId}/revisions` applies human `moves` (`name`, `toBucket`, optional `role`/`classifications`, required `rationale`). The stored move records `fromBucket`, so any move can be reversed by a later one.
- A re-triage records how it differs from the latest version as `moves`.
- `gaps` in the triage request, or `researchRunId` (unresolved ResearchGaps and DataRequirements of that run's latest iteration), make the triage gap-aware: deferred or unclear variables whose names match a gap are re-proposed as `INCLUDE` with `linkedGapIds`.

Every plan states its `processingBoundary`: numeric results come solely from deterministic processing of the source data.

## Operations

| op | method and path |
|---|---|
| `createDatasetProfile` | `POST /api/public/v1/subjects/{subjectId}/dataset-profiles` |
| `getDatasetProfile` | `GET /api/public/v1/subjects/{subjectId}/dataset-profiles/{profileId}` |
| `triage` | `POST /api/public/v1/subjects/{subjectId}/dataset-profiles/{profileId}/triage` |
| `listSelectionPlans` | `GET /api/public/v1/subjects/{subjectId}/dataset-profiles/{profileId}/selection-plans` |
| `getSelectionPlan` | `GET /api/public/v1/selection-plans/{planId}` |
| `reviseSelectionPlan` | `POST /api/public/v1/selection-plans/{planId}/revisions` |

Schemas are in `contracts/public-engine/v1/schema.json`. Conformance fixtures `14-data-triage.json` and `15-data-retriage-from-research-gaps.json` cover the invariant, versioning, reversible exclusion and gap-driven re-triage.

## Boundary

Triage is generic. Consumer-specific selection policy, human review workflow and cost-aware routing belong to the consumer. Profiles reference datasets by id, version and hash; how raw input is sourced is the InputSource contract (#90).
