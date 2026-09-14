# Research Loop dogfooding

The Research Loop preserves repeated investigations as append-only history. It projects the latest existing Insight analysis into structured gaps and provider-neutral data requirements; it is not a second analysis pipeline.

## Boundary

```text
CSV / text → existing analysis → ResearchRun → immutable ResearchIteration
                                      ↓
                         human novelty evaluation
```

The application, not the LLM, owns iteration history, provenance references, validation/identification statuses, and human evaluation. A `STRENGTHENED` history label means the validation status gained evidence support; it is not a causal probability. Suggested data or research designs are not treated as collected data or applied estimators.

## Shortest dogfood path

Start the server and create a project using the existing UI or API. Import the existing `id,source,title,content` CSV contract and run the existing analysis. After that analysis reaches `completed`, create the first research iteration:

```bash
curl -sS -X POST http://127.0.0.1:8787/api/projects/PROJECT_ID/research-runs \
  -H 'content-type: application/json' \
  -d '{"question":"Did the intervention cause the increase?","inputReferences":["synthetic-policy.csv"]}'
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
  -d '{"inputReferences":["comparison-period.csv"],"addedEvidence":["untreated comparison outcomes"]}'
```

Inspect the complete audit trail:

```bash
curl -sS http://127.0.0.1:8787/api/research-runs/RUN_ID
curl -sS http://127.0.0.1:8787/api/research-runs/RUN_ID/report.md
```

## Golden case

`internal/service/testdata/research_loop_policy.json` is fictional. The treated series rises from 100 to 130 while the comparison rises from 200 to 240. Tests require three distinct explanations and retain `NOT_IDENTIFIED`; no DiD estimate is calculated.

## Human pass/fail

A mechanically valid run is not automatically useful. The human records whether it revealed something new. Useful dogfooding should contain a grounded surprise, meaningful competing hypotheses, actual counter-evidence where available, specific missing evidence, actionable next data, and an honest identification boundary.

The core deliberately does not fetch e-Stat, RESAS, ja-company-base, web search, or customer data. Those remain adapter responsibilities.
