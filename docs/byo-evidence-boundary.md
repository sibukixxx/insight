# BYO-Evidence boundary

Insight Lab is a **Bring Your Own Evidence** OSS research / reasoning engine. It reasons over evidence that is handed to it. It does not go and get evidence. This page fixes that boundary (issue #21, closed) so that features are not added on the wrong side of it.

The boundary is not "stop when evidence is missing". Insight Lab must be able to say precisely what is missing, hand that requirement out, accept the acquired evidence back, and continue the same research as a new iteration.

## What Insight Lab does

- Reasoning over provided documents and CSV datasets. A first-class, explicitly-selected Discovery / Dataset Analysis / Research Review mode split is being formalized in #18; today the reasoning path is the same pipeline regardless of input maturity.
- Deterministic dataset analysis (aggregates, differences, comparability labels) outside the LLM.
- Observation and Claim extraction with grounding.
- Expectation / Mismatch (Surprise) with provenance (`SOURCE_BACKED`, `MODEL_PROPOSED`).
- Primary and competing hypotheses; supporting / counter / neutral evidence.
- Validation, causal and identification statuses, kept conservative (see [causal-reasoning.md](causal-reasoning.md)).
- `ResearchGap` and `DataRequirement` as structured statements of what is missing.
- Append-only `ResearchIteration` history; re-analysis when added evidence is supplied.
- Research Artifact / report export: Markdown report and a versioned JSON artifact (`GET /api/research-runs/{id}/artifact.json`, schema `insight-lab.research-artifact` v1).
- Local or self-hosted analysis.

## What Insight Lab does not do (hard boundary)

Insight Lab itself never fetches missing evidence from an external service. Out of scope for the OSS core:

- autonomous web-search orchestration;
- automatic search of or authenticated retrieval from e-Stat, RESAS, the National Tax Agency registry, or similar sources;
- managed SaaS / Drive / CRM connectors;
- customer-specific credential management (for example `HOUJIN_APP_ID` handling belongs to `ja-company-base`, not here);
- continuous external monitoring;
- an autonomous "research agent" that loops fetch → analyze without a human or a private layer in between;
- commercial recommendation, pricing or proposal generation (already out of scope in [project-scope.md](project-scope.md)).

The public evidence report under `reports/japan-company-count-2021-2024` follows this rule: its reference table was downloaded and checksummed by a person, recorded in `ACQUISITION.md`, and normalized into a CSV before Insight Lab code touched it.

## The loop with the boundary drawn in

```text
Provided Evidence
      ↓
Insight Lab
      ↓
Evidence Reasoning
      ↓
ResearchGap / DataRequirement
      ↓
━━━━━━━━━━━━ BOUNDARY ━━━━━━━━━━━━
      ↓
external / private acquisition   (adapters, humans, a private orchestration layer)
      ↓
Additional Evidence + acquisition manifest
      ↓
━━━━━━━━━━━━ BOUNDARY ━━━━━━━━━━━━
      ↓
Insight Lab: new ResearchIteration
      ↓
Re-analysis
```

Only the **acquisition responsibility** leaves Insight Lab. The research loop itself returns to it.

## External and AI artifacts as input

Reports from ChatGPT / Claude, external research firms, consultants or news articles can be supplied as Research Artifacts. They are inputs, not evidence of the same rank as primary data:

- an external claim is not an Observation;
- an AI answer is not primary evidence;
- a citation that cannot be followed back to its source limits evidence quality;
- AI output is never used directly as validation evidence.

The golden case `GS-07` in `testdata/golden` encodes this: an `ai_output` or `external_artifact` item that is not traceable to a primary source cannot be counted as `independent_validation`.

## Integration contract

Insight Lab exports to a private or downstream layer:

| Export | Status today |
|---|---|
| Research Artifact (Markdown report) | implemented (`/api/research-runs/{id}/report.md`), includes the current Research Stage |
| Research Artifact (versioned JSON) | implemented (`/api/research-runs/{id}/artifact.json`, schema v1, #17) |
| current Research Stage on the JSON artifact | tracked in the domain (`ResearchIteration.Stage`, #26) and in the Markdown report, but not yet a field on the JSON artifact; closing that gap is tracked in #37 |
| Claim / Hypothesis states (validation, identification) | implemented in `ResearchIteration.hypothesisStates` |
| `ResearchGap` | implemented |
| `DataRequirement` | implemented as a struct and exported on the JSON artifact (#17) |
| provenance references | implemented (`inputReferences`) |
| what-we-cannot-conclude | implemented |

Insight Lab re-imports from that layer:

| Import | Status today |
|---|---|
| acquired dataset / artifact | via the existing CSV / document ingestion |
| acquisition manifest (source, retrieval method/time, dataset id, hashes, caveats) | implemented as a validated JSON schema (`AcquisitionManifest`, #16); credential-looking fields are rejected |
| dataset compatibility warnings (unit / population / period / schema version mismatch across manifests) | implemented (`CheckDatasetCompatibility`) |
| requirement linkage (`DataRequirement.gapId` → added evidence) | not yet implemented; `ResearchIteration.addedEvidence` is still free text with no explicit link back to the `gapId` it resolves. Tracked in #39 |
| collection timestamp | implemented at the dataset level via `AcquisitionManifest.retrievedAt`; a free-text `addedEvidence` entry supplied without a manifest still has no timestamp |

New iterations append; they never overwrite a previous iteration.

## Design test

Before adding a feature that touches an external system, ask:

> Does this feature *reason about* evidence, or does it *go and get* evidence?

Reasoning belongs here. Getting belongs to an adapter, a human, or the private layer, and the result comes back through the generic ingestion boundary.

## Related

- [project-scope.md](project-scope.md) — public / private and external-data boundaries
- [research-loop.md](research-loop.md) — append-only iterations and the dogfood path
- `testdata/golden/README.md` — invariants that keep external artifacts from being promoted to primary evidence
- Issues #7 (Research Loop), #16 (reproducible external dataset runs, shipped), #17 (artifact export, shipped), #18 (multi-mode analysis, in progress), #21 (this boundary, closed), #23 (decision-ready loop gate, shipped), #37 (Research Stage on the JSON artifact), #39 (requirement linkage by `gapId`)
