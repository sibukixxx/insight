# Evidence-Grounded Causal Reasoning Semantics

This document defines how Insight Lab represents causal questions. It is an implementation contract, not a causal-inference tutorial. See `learning-notes-causal-inference.md` for learning material.

## Boundary

Insight Lab is an evidence-grounded research and diagnostic engine. It preserves the sequence:

`data → observation → expectation → mismatch → surprise → competing hypotheses → candidate causal structure → evidence/counter-evidence → validation status`

It does not estimate causal effects, discover a true DAG, or produce assessments, recommendations, architectures, estimates, proposals, or commercial decisions. Those commercial decisions belong outside this OSS repository.

## Main audit (2026-09-13, updated 2026-09-20)

### EXISTING

- Exact-quote Observation extraction and application-side grounding.
- Repetition and deviation Pattern types; deviation stores expectation and surprising behavior.
- Abductive hypothesis generation with a visible rationale.
- Supporting, counter, and neutral Evidence types; `CounterSearched` coverage tracking.
- Application-side confidence based on evidence strength, coverage, source diversity, frequency, and a counter-evidence penalty.
- Grounding and quality checks, Markdown report export, demo fixtures, tests, Makefile, and GitHub Actions.
- CSV ingestion through the generic Document boundary and `make eval-demo`.
- Research Stage (`DISCOVERY` / `EXPLORATORY` / `VALIDATION` / `SYNTHESIS`) gating what a finding may be labeled, wired through the domain, service, usecase, and report layers.
- First-class `Expectation` entities with a nine-value provenance vocabulary, freeze-for-validation, and cross-iteration derivation, wired into the Research Loop (`BuildResearchIteration`, `AppendResearchIteration`, `TransitionResearchStage`) and the versioned Research Artifact export.

### PARTIALLY_IMPLEMENTED

- Alternative interpretation existed as prose, but multiple structured competing hypotheses did not.
- Counter-evidence existed, but falsification criteria and missing evidence were not represented separately.
- The learning note described causal concepts, but runtime semantics did not represent status, identification, or a candidate graph.

### MISSING

- Causal status, validation status, and identification status.
- Minimal Variable/Relation/Role candidate causal structure.
- Structured missing evidence and next-validation requirements.
- A deterministic guardrail preventing LLM output or a high confidence score from promoting a claim to `CAUSALLY_SUPPORTED`.

### CONFLICTING

- The old report label `Confidence` could be read as causal probability. It is now explicitly labeled as an evidence-quality score.
- Existing product/monetization fields predate this foundation. They remain for backward compatibility, but causal validation never depends on them.

### OUT_OF_SCOPE

- Statistical estimators, automatic causal discovery, automatic control selection, and a DAG editor.
- Article generation and TechVit commercial decision logic.

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

`ja-company-base → CSV/JSONL adapter → Insight document ingestion`

Insight Lab does not depend on `ja-company-base` and does not contain NTA-specific fields. CSV is currently supported; JSONL adapter work remains a later phase.

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
