# Research Loop dogfooding

The Research Loop preserves repeated investigations as append-only history. It projects the latest existing Insight analysis into structured gaps and provider-neutral data requirements; it is not a second analysis pipeline.

## Boundary

```text
provided evidence / dataset / research artifact
        ↓
semantic AnalysisMode
        ↓
existing analysis / deterministic pre-analysis
        ↓
Observation / Claim
        ↓
Connection / Mechanism Candidate
        ↓
ResearchRun → immutable ResearchIteration
        ↓                     ↓
Insight Delta        human novelty evaluation
```

The application, not the LLM, owns iteration history, provenance references, validation/identification statuses, and human evaluation. A `STRENGTHENED` history label means the validation status gained evidence support; it is not a causal probability. Suggested data or research designs are not treated as collected data or applied estimators.

An Insight may now preserve a non-obvious `Connection`, a candidate explanatory `Mechanism`, and a candidate `Generalization`. These are inspectable hypotheses, not truth labels. Narrative coherence does not promote causal or identification status, and a generalization cannot be presented as transferable until a human review explicitly supports it.

Each iteration can carry a provider-neutral `ResearchInputSnapshot`. The next iteration receives an `InsightDelta` describing which references, evidence items, variables, dimensions, filters, contexts, transforms, periods, populations, or geographies changed and which observations, hypotheses, gaps, requirements, and insights changed with them. This is a before/after audit trail, never causal attribution.

## Shortest dogfood path

Start the server and create a project using the existing UI or API. Import the existing `id,source,title,content` CSV contract and run the existing analysis. After that analysis reaches `completed`, create the first research iteration:

```bash
curl -sS -X POST http://127.0.0.1:8787/api/projects/PROJECT_ID/research-runs \
  -H 'content-type: application/json' \
  -d '{"question":"Did the intervention cause the increase?","artifactClass":"STRUCTURED_DATASET","inputReferences":["synthetic-policy.csv"],"inputSnapshot":{"references":["synthetic-policy.csv"],"variables":["treated_outcome"]}}'
```

The returned run contains structured `researchGaps`, `dataRequirements`, and `whatWeCannotConclude`. Record the required human judgment; the pipeline has no route that can self-assign this value:

```bash
curl -sS -X PUT http://127.0.0.1:8787/api/research-runs/RUN_ID/iterations/ITERATION_ID/evaluation \
  -H 'content-type: application/json' \
  -d '{"observationGrounding":4,"surpriseUsefulness":4,"hypothesisDiversity":5,"counterEvidenceQuality":3,"missingEvidenceQuality":4,"identificationHonesty":5,"nextDataUsefulness":4,"novelty":"PARTIALLY_NEW","overallUsefulness":4,"notes":"The common-trend alternative was new to me."}'
```

Add documents through the existing document or CSV endpoint, run analysis again, then append rather than replace history:

```bash
curl -sS -X POST http://127.0.0.1:8787/api/research-runs/RUN_ID/iterations \
  -H 'content-type: application/json' \
  -d '{"inputReferences":["synthetic-policy.csv","comparison-period.csv"],"inputSnapshot":{"references":["synthetic-policy.csv","comparison-period.csv"],"variables":["treated_outcome","comparison_outcome"]},"addedEvidence":["untreated comparison outcomes"],"evidenceAdditions":[{"reference":"comparison-period.csv","gapIds":["GAP_ID"]}]}'
```

Only structured `evidenceAdditions[].gapIds` may resolve a specific ResearchGap. The legacy free-text `addedEvidence` field remains readable for backward compatibility, but cannot silently close every missing-evidence item.

To compare two iterations directly:

```bash
curl -sS "http://127.0.0.1:8787/api/research-runs/RUN_ID/compare?from=ITERATION_1&to=ITERATION_2"
```

A move into `VALIDATION` requires a frozen Expectation plus concrete `validationEvidence` provenance (Expectation ID, evidence reference, independence relation, rationale). A boolean such as "independent evidence planned" is no longer sufficient.

Inspect the complete audit trail:

```bash
curl -sS http://127.0.0.1:8787/api/research-runs/RUN_ID
curl -sS http://127.0.0.1:8787/api/research-runs/RUN_ID/report.md
```

## Golden case

`internal/service/testdata/research_loop_policy.json` is fictional. The treated series rises from 100 to 130 while the comparison rises from 200 to 240. Tests require three distinct explanations and retain `NOT_IDENTIFIED`; no DiD estimate is calculated.

## Human pass/fail

A mechanically valid run is not automatically useful. The human records whether it revealed something new. Useful dogfooding should contain a grounded surprise, meaningful competing hypotheses, actual counter-evidence where available, specific missing evidence, actionable next data, and an honest identification boundary.

The core deliberately does not fetch e-Stat, RESAS, ja-company-base, web search, or customer data. Those remain adapter responsibilities. The full boundary and the export / re-import contract are in [byo-evidence-boundary.md](byo-evidence-boundary.md).
