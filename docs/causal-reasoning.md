# Evidence-Grounded Causal Reasoning Semantics

This document defines how Insight Lab represents causal questions. It is an implementation contract, not a causal-inference tutorial. See `learning-notes-causal-inference.md` for learning material.

## Boundary

Insight Lab is an evidence-grounded research and diagnostic engine. It preserves the sequence:

`data → observation → expectation → mismatch → surprise → competing hypotheses → candidate causal structure → evidence/counter-evidence → validation status`

It does not estimate causal effects, discover a true DAG, or produce assessments, recommendations, architectures, estimates, proposals, or commercial decisions. Those commercial decisions belong outside this OSS repository.

## Main audit (2026-09-13, updated 2026-09-21)

### EXISTING

- Exact-quote Observation extraction and application-side grounding.
- Repetition and deviation Pattern types; deviation stores expectation and surprising behavior.
- Abductive hypothesis generation with visible rationale.
- Structured primary / competing hypotheses grouped by `HypothesisSetID`.
- Supporting, counter, and neutral Evidence types; `CounterSearched` coverage tracking.
- Falsification criteria and structured missing evidence / `ResearchGap` / `DataRequirement`.
- Explicit causal status, validation status, and identification status.
- Minimal candidate causal structures with Variable / Relation / Role semantics.
- Deterministic guardrails preventing LLM prose, evidence counts, or confidence from promoting a claim into `CAUSALLY_SUPPORTED`.
- Application-side confidence as evidence-quality / coverage signal, not causal probability.
- Research Stage (`DISCOVERY` / `EXPLORATORY` / `VALIDATION` / `SYNTHESIS`).
- First-class Expectations with provenance, freeze-for-validation, and cross-iteration derivation.
- Persisted independent-validation evidence provenance.
- Semantic Analysis Mode (`DISCOVERY` / `DATASET_ANALYSIS` / `RESEARCH_REVIEW`) and imported Research Claims.
- Explicit gap → added-evidence linkage and append-only re-analysis.
- Connection / candidate Mechanism / Generalization semantics.
- Input snapshots and Insight Delta across research iterations.
- Versioned Research Artifact export, Promotion review, and persisted approved-artifact snapshots.
- Shared Eval / Golden regressions for the major research-integrity invariants.

### PARTIALLY IMPLEMENTED

- The engine can represent and audit candidate mechanisms, but it does not execute a causal identification design or statistical causal estimator.
- Structured data support has deterministic CSV-oriented foundations; not every large or mixed artifact format has a stable ingestion contract.
- Semantic Analysis Mode is first-class, while source-specific acquisition / format adapters remain intentionally outside or downstream from the core.

### MISSING / NOT IMPLEMENTED

These are not current capabilities and must not be inferred from the richer semantics above:

- causal effect estimation;
- automatic causal discovery or a true discovered DAG;
- automatic control-variable selection;
- autonomous external evidence acquisition;
- a generic bulk-ingestion platform for huge/mixed file collections;
- a released Go SDK or Node.js SDK.

### CONFLICTING / LEGACY COMPATIBILITY

- The old report label `Confidence` can be misread as causal probability; current documentation and semantics define it only as evidence-quality / coverage.
- Legacy Expectation provenance values `SOURCE_BACKED` and `MODEL_PROPOSED` remain readable, but they are normalized conservatively and never upgraded into prior evidence.
- Research Artifact v1 retains its historical JSON `analysisMode` key for execution mode compatibility; semantic Analysis Mode is a separate field.

### OUT OF SCOPE

- Statistical estimators, automatic causal discovery, automatic control selection, and a DAG editor.
- Evidence acquisition credentials/connectors in the OSS core.
- Customer-specific commercial recommendations, pricing, proposals, or architecture decisions.
- Media-specific narrative generation.
## Semantics and guardrails

An Observation contains only a fact directly verifiable in input. “Policy introduction was followed by more corporate-number designations” may be an observation. “The policy increased startups” is a hypothesis.

`ExpectationBasis` records where an expectation came from relative to the data it is compared against: `PRIOR`, `LITERATURE`, `DOMAIN_KNOWLEDGE`, `MODEL_PROPOSED_POST_HOC`, `HUMAN_POST_HOC`, `DERIVED_FROM_PRIOR_RUN`, `OTHER`, or `UNKNOWN` (missing provenance is kept as `UNKNOWN`, never guessed). `SOURCE_BACKED` and `MODEL_PROPOSED` are legacy values normalized on read (`MODEL_PROPOSED` → `MODEL_PROPOSED_POST_HOC`; `SOURCE_BACKED` keeps unknown timing). Provenance is never upgraded: a post-hoc or unknown value is never rewritten as `PRIOR`. Post-hoc and unknown-provenance expectations are never Evidence and never permit a strong (`VALIDATION_RESULT`) claim, even once frozen.

