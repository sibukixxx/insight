# Project Status

_Last reviewed: 2026-09-20_

This document describes the current project state. It is intentionally more conservative than roadmap or design documents.

## Current stage

Insight Lab has moved beyond a simple “find patterns and summarize them” pipeline.

The implemented reasoning path now covers:

```text
Data
→ Observation
→ Expectation
→ Expectation mismatch / surprising fact
→ Primary + competing hypotheses
→ Independent evidence / counter-evidence evaluation
→ Missing evidence / falsification criteria
→ Candidate causal structure
→ Validation status
→ Identification status
→ Auditable report
```

## Implemented foundation

### Grounding and observations

- Source-backed observations and quote grounding.
- Pattern and deviation/trace representation.
- Separation between directly observed facts and generated explanations.

### Hypothesis reasoning

- Abductive hypothesis generation.
- Structured competing hypotheses.
- `HypothesisSetID` and `PRIMARY` / `COMPETING` roles.
- Independent evidence and counter-evidence processing for candidates in the same hypothesis set.
- Deterministic warning when too few competing explanations are independently evaluated.

### Causal guardrails

- Explicit causal, validation, and identification status.
- Candidate causal structures and variable roles.
- Separation of falsification criteria from actual counter-evidence.
- Model-proposed expectations are not automatically evidence.
- Application confidence is not interpreted as causal probability.
- The current pipeline does not automatically emit `CAUSALLY_SUPPORTED`.

### Product mechanics

- Local-first SQLite persistence.
- Text and CSV ingestion.
- OpenAI-compatible model configuration.
- Markdown report export.
- Production and fictional-demo builds.
- Repeatable `make eval-demo` workflow.

### External dataset provenance (BYO-Evidence)

- Deterministic dataset pre-analysis for reproducible external-data runs, independent of any model call.
- `ExecutionMode` distinguishes deterministic-only runs from model-backed ones (`RunProvenance`). It is deliberately not named `AnalysisMode`: that name is reserved for the semantic Discovery / Dataset Analysis / Research Review split #18 is still completing (see #38).
- Validated acquisition manifest schema (source, retrieval method, retrieval time, dataset id, hashes, caveats); credential-looking fields are rejected.
- Dataset hash provenance and cross-dataset compatibility warnings (unit / population scope / period granularity / schema version mismatches).
- See [byo-evidence-boundary.md](byo-evidence-boundary.md) for what Insight Lab does and does not fetch itself.

### Research Loop, Stage and artifact export

- Append-only `ResearchRun` / `ResearchIteration` history; prior iterations are never overwritten.
- `ResearchGap` / `DataRequirement`, including gap dependencies and discriminating-power priority ordering.
- Decision readiness assessment, explicit stop reasons, and human override / evaluation handoff.
- `ResearchStage` (`DISCOVERY` / `EXPLORATORY` / `VALIDATION` / `SYNTHESIS`) with guarded transitions, wired into the service, usecase and Markdown report layers.
- Expectation provenance (`SOURCE_BACKED` vs `MODEL_PROPOSED`) guards against treating a post-hoc explanation as a prior prediction.
- Versioned, machine-consumable Research Artifact JSON export (`/api/research-runs/{id}/artifact.json`, schema v1), including the latest iteration's Research Stage (#37).
- Requirement linkage from an acquired evidence item back to the `DataRequirement.gapId` it resolves is not yet implemented; `addedEvidence` remains free text (tracked in #39).

### Promotion Gate

- Domain and service layer implemented (rules for when a claim is fit to promote into a public report).
- Wired into the usecase (`SubmitPromotionReview`, `TransitionPromotionState`), HTTP (`PUT /api/research-runs/{id}/iterations/{id}/promotion-review`, `.../promotion-transition`), and Markdown report (`## Promotion Status`) layers.
- The versioned Research Artifact JSON export (`artifact.json`) does not yet surface promotion status (tracked in #24).

## Known limitations

- Insight Lab does not estimate causal effects.
- `NOT_IDENTIFIED` is expected for causal questions that only contain observational association without an appropriate identification design.
- Suggested DiD/RDD/IV/natural-experiment approaches are recommendations for validation, not analyses that have been executed.
- External structured-data sources require adapters; the core does not contain source-specific schemas.
- CSV ingestion exists; a generic JSONL adapter is not yet part of the documented stable path.
- LLM quality remains model- and data-dependent, so generated hypotheses require human review.

## Current development priority: dogfooding before expansion

The next milestone is not another large causal feature. It is repeatable evaluation on real or carefully curated public-data cases.

The initial target is a small Golden Dogfooding set with different causal difficulty levels, for example:

1. temperature and heat-related emergency transports;
2. municipal startup support and company-formation/designation activity;
3. inbound tourism and local economic outcomes.

Each case should define expected behavior before running the model, including:

- observations the system must detect;
- causal claims it must not make;
- important alternative explanations;
- expected missing evidence;
- expected identification state;
- human usefulness of the resulting next-validation steps.

The purpose is to discover systematic failure modes. Implementation work should then target observed failures rather than speculative features.

## Success criteria for the next phase

A useful result does not require proving causality.

The system should reliably help a reviewer move from:

```text
interesting correlation
```

toward:

```text
grounded observation
+ competing explanations
+ counter-evidence
+ missing evidence
+ explicit identification limits
+ concrete next validation step
```

A correct `NOT_IDENTIFIED` or `INSUFFICIENT_EVIDENCE` result is considered successful when the available data cannot support a stronger conclusion.

## Not a promise of future features

This status document describes direction, not a commitment to implement statistical estimators, automatic causal discovery, commercial decision logic, or domain-specific integrations.

Future work should be driven by dogfooding evidence and contributor demand.
