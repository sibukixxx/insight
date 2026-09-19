# Public Evidence Report — Japan company count (P0 + P1)

Research Question: **日本の会社は本当に減っているのか？**

This is the first reproducible public-evidence report fixture for Insight Lab. It deliberately uses a very small normalized dataset so every public claim can be traced to an official source and independently recomputed.

- **P0** (2026-09-18): four published aggregates; result intentionally inconclusive because the 2021 and 2024 populations differ.
- **P1** (2026-09-19, issue #12): adds the Statistics Bureau's 2024 reference table that includes individual proprietorships with no employees, recomputes the delta on the harmonized population, and adds a deterministic comparability label to every claim.

## Pipeline

```text
Official Statistics Bureau publications (結果の概要 PDF, e-Stat 参考表 Excel)
  ↓ manual source verification / normalization — see ACQUISITION.md
normalized.csv   (population_definition_id + harmonization_method per row)
  ↓ deterministic Python calculation + comparability labelling
report.md
  ├─ evidence-ledger.json   (claim → source rows → calculation → comparability → counter evidence)
  ├─ note-draft.md
  └─ sns-summary.md
```

The published counts are third-party official statistics. TechVit's primary analysis is the deterministic difference/rate/decomposition calculation, the comparability check, competing-explanation framing, and the evidence ledger. It must not be described as TechVit measuring the original counts.

## Comparability labels

Labels are derived from row metadata, never written by hand in prose. `validate_ledger` rejects a ledger entry whose label is stronger than what its underlying comparisons allow.

| Label | Meaning |
|---|---|
| `COMPARABLE` | same `population_definition_id`, both rows `observed` |
| `HARMONIZED_REFERENCE` | same `population_definition_id`, but at least one row is a reference value (e.g. `carry_forward_2021_no_employee_individual`) |
| `NOT_COMPARABLE` | different `population_definition_id` |

No claim in this report is `COMPARABLE`. The 2021→2024 harmonized pair is `HARMONIZED_REFERENCE` because the 2024 reference table carries the 2021 no-employee individual values forward.

## Result in one line

The naive top line (−30.8%) is `NOT_COMPARABLE`. On the harmonized population the change is −332,030 (−9.0%), of which nothing comes from the carried-forward segment by construction. The company-enterprise slice is +1.1% in one 2024 table and −2.6% in another. The report therefore still does not conclude that Japanese companies are declining; it narrows what the public numbers can and cannot say.

## Reproduce

From the Insight Lab repository root:

```bash
python3 scripts/public_evidence_report.py \
  --input reports/japan-company-count-2021-2024/normalized.csv \
  --out-dir reports/japan-company-count-2021-2024/generated

python3 scripts/test_public_evidence_report.py
```

The generator uses only the Python standard library. It does not require an LLM API key. The test suite also re-materializes the P0-only dataset (four rows, original columns) and checks that the P0 report is still produced from it.

## Files

| File | Purpose |
|---|---|
| `normalized.csv` | source-of-truth rows with provenance, population definition and harmonization method |
| `ACQUISITION.md` | where the P1 reference values came from: URLs, statInfId, sheet/row, checksums, footnotes |
| `AUDIT.md` | P0 current-reality audit against the codebase |
| `generated/` | outputs regenerated from `normalized.csv`; do not hand-edit |

## Authentication boundary

This report does **not** require `HOUJIN_APP_ID`; it uses Statistics Bureau aggregate publications only. The existing `ja-company-base` integration remains the next registry-data path:

```text
HOUJIN_APP_ID → ja-company-export → AnalysisRecord CSV → Insight Lab dataset import
```

Do not commit the AppID. Also do not interpret `ASSIGNED` as incorporation/startup or `CLOSED` as bankruptcy without an explicit validated mapping. Fetching e-Stat or registry data is an adapter/acquisition responsibility outside Insight Lab's core (see `docs/byo-evidence-boundary.md`).

## Why the result is still inconclusive

The apparent 2021→2024 drop in the top-line enterprise-equivalent count fails the population-comparability check. Harmonizing the population shrinks the drop to about a third, but the harmonization itself substitutes 2021 values for a segment the 2024 survey did not measure, and the company-count direction depends on which official table is cited. The correct result is therefore not a trend claim; it is a reproducible demonstration that definition mismatch can invalidate a simple delta, and that "harmonized" is not the same as "observed".
