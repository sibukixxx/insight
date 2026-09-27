# Claim Inspection v1 (#119)

Claim Inspection checks statements made elsewhere — by a model, a person or another application — against the research state Insight already holds. It is a research debugger, not a fact checker: it shows what the existing evidence and hypotheses can and cannot carry, and never proves a claim true or false.

## Scope of v1

- **In:** caller-supplied claims (`id`, `statement`, `sourceReference`, `evidenceReferences`, optional `hypothesisReferences`, `assumptions`) submitted with `createResearchRun` / `appendIteration` (public contract) or the internal research API.
- **Out:** splitting free text into claims. That needs a model and is left to callers (for example an MCP or consumer step that extracts claims and then submits them here). The research use case never calls a model.
- Claims are accepted in every semantic mode; inspection runs whenever claims are present.

## How a claim is inspected (`claim-inspection/v1`)

1. **Citations.** Each `evidenceReference` is resolved against the subject's existing observations and documents (by observation ID, document ID or the document's public `externalRef`). A reference that is not there is `NOT_FOUND` and flagged `CITATION_NOT_FOUND`. A claim without references is flagged `NO_SOURCE_CITATION`. Nothing is ever created from a claim: no document, observation or evidence.
2. **Basis.** With `hypothesisReferences`, those hypotheses are the basis (unknown ones are flagged `HYPOTHESIS_NOT_FOUND`). Without them, every hypothesis whose evidence the claim cites is the basis. Other hypotheses that use the cited evidence are listed as `competingHypotheses`.
3. **Status**, derived only from the basis hypotheses' existing states and never stronger than they allow:

   | Situation | Status |
   |---|---|
   | No basis hypothesis | `UNKNOWN` |
   | Every basis hypothesis `CONTRADICTED` | `CONTRADICTED` |
   | Basis both supported and contradicted | `INSUFFICIENT` + `CONFLICTING_EVIDENCE` |
   | Any basis hypothesis untested / plausible / insufficient | `INSUFFICIENT` |
   | All supported, but a competing explanation over the same evidence is not contradicted | `INSUFFICIENT` + `COMPETING_EXPLANATION_OPEN` |
   | All supported, but causal wording without a causally supported and identified hypothesis | `INSUFFICIENT` + `CAUSAL_LANGUAGE_WITHOUT_IDENTIFICATION` |
   | All supported, but the claim cites none of their evidence | `INSUFFICIENT` |
   | Otherwise | `SUPPORTED` |

4. **Hints.** `OVERGENERALIZATION_LANGUAGE` for universal wording; `DEFINITION_OR_POPULATION_MISMATCH` when the cited datasets are incompatible (unit, population, period granularity or schema version, using the same check as dataset pre-analysis).
5. **Carried context.** Supporting / counter / neutral evidence (existing grounded evidence only), `unverifiedAssumptions` (passed through, never evaluated), `requiredEvidence` (the basis hypotheses' missing evidence), `researchGapIds`, and `cannotConclude` statements.

## Identity, idempotence and history

- `contentHash` covers statement, source, references and assumptions. Reusing an `id` for different content is rejected; the same content under another id is inspected identically and marked `duplicateOf`. Identical inputs produce byte-identical inspections.
- Public requests are idempotent through `idempotencyKey`, like every other write.
- `appendIteration` without `claims` keeps the run's claims and re-inspects them against the new analysis. Each inspection records `previous` (earlier iteration, its status, whether it changed). Earlier iterations keep their inspections.

## Verification

- Unit tests: `internal/service/claim_inspection_test.go` (status rules, flags, citations, duplicates, determinism, re-inspection).
- Use case: `internal/usecase/claim_inspection_test.go` (inspection on create and append, artifact and report export, no observation created).
- Conformance: `21-claim-inspection` (model-backed, scripted model).
- Golden cases: `testdata/golden/claims/cases/` — public statistics (definition change), technical diagnostics (invented source, unknown hypothesis) and marketing (over-generalization, Japanese causal wording, contradicted explanation), run by `internal/goldenset/claim_inspection_golden_test.go` under `make test-golden`. Human review stays `PENDING` until a person records it.

## Known limitations

- Keyword hints are small English and Japanese lists. Other wording ("explains", indirect causal phrasing, other languages) is not flagged, so an absent flag proves nothing.
- Linking without `hypothesisReferences` is by shared evidence, so a claim that cites broadly used evidence links to every hypothesis on it; that usually yields `INSUFFICIENT`, which is the honest result while the explanations compete.
- A claim's statement is never compared with the hypothesis text; whether the claim actually says what the hypothesis says is a human judgement (or a future model-assisted step with its own provenance).
- Inspections do not change decision readiness or promotion.
