# Longitudinal research (#71) and the temporal flow (#68)

Longitudinal research asks the **same Research Question again as the world
moves on** and keeps how the research interpretation changed. It is different
from time-series analysis:

- time-series analysis (#70, #73): *how did the values move?*
- longitudinal research (#71): *how did new evidence change the earlier
  hypotheses and understanding?*

It reuses ResearchRun, ResearchIteration, HypothesisChange, InsightDelta and
the run identity/snapshot from #81/#82. There is no second research aggregate.

## End-to-end flow

```text
producer (SQL / DuckDB / dataframe)
   │ temporal Analytical Artifact (#69/#70: period, observedAt, metric version)
   ├──► optional Temporal Analytics Pack (#73): yoy, cagr, anomaly, change point …
   │        → derived Analytical Artifact (origin=derived, still neutral)
   ▼
addEvidence → analysis run (#82 snapshot + fingerprints)
   ▼
research iteration T0 ── observationWindow{asOf}
   ▼  new evidence, new analysis run
research iteration T1 ── observationWindow{asOf ≥ T0}, previousIterationId
   ▼  …
GET /research-runs/{id}/timeline  → evidence lane | observation deltas |
                                    hypothesis events | insight versions |
                                    instrument lane (separate)
```

## Recording iterations

`createResearchRun` and `appendIteration` (public v1 and internal API) accept
an optional `observationWindow`:

```json
{"asOf": "2024-12-31T23:59:59Z", "start": "2022", "end": "2024", "basis": "calendar-year"}
```

- `asOf` is the latest instant whose evidence the iteration may use.
- A new iteration may not use an `asOf` earlier than the latest recorded one;
  the request fails with `INVALID_REQUEST`. History is not rewritten.
- `previousIterationId` is recorded automatically.
- Iterations remain append-only. Existing stop/reopen and human-override
  behavior is unchanged.

## Timeline read model

`GET /api/public/v1/research-runs/{researchRunId}/timeline` (and the internal
`GET /api/research-runs/{runID}/timeline`) return a `ResearchTimeline` derived
from stored iterations and the analyses they are bound to. Building it never
mutates anything, and entries for earlier iterations are identical before and
after a new iteration is appended.

| Array | Lane | Meaning |
| --- | --- | --- |
| `iterations` | — | frozen interpretation per iteration: window, readiness, stop, unresolved gaps, cannot-conclude statements, and which of them were carried over or resolved |
| `evidenceEvents` | world/evidence | `ADDED`, `REMOVED`, `CONTENT_CHANGED`, `DEFINITION_CHANGED`, with linked gap IDs |
| `observationDeltas` | world/evidence | deterministic #70 deltas between the latest window of each series in consecutive analyses |
| `hypothesisEvents` | interpretation | hypothesis evolution with the iteration's evidence references and an attribution |
| `insightVersions` | interpretation | insight IDs and InsightDelta explanation per iteration |
| `instrumentChanges` | instrument | execution fingerprint `CHANGED` or `UNKNOWN` between consecutive analyses, with changed snapshot fields |

Rules:

- **Removal needs a declared set.** `addedEvidence` is incremental, so only a
  caller-declared input set (`inputReferences` / artifact references) can show
  that something was removed.
- **Definition vs. world change.** A changed metric version, population,
  geography, dimensions, spec or filters is a `DEFINITION_CHANGED` event and
  suppresses arithmetic in the observation delta. A restated value for the same
  window is `CONTENT_CHANGED`.
- **Instrument changes never enter the evidence lane.** An empty fingerprint is
  `UNKNOWN`, never `SAME`, and is not claimed as an instrument change.
- **Attribution is bookkeeping, not causation.** `EVIDENCE_DELTA`,
  `INSTRUMENT_CHANGE`, `EVIDENCE_AND_INSTRUMENT` or `UNATTRIBUTED` states what
  the change can be tied to in the record; it never says why the world changed.
- `WEAKENED` and `CONTRADICTED` set `invalidatesPriorInterpretation`: the earlier
  interpretation stays frozen but is no longer current.

## Checks

- `internal/service/longitudinal_timeline_test.go`: three Japan→EU periods,
  separate lanes, evidence-tied hypothesis changes, carried uncertainty,
  immutability after append, definition change, unknown fingerprints,
  retroactive windows, incremental evidence.
- Conformance fixture `contracts/public-engine/v1/fixtures/12-longitudinal-timeline.json`
  re-observes the Japan→EU export question over three windows and checks that
  a retroactive as-of is rejected.

## Not included

- Scenario expectations (#66) can later be evaluated against new observations
  through the same timeline; scenario semantics themselves are #66.
- Product timeline/workbench UI is downstream and not part of the OSS engine.
