# Public Evidence Report P0 — Japan company count

Research Question: **日本の会社は本当に減っているのか？**

This is the first reproducible public-evidence report fixture for Insight Lab. It deliberately uses a very small normalized dataset so every public claim can be traced to an official source and independently recomputed.

## Pipeline

```text
Official Statistics Bureau publications
  ↓ manual source verification / normalization (P0)
normalized.csv
  ↓ deterministic Python calculation
report.md
  ├─ evidence-ledger.json
  ├─ note-draft.md
  └─ sns-summary.md
```

The published counts are third-party official statistics. TechVit's primary analysis is the deterministic difference/rate calculation, comparability check, competing-explanation framing, and evidence ledger. It must not be described as TechVit measuring the original counts.

## Reproduce

From the Insight Lab repository root:

```bash
python3 scripts/public_evidence_report.py \
  --input reports/japan-company-count-2021-2024/normalized.csv \
  --out-dir reports/japan-company-count-2021-2024/generated

python3 scripts/test_public_evidence_report.py
```

The generator uses only the Python standard library. It does not require an LLM API key.

## Authentication boundary

This first report does **not** require `HOUJIN_APP_ID`; it uses Statistics Bureau aggregate publications. The existing `ja-company-base` integration remains the next registry-data path:

```text
HOUJIN_APP_ID → ja-company-export → AnalysisRecord CSV → Insight Lab dataset import
```

Do not commit the AppID. Also do not interpret `ASSIGNED` as incorporation/startup or `CLOSED` as bankruptcy without an explicit validated mapping.

## Why the first result is intentionally inconclusive

The apparent 2021→2024 drop in the top-line enterprise-equivalent count fails a population-comparability check. The 2024 survey excludes individual proprietorship establishments with no employees, while a company-enterprise slice points in the opposite direction. The correct P0 result is therefore not a dramatic trend claim; it is a reproducible demonstration that definition mismatch can invalidate a simple delta.
