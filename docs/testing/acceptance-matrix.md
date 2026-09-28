# Acceptance matrix (#156)

One page that says which user-visible behavior is proven, which test proves it, how to re-run it, and what is **not** verified yet. Rows describe observable boundaries (HTTP, the real binary, pinned files), not Go functions. Unit, property, golden and conformance tests stay as they are; this page only points at them.

- **proved** — a runnable test asserts the outcome today (last result in the [report](#report-2026-09-28)).
- **proved (E2E)** — asserted only by the browser suite against a real binary; re-run with `make web-e2e`.
- **pending** — no test asserts it yet; the reason is given.

Routes and payload shapes are not listed here: the OpenAPI documents in `docs/openapi/public-engine-v1.json` and `docs/openapi/reference-api.json` (#157) own the route inventory. Test names below are `file:TestName` (Go) or `file › "title"` (Playwright).

## Scopes

| Scope | What is guaranteed | Where it is proven |
| --- | --- | --- |
| **Public Engine v1** (`/api/public/v1`, `contracts/public-engine/v1`) | The versioned, SDK-facing contract. Breaking it needs a new contract version ([public-engine-contract.md](../public-engine-contract.md)). | `internal/http/public_conformance_test.go:TestPublicContractConformance` (21 shared fixtures over real HTTP, deterministic and scripted-model engines), `internal/publicengine/drift_test.go` |
| **Reference Web / BFF** (`/api/*` outside `/public/v1`, the embedded UI) | Not a public contract. May change with the UI; tested so the shipped product behaves as the README says. | `internal/http/*_integration_test.go`, `internal/http/acceptance_boundary_test.go`, `web/src/test/*`, `web/e2e/*` |
| **Demo-only** (`/api/demo*`, `internal/sampledata` with `-tags demo`) | Exists only in `make build-demo`. The delivery binary must contain none of it. | `internal/sampledata/scenarios_{demo,delivery}_test.go`, `internal/http/demo_scenario_*_test.go`, `web/e2e/samples-ja.spec.ts` |

`insight-scripted-llm` / `internal/llm/scripted` is a deterministic stand-in used for transport and contract checks. A passing scripted run proves wiring, never real model quality.

## Journeys

### (a) Japanese Documents CSV sample → preview/import → analysis → Evidence and result

Fixture: `internal/sampledata/scenarios/ja-shop-records/input.csv` (synthetic, 9 rows, `id,source,title,content`).

| Capability | Boundary | Expected outcome | Test | Run | State |
| --- | --- | --- | --- | --- | --- |
| Sample gallery lists 3 packs with data kind | demo HTTP, Home | 3 scenarios in fixed order, kinds synthetic/official/mixed, `importKind=documents` | `internal/sampledata/scenarios_demo_test.go:TestScenariosListsThreePacksInDisplayOrderWhenDemoBuild`; `web/e2e/samples-ja.spec.ts › "サンプル選択 → CSV 取り込み → 分析 → 根拠 → レポート"` | `go test -tags demo ./internal/sampledata/`; `make web-e2e` | proved |
| Served CSV is the exact bundled file; the ordinary importer accepts every row; the sample project is reused | demo HTTP | byte-identical CSV, `imported == rows`, `skipped == 0`, same project id twice | `internal/http/demo_scenario_integration_test.go:TestDemoScenariosHTTPOpensIdempotentEmptyProjectAndServesImportableCSVWhenDemoBuild` | `go test -tags demo ./internal/http/` | proved |
| Preview stores nothing; bad header / unknown kind rejected | Reference HTTP | 400, no documents stored | `internal/http/import_preview_integration_test.go:TestImportPreviewHTTPReportsRowsWithoutStoringDocumentsWhenUploaded`, `…:TestImportPreviewHTTPRejectsUnknownKindAndBadHeaderWhenUploaded` | `go test ./internal/http/` | proved |
| Arbitrary numeric CSV is not silently accepted as a Documents CSV | Reference HTTP | `examples/demo-ja/03-numeric-raw-aggregation.csv` → 400 on preview and import for both `documents` and `analysis`; nothing stored | `internal/http/acceptance_boundary_test.go:TestImportHTTPRefusesNumericRawAggregationCSVForEveryKindAndStoresNothing` (new) | `go test ./internal/http/` | proved |
| Model-backed analysis completes; findings link to Evidence whose quote is the imported Japanese text; report downloads | real demo binary + scripted model | run completes, evidence `<mark>` contains `架空データ`, report contains it | `web/e2e/samples-ja.spec.ts › "サンプル選択 → CSV 取り込み → 分析 → 根拠 → レポート"` | `make web-e2e` | proved (E2E) |
| Quotes are verbatim source spans and stay so across output locales | Public Engine HTTP, scripted model | quote == `source[start:end]`, never translated | `internal/http/output_locale_fidelity_test.go:TestOutputLocaleChangesGeneratedLanguageButNotEvidenceOrResearchState` | `go test ./internal/http/` | proved (English evidence only) |
| Same journey at Go/HTTP level with Japanese input (quotes verbatim, ResearchGap present) | Reference HTTP + scripted model | — | none | — | pending: only the browser suite covers the Japanese path end to end |
| Provenance and limitations visible after a run, phone width | real demo binary | synthetic limitation text shown, no horizontal scroll | `web/e2e/samples-ja.spec.ts › "分析済みのサンプルへ戻り、スマートフォン幅でも横スクロールしない @mobile"` | `make web-e2e` | proved (E2E) |

### (b) Pinned official population sample and its provenance

Fixture: `internal/sampledata/scenarios/ja-official-population/` (8 rows, 国勢調査 総人口 2015/2020, 水戸市・常陸太田市・つくば市), raw snapshots in `examples/demo-ja/official/raw/`.

| Capability | Boundary | Expected outcome | Test | Run | State |
| --- | --- | --- | --- | --- | --- |
| Generated CSV/manifest are reproducible offline from raw snapshots | pinned files | `ok:` for every output, exit 0 | `node examples/demo-ja/official/build.mjs --check` | same | proved locally; **not in CI** (pending) |
| Every official value traces to a raw file whose sha256 matches the manifest; synthetic rows say `架空` | pinned files | hashes equal, required provenance fields present | `internal/sampledata/scenarios_demo_test.go:TestScenarioSourcesKeepOfficialAndSyntheticRowsDistinguishableWhenLoaded` | `go test -tags demo ./internal/sampledata/` | proved |
| Bundled input matches `inputSha256` and the manifest row count | pinned files | `ScenarioInput` returns the CSV (it checks the hash), rows == manifest `rows`, Documents header | `internal/sampledata/scenarios_demo_test.go:TestScenarioInputMatchesManifestRowsAndDocumentsHeaderWhenRead` | `go test -tags demo ./internal/sampledata/` | proved |
| A hash mismatch is refused instead of served | runtime | error, not a silent serve | none (only the matching path is exercised) | — | pending |
| The provenance the UI shows over HTTP equals the files on disk | demo HTTP | `rawSha256` == sha256(raw file), served CSV sha256 == `inputSha256` == on-disk file; retrievedAt/license/attribution/unit/periods present; limitations non-empty | `internal/http/demo_scenario_provenance_test.go:TestDemoScenariosHTTPReportsProvenanceMatchingPinnedFilesAndKeepsMixedPeriodsSeparate` (new) | `go test -tags demo ./internal/http/` | proved |
| Downloaded CSV is what the importer receives | real demo binary | header `id,source,title,content`, contains `総人口は241,656人` | `web/e2e/samples-ja.spec.ts › "サンプル選択 → …"` | `make web-e2e` | proved (E2E) |
| **Refusal:** without a model this sample is not analyzed and no success is faked | Reference HTTP, no model | run `failed`, `failureCode=ERROR`, message names Settings; no metrics, observations, patterns or insights | `internal/http/acceptance_boundary_test.go:TestAnalysisHTTPFailsWithGuidanceAndStoresNoFindingsWhenNoModelIsConfigured` (new) | `go test ./internal/http/` | proved |
| Delivery binary contains no sample markers | linked delivery binary | none of the markers present | `internal/sampledata/scenarios_delivery_test.go:TestDeliveryBinaryContainsNoSampleDataWhenBuiltWithoutDemoTag`; delivery HTTP refuses demo routes: `internal/http/demo_scenario_integration_test.go:TestDemoScenariosHTTPIsEmptyAndRefusesProjectsWhenDeliveryBuild` | `go test ./internal/sampledata/ ./internal/http/` | proved |
| Model-backed analysis of the official sample (Evidence quotes, no invented values) | real binary | — | none (the E2E journey uses sample 01) | — | pending |

### (c) Mixed-source sample keeps incompatible periods apart

Fixture: `internal/sampledata/scenarios/ja-population-establishments/` (12 rows: 国勢調査 2020 + 経済センサス 事業所数（民営）2016/2021 + 3 synthetic memos).

| Capability | Boundary | Expected outcome | Test | Run | State |
| --- | --- | --- | --- | --- | --- |
| Official and synthetic parts are both present and distinguishable | pinned files | ≥1 official and ≥1 synthetic source; memo rows say `架空` | `internal/sampledata/scenarios_demo_test.go:TestScenarioSourcesKeepOfficialAndSyntheticRowsDistinguishableWhenLoaded` | `go test -tags demo ./internal/sampledata/` | proved |
| Each survey keeps its own period on the wire; the time-point limitation is stated | demo HTTP | periods `国勢調査=2020`, `事業所数（民営）（～2016年）=2016`, `事業所数（民営）=2021` kept separate; 3 synthetic rows; limitation `同一時点の比率として組み合わせられない` present | `internal/http/demo_scenario_provenance_test.go:TestDemoScenariosHTTPReportsProvenanceMatchingPinnedFilesAndKeepsMixedPeriodsSeparate` (new) | `go test -tags demo ./internal/http/` | proved |
| Deterministic comparisons never cross units, populations or duplicate periods | service rule | no comparison across incompatible series | `internal/service/dataset_preanalysis_test.go:TestComputeDatasetComparisonsDoesNotCompareAcrossIncompatibleUnitsOrPopulations`, `…:TestComputeDatasetComparisonsSkipsSeriesWithDuplicatePeriods`; `internal/analytical/temporal_test.go:TestTemporalIncompatibilitiesSuppressArithmetic` | `go test ./internal/service/ ./internal/analytical/` | proved (rule level) |
| No forced join / causal claim when the mixed sample is analyzed | analysis output | — | none | — | pending. The sample rows carry no `record_count` metadata, so the deterministic pre-analysis does not touch them at all ("no join" is trivially true there). What a model says about them needs a real-model evaluation, which this slice does not run. |

## Boundary checks

| Check | Expected outcome | Test | State |
| --- | --- | --- | --- |
| No fake success when no model is configured | failure with guidance, nothing stored | new `acceptance_boundary_test.go:TestAnalysisHTTPFailsWithGuidanceAndStoresNoFindingsWhenNoModelIsConfigured`; `internal/service/pipeline_test.go:TestPipelineRunWithoutLLMFailsWhenNothingCanBeAnalyzedDeterministically`; deterministic mode records no model provenance and no insights: `…:TestPipelineRunWithoutLLMCompletesDeterministicPreAnalysis`; UI blocks the start: `web/e2e/delivery.spec.ts › "without a model and without datasets text evidence is blocked before starting"`, server failure stays visible: `› "a real server-side failure stays visible and retry preserves run settings"`; fixture `03-deterministic-analytical-artifact` (`modelBacked=false`, never hypotheses) | proved |
| Unknown/unavailable is not zero | missing ≠ 0, undefined rate from 0, unrecorded ≠ empty | `internal/analytical/temporal_test.go:TestMissingIsNotZero`; `internal/service/dataset_preanalysis_test.go:TestComputeDatasetComparisonsLeavesRateUndefinedWhenStartingFromZero`; `internal/http/run_snapshot_integration_test.go:TestLegacyRunsExposeNoSnapshotsInsteadOfEmptyOnes`; `internal/usecase/run_scope_test.go:TestProjectReportMarksMissingProvenanceAsNotRecorded`; `internal/publicengine/engine_state_test.go:TestEngineInfoOmitsEngineStateWhenUnknown`; `web/e2e/ux.spec.ts › "UNKNOWN settings remain distinct and defaults follow selected values"` | proved |
| Provenance attached to imported evidence | manifest or at least file hash on every document/aggregate | `internal/http/document_import_integration_test.go:TestImportDocumentsCSVHTTPWithManifestAttachesProvenanceToDocuments`, `…WithoutManifestStillRecordsFileHash`, `…RejectsInvalidManifestBeforeImporting`, `…:TestImportAnalysisCSVHTTPWithManifestAttachesProvenanceToAggregates` | proved |
| Evidence links and counter-evidence | findings cite observations; counter-evidence returned next to support | fixtures `01`, `05-conflicting-counter-evidence`; `output_locale_fidelity_test.go` (verbatim offsets) | proved |
| Duplicate input is not a fresh Observation (#120) | exact re-import counted once; a differing value for the same period stays a conflict | `internal/service/dataset_preanalysis_test.go:TestRunDatasetPreAnalysisCountsAnExactReimportOnceWhenCopied`, `…:TestRunDatasetPreAnalysisKeepsConflictingDuplicatePeriodWhenValuesDiffer`; golden `testdata/golden/discovery/cases/DB-F03.json`; ingest replay: `internal/service/ingest_test.go:TestIngestSameBytesAndIdentityReturnTheSameReceiptWithoutDuplicateEvidence`; fixture `07-idempotent-requests` | proved (dataset documents). Pending: re-importing the same **Documents CSV** through the Reference import route is not asserted at the HTTP boundary. |
| Unchanged result on a deterministic fixture | same prepared input → same results | fixture `08-execution-profile-equivalence` (LIGHT/STANDARD/HEAVY equal — compares profiles, not repeated runs); fixture `11-re-evaluation` (unchanged input appends no iteration); `internal/analytical/artifact_test.go:TestReproducibilityKeyIgnoresDatasetOrder`; Discovery Benchmark baseline fingerprints (`make test-golden`) | proved for those forms; a repeated identical Reference-API run compared field by field is pending |
| `outputLocale` changes only generated text (engine) | quotes, findings and research state identical for ja-JP and en-US; omitted locale keeps legacy fingerprints | `output_locale_fidelity_test.go:TestOutputLocaleChangesGeneratedLanguageButNotEvidenceOrResearchState`, fixture `20-output-locale`, `internal/service/output_locale_test.go` | proved |
| UI language is independent of `outputLocale` (browser) | switching the UI language does not set `outputLocale`, and vice versa | partial only: `web/src/test/app.test.tsx › "changes the language in place and keeps the route and typed values"` (UI switch keeps route and typed text, does not look at `outputLocale`); `web/e2e/ux.spec.ts › "UNKNOWN settings remain distinct …"` selects `en-US` output under the `ja` UI project | pending: no test asserts the independence directly; UI unit tests for the output-language start control are **pending merge** on `feat/131-output-locale-primary` (`app.test.tsx`: "offers the output language in the main start control …" and two more) |
| No secrets in responses | API key, key query, base URL never echoed | `internal/http/run_snapshot_integration_test.go:TestCreateAnalysisRecordsLabelNoteAndExecutionSnapshotWithoutSecrets` | proved |
| Intentional insufficient / refused outcomes | at least one asserted | new no-model test above; fixture `06-contract-version-mismatch`; fixture `04-missing-evidence-re-evaluation` (missing evidence → ResearchGap); golden `TestGoldenPolishedButUnsupportedCannotReachPublicationReady` | proved |

## Public contract inventory (issue item 4) and adversarial probes (item 5)

All fixtures run through `TestPublicContractConformance` against a real router, SQLite and JobManager (`go test ./internal/http/ -run TestPublicContractConformance`). Wire types and error codes are checked against the schema by `internal/publicengine/drift_test.go:TestWireTypesMatchTheContractSchema` and `…:TestErrorCodesMatchTheContractSchema`.

| Fixture | Covers | Engine |
| --- | --- | --- |
| 01 | generic public-data subject end to end | scripted |
| 02 | commerce-like consumer, opaque refs only, no consumer action | scripted |
| 03 | deterministic Analytical Artifact → grounded observations, never hypotheses | deterministic |
| 04 | missing evidence → ResearchGap/DataRequirement; append-only re-evaluation | scripted |
| 05 | counter-evidence returned next to support | scripted |
| 06 | **rejects** unsupported `contractVersion` (400 `UNSUPPORTED_CONTRACT_VERSION`), invalid subject (400 `INVALID_REQUEST`), unknown resources (404) | deterministic |
| 07 | idempotency replay; **rejects** key reuse with different content | deterministic |
| 08 | LIGHT/STANDARD/HEAVY give the same results | deterministic |
| 09 | raw artifact verified, not trusted by claim | deterministic |
| 10 | run comparison; differences never called causal | scripted |
| 11 | re-evaluation append-only; stale iteration **rejected** | scripted |
| 12 | longitudinal as-of windows; retroactive as-of **rejected** | scripted |
| 13 | scenarios; horizon required; no invented probability | scripted |
| 14–15 | data triage and re-triage from ResearchGaps; model triage **refused** without a model | deterministic / scripted |
| 16 | temporal operation pack; unknown operation **rejected** | deterministic |
| 17 | model bindings; unknown stage / disallowed model **rejected** | scripted |
| 18 | reasoning profiles | scripted |
| 19 | engine-state identity | deterministic |
| 20 | output locale; quotes verbatim | scripted |
| 21 | claim inspection; fabricated citations never become evidence | scripted |

Other rejection cases outside the fixtures: `internal/publicengine/generic_research_test.go:TestPublicAnalysisRejectsUnknownReasoningProfile`, `internal/publicengine/output_locale_test.go:TestStartAnalysisRejectsUnsupportedOutputLocale`, `internal/publicengine/claims_test.go:TestPublicClaimsValidationRejectsMalformedClaims`, Reference: `internal/http/run_snapshot_integration_test.go:TestCreateAnalysisRejectsAnUnknownSemanticAnalysisMode` (no run created), `internal/http/run_scope_integration_test.go:TestRunScopedEndpointsRejectAnotherProjectsRun`.

Pending: bounded, seeded fuzz/corpus probes of the CSV/JSON parsers and authorization-related routes (optional per the issue, out of scope for this slice). No security certification is implied by anything on this page.

## Quality entry and changed-area mapping (issue item 6)

**Mandatory before every PR** (not selectable, not replaced by any LLM choice): `make test && make vet` (normal + demo tags, includes contract conformance and drift), and for any change touching `web/` or the embedded UI `make web-check`. The short real-binary smoke is `make web-e2e`. In CI it runs only on pull requests that touch `web/**`, `internal/web/**`, `internal/http/**` or `Makefile` (`.github/workflows/frontend-checks.yml`); `make test && make vet` run only when `**.go`, `go.mod/go.sum`, `Makefile`, `contracts/**` or migrations change (`.github/workflows/temporal-evidence-checks.yml`). Run them locally when your change falls outside those filters. A CI billing lock or a skipped job is not evidence that a test passed.

The mapping below restates [AGENTS.md → Change-dependent checks](../../AGENTS.md); AGENTS.md wins if they ever differ. It is a lookup table, not a selector: when a path matches several rows, run all of them.

| Changed path | Also run |
| --- | --- |
| `internal/domain/`, `internal/service/`, `internal/usecase/`, `internal/http/` | `make test && make vet` |
| insight scoring / evaluation (e.g. `internal/service/confidence.go`, `internal/goldenset/`, `testdata/golden/`) | `make test-golden` |
| `internal/sampledata/`, `examples/demo-ja/`, demo handlers | `make test` (both tags) + `node examples/demo-ja/official/build.mjs --check` when official data or `build.mjs` changes |
| `web/` | `make web-check`, commit rebuilt `internal/web/dist`; `make web-e2e` for flow changes |
| `contracts/public-engine/v1/`, `contracts/analytical-artifact/v1/` | `make test`, update `docs/public-engine-contract.md`, resync SDK snapshots |
| docs only | none beyond link sanity |

Known false negatives: Go changes outside `internal/http/` can break browser flows, but CI runs `make web-e2e` only for the paths above; data-only edits under `internal/sampledata/scenarios/` or `examples/demo-ja/` (CSV/JSON, no `.go`) trigger neither CI workflow; `build.mjs --check` is not part of `make test` or CI.

## Report (2026-09-28)

Branch `test/156-acceptance-matrix` on top of `origin/main` 6d78133. No paid model was called.

| Command | Result |
| --- | --- |
| `make test` (normal + `-tags demo`) | pass (all packages `ok`) |
| `make vet` (normal + `-tags demo`) | pass |
| `node examples/demo-ja/official/build.mjs --check` | pass (4 × `ok:`, exit 0) |
| new tests: `go test ./internal/http/ -run 'TestImportHTTPRefusesNumeric\|TestAnalysisHTTPFailsWithGuidance'` and `go test -tags demo ./internal/http/ -run 'TestDemoScenariosHTTPReportsProvenance'` | 3 pass |
| `make web-e2e` | not re-run in this slice (ports shared with other work); last known 25/25 on 2026-09-28 per PR #153 and a local run — **maintainer to re-run** |
| `make web-check`, `make test-golden` | not run (no `web/` or golden changes in this slice) |

Fixture hashes (from the committed manifests, verified by the tests above):

| File | sha256 |
| --- | --- |
| `ja-shop-records/input.csv` | `49979dc0a73fd8d29680a0ed1f4c808c3a8ec1f2040c67afd6e657781ed57e09` |
| `ja-official-population/input.csv` | `fea2fca87c3e83ef011d7fbea9839ed595431720391f2d3f02ed95d0d94693b2` |
| `ja-population-establishments/input.csv` | `3e6f0e9483b8af6e03ac1e36fd76976a278870fc7099fd8c606698cea0e1f0c4` |
| `raw/population_census_2015_2020.json` | `163f1816517f73f6d1641b43e79ac1663371ef1ab6c0227d4b5bcff1ff7e55a5` |
| `raw/establishments_private_2016.json` | `40a4004776d1d1d81d2a5c6a892aa4a2d5a9ecbaf8362f1ed17e782f2201f8f0` |
| `raw/establishments_private_2021.json` | `b2a0b3382d830f959d2c7fcd087170df20a37c8ca825d74cf15d6985d94fe41e` |

Observed gaps (functional ownership stays with #131/#151; UX wording with #141):

1. `build.mjs --check` passes locally but runs in no CI workflow, and data-only sample edits trigger no CI job (see the false negatives above).
2. Journeys (b) and (c) have no model-backed analysis test; only sample 01 goes through analysis in the browser suite.
3. "No forced join/causality" for the mixed sample is only guaranteed at the rule level; the sample itself is never parsed deterministically, and model output is unevaluated.
4. The Japanese journey is proven end to end only by `make web-e2e`; there is no Go-level equivalent.
5. Re-importing the same Documents CSV via the Reference route is not asserted.
6. Repeated identical runs are not compared field by field at the Reference API.
7. UI-language vs `outputLocale` independence is not asserted directly in the browser/UI tests; the start-control tests wait for `feat/131-output-locale-primary`.
8. A scenario whose bundled CSV does not match `inputSha256` is refused by code, but no test exercises that branch.
9. Fuzz/corpus probes: not started.
