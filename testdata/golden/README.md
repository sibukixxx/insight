# Golden Set — Shared Eval Contract fixtures (issue #14)

Versioned, Insight-owned domain fixtures for evaluating Public Evidence Reports and Research Runs. Shared schema semantics may later live in TechVit Business; the fixtures and the invariants they encode stay here.

## Layout

```text
testdata/golden/
  README.md                      this file
  schema/eval-case.v0.1.schema.json   shape of one case (JSON Schema, documentation + future validator)
  cases/                         one JSON file per golden case, versioned via "version"
  harness/golden_eval.py         stdlib-only invariant checker + CLI
  harness/test_golden_eval.py    unittest: fixtures stay consistent, every invariant fires when broken
```

`testdata/` is ignored by the Go toolchain. The domain/service regressions that exercise these semantics live under `internal/goldenset` with the `golden` build tag.

## Cases

| Index | Case | Verdict | What it guards |
|---:|---|---|---|
| 1 | `GS-01` synthetic association-only | `ASSOCIATION_ONLY` | two points, one region, post-hoc expectation; must stay at association |
| 2 | `GS-02` population / definition mismatch | `NOT_COMPARABLE` | naive −30% is a definition change; harmonized pair is `HARMONIZED_REFERENCE`, never `COMPARABLE` |
| 3 | `GS-03` real Open Data reproducibility | `NOT_COMPARABLE` | reproduces `reports/japan-company-count-2021-2024`; naive -30.8% is a population-definition change, harmonized pair stays `HARMONIZED_REFERENCE` |
| 4 | post-hoc expectation / HARKing guard | — | not yet added as a JSON case; covered at the Go domain level by `internal/goldenset/testdata/post_hoc_guard.json` |
| 5 | `GS-05` new-evidence re-analysis | `COMPETING_UNRESOLVED` | an independently pulled source log re-analyzes and contradicts the initial referral-program hypothesis; the stale conclusion must not stand |
| 6 | `GS-06` valid inconclusive | `INCONCLUSIVE` | inconclusive is correct; both "worked" and "did nothing" are forbidden |
| 7 | `GS-07` competing hypotheses / counter-evidence | `COMPETING_UNRESOLVED` | prior met but confounded; all three hypotheses stay open |

## Invariants checked by the harness

| Invariant | Semantic check from #14 |
|---|---|
| `SCHEMA` | required keys and vocabularies (mirrors `internal/domain` enums where they exist) |
| `EXPECTATION_PROVENANCE` | a `MODEL_PROPOSED` expectation is `POST_HOC`; `PRIOR` requires `declaredBeforeDataSeen` |
| `STAGE_SEPARATION` | data used to generate a hypothesis cannot be counted as `independent_validation` |
| `COMPARABILITY` | label is derived from `populationDefinitionId` + `harmonizationMethod`, same rule as `scripts/public_evidence_report.py` |
| `DETERMINISTIC_CALCULATION` | delta and percent change are recomputed from rows (Decimal, half-up to 0.1) |
| `CONFIDENCE_IS_NOT_CAUSAL` | non-causal verdicts cannot carry `IDENTIFIED` / `CAUSALLY_SUPPORTED` hypotheses |
| `COUNTER_EVIDENCE` | no search ⇒ no exhaustive-validation claim; `requiresCounterEvidence` ⇒ at least one `counter` item |
| `INCONCLUSIVE_IS_VALID` | inconclusive cases must list limitations and forbidden stronger narratives |
| `IDENTIFICATION_GAP_VISIBLE` | unresolved identification requires research gaps |
| `EXTERNAL_ARTIFACT_NOT_PRIMARY` | untraceable external / AI artifacts cannot be validation evidence |
| `HUMAN_REVIEW` | a review block exists; `PENDING` is a warning, an unknown status fails |

## Additional semantic regressions

`internal/goldenset/research_semantics_test.go` pins the implementation-level contracts introduced by the Research Loop:

- non-obvious Connection / candidate Mechanism never self-promotes causality;
- Generalization requires human review;
- same-pass post-hoc evidence cannot validate itself;
- EvidenceAddition resolves only the exact linked ResearchGap;
- Insight Delta records changed and unchanged interpretations without causal attribution;
- semantic AnalysisMode remains orthogonal to ResearchStage;
- a polished but unsupported artifact cannot pass the publication Promotion Gate.

## Run

```bash
make test-golden
# equivalent to:
go test -tags=golden ./internal/goldenset/...
python3 testdata/golden/harness/test_golden_eval.py          # regression tests
python3 testdata/golden/harness/golden_eval.py               # table of checks, exit 1 on FAIL
python3 testdata/golden/harness/golden_eval.py --json
python3 testdata/golden/harness/golden_eval.py --case GS-02 \
  --report reports/japan-company-count-2021-2024/generated/report.md   # forbidden-claim scan
```

## Remaining boundary

- The Python contract harness still does not score arbitrary generated prose beyond the forbidden-claim scan.
- Model quality itself is not certified by these deterministic tests; model-backed dogfooding remains a separate evaluation step.
- JSON Schema validation remains dependency-free and is implemented as the explicit subset checked by the harness.

## Editing rules

- Bump `version` when `expected` changes. Do not silently change expected numbers.
- Keep synthetic cases synthetic; real Open Data goes into `reports/` with provenance and is referenced from a case, not copied into it.
- A case that a "polished but unsupported" report would pass is a bug in the case.
- `humanReview` is filled by a person. The harness never sets it.