Each generated explanation is `CAUSAL_HYPOTHESIS`. The current pipeline performs no causal research design, so application code always records `NOT_IDENTIFIED` and can never emit `CAUSALLY_SUPPORTED`. Confidence is not consulted when assigning causal or identification status.

Validation status describes evidence state separately:

- no support: `INSUFFICIENT_EVIDENCE`
- support only: `PLAUSIBLE`
- support plus some counter-evidence: `PARTIALLY_SUPPORTED`
- counter-evidence at least as numerous as support: `CONTRADICTED`

These are conservative workflow states, not statistical conclusions.

Candidate causal structures contain Variables and directed Relations. Roles are `EXPOSURE`, `OUTCOME`, `CONFOUNDER`, `MEDIATOR`, `COLLIDER`, or `UNKNOWN`; role status is `PROPOSED`, `SUPPORTED`, or `UNKNOWN`. The application never treats a collider or merely proposed confounder as an automatic control.

Falsification criteria describe what future observation would weaken a hypothesis. Only grounded observations returned by Evidence Retrieval become actual counter-evidence.

## Research Stage and Expectation lifecycle

A `ResearchIteration` carries a `ResearchStage`: `DISCOVERY` (turning input into observations), `EXPLORATORY` (generating expectations and hypotheses from data already observed), `VALIDATION` (testing frozen expectations against evidence independent of the exploratory pass), or `SYNTHESIS` (combining results while stating remaining uncertainty). A result's `FindingKind` follows directly from the stage: only `VALIDATION` produces `VALIDATION_RESULT`, only `SYNTHESIS` produces `SYNTHESIS`, everything else is `EXPLORATORY_FINDING`. Stage never advances on its own; a human calls `TransitionResearchStage` for each forward move, and stepping back is always allowed because it never promotes a claim.

`BuildResearchIteration` projects each insight's `Expectation` / `ExpectationBasis` / `FalsificationCriteria` into a first-class `Expectation` entity on the iteration, unfrozen, with `ObservedDataAvailableAtCreation` set solely from the basis's own timing (never guessed, never used to relabel a post-hoc basis as prior). A human fixes one of these for validation via `FreezeResearchExpectation`; freezing requires known provenance and at least one falsification criterion, and it never changes the statement or provenance it fixes. `EXPLORATORY -> VALIDATION` is rejected (`ErrStageRequiresFrozenTarget`) unless the iteration carries at least one frozen, valid expectation, and (`ErrStageRequiresIndependentEvidence`) unless the caller states that evidence independent of the generating iteration has been identified for the validation run.

Appending a new iteration carries every frozen expectation from the previous iteration forward as a fresh, unfrozen validation target on the new one (`DERIVED_FROM_PRIOR_RUN`, with `DerivedFromExpectationID` recording the lineage). This is how an expectation formed while exploring one dataset becomes a pre-registered target tested against a later, independent pass — the historical iteration itself is never mutated.

Invariants enforced by the domain model:

- a post-hoc expectation's provenance is never rewritten to `PRIOR`;
- freezing never changes provenance;
- evidence from the same iteration that produced an expectation counts as independent only if the expectation was fixed before that iteration's data was observed (`Expectation.IsIndependentEvidence`);
- a validation target can be frozen before any additional evidence is seen;
- historical iterations are never overwritten — carrying an expectation forward or freezing it always produces a new value.

## External structured data

External systems remain adapters outside the domain:

`external source → adapter → normalized CSV/Document input → Insight ingestion`

Insight Lab does not depend on source-specific exporter repositories or registry-specific schemas. CSV is currently supported; JSONL adapter work remains a later phase.

## Synthetic policy scenario

For “Did municipal startup support increase company formation?”, the expected safe result is:

- observation: designations rose after policy introduction;
- competing candidates: policy effect, population inflow, redevelopment, macro trend, and registration/data artifact;
- missing evidence: control regions, pre-policy period, population/economic controls, and exact policy timing;
- identification: `NOT_IDENTIFIED`;
- validation: no stronger than `PLAUSIBLE` from association-only input.

The system may propose a control group, pre/post data, natural experiment, DiD, RDD, or IV as a candidate design. It must not claim that design was applied.

## Competing hypothesis evaluation

Alternative explanations are promoted into first-class Insight candidates before evidence retrieval. Candidates derived from the same surprising fact share a `HypothesisSetID`; their role is `PRIMARY` or `COMPETING`.

Each candidate passes independently through the existing evidence retrieval, grounding, counter-evidence, confidence, validation, persistence, and report path. Evidence for one candidate is not automatically evidence for another candidate in the set.

A set with fewer than three independently evaluated explanations receives the deterministic `insufficient_competing_hypotheses` quality warning. This is a review warning, not a reason to discard the hypothesis. Markdown reports include a per-set comparison table with support, counter-evidence, missing-evidence, validation, and identification state.
