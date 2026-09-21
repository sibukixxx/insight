#!/usr/bin/env python3
"""Golden Set harness skeleton for Insight Lab public evidence reports (issue #14).

What this is
------------
A deterministic, stdlib-only checker that validates each golden case against the
semantic invariants of the Shared Eval Contract. It runs on the case JSON alone, so it
can be used (a) as a regression test for the fixtures and (b) as the contract that a
report-under-test must satisfy. The only report-facing hook implemented today is the
forbidden-claim scan; scoring a generated report/ledger against `expected` is the next
step and is intentionally not faked here.

What this is not
----------------
It does not call an LLM, does not fetch anything, and does not compute statistics
beyond the difference / percent-change arithmetic that the report generator also uses.

Usage
-----
    python3 testdata/golden/harness/golden_eval.py                 # check all cases
    python3 testdata/golden/harness/golden_eval.py --json          # machine-readable
    python3 testdata/golden/harness/golden_eval.py --case GS-02 --report path/to/report.md
"""

from __future__ import annotations

import argparse
import json
import sys
from dataclasses import asdict, dataclass
from decimal import Decimal, ROUND_HALF_UP
from pathlib import Path
from typing import Any, Iterable

HERE = Path(__file__).resolve().parent
CASES_DIR = HERE.parent / "cases"
CONTRACT_VERSION = "0.1"
SHARED_EVAL_CONTRACT_VERSION = "1"

# --- vocab -------------------------------------------------------------------------
# Mirrors internal/domain where a concept already exists there.
STAGES = {"EXPLORATORY", "VALIDATION"}
VERDICTS = {"ASSOCIATION_ONLY", "NOT_COMPARABLE", "INCONCLUSIVE", "COMPETING_UNRESOLVED", "SUPPORTED_WITH_LIMITS"}
EXPECTATION_BASIS = {"SOURCE_BACKED", "MODEL_PROPOSED", "UNKNOWN"}
EXPECTATION_PROVENANCE = {"PRIOR", "POST_HOC"}
HYPOTHESIS_ROLES = {"PRIMARY", "COMPETING"}
CAUSAL_STATUS = {"OBSERVED_ASSOCIATION", "CAUSAL_HYPOTHESIS", "CAUSALLY_SUPPORTED", "NOT_IDENTIFIED"}
VALIDATION_STATUS = {"UNTESTED", "PLAUSIBLE", "PARTIALLY_SUPPORTED", "SUPPORTED", "INSUFFICIENT_EVIDENCE", "CONTRADICTED"}
IDENTIFICATION_STATUS = {"IDENTIFIED", "NOT_IDENTIFIED", "UNKNOWN"}
EVIDENCE_TYPES = {"support", "counter", "neutral"}
EVIDENCE_USAGE = {"generation", "re_analysis", "independent_validation", "context"}
SOURCE_KINDS = {"primary", "external_artifact", "ai_output"}
DATASET_ROLES = {"hypothesis_generation", "validation", "context"}
COMPARABILITY = {"COMPARABLE", "HARMONIZED_REFERENCE", "NOT_COMPARABLE"}
GAP_CATEGORIES = {"CONFOUNDER", "COMPARISON_CONTROL", "PRE_PERIOD", "TIMING", "MEASUREMENT", "EXTERNAL_CONTEXT", "SOURCE_QUALITY", "OTHER"}
HUMAN_REVIEW_STATUS = {"PENDING", "PASS", "FAIL"}
SHARED_HUMAN_REVIEW_STATUS = {"PASS", "FAIL", "PARTIAL", "NOT_REVIEWED"}
SHARED_OUTCOME_STATUS = {"PASS", "FAIL", "PARTIAL"}
NON_CAUSAL_VERDICTS = {"ASSOCIATION_ONLY", "NOT_COMPARABLE", "INCONCLUSIVE", "COMPETING_UNRESOLVED"}

REQUIRED_TOP = ["caseId", "version", "title", "kind", "goldenSetIndex", "dimensions", "input", "expected", "humanReview"]
REQUIRED_INPUT = ["researchQuestion", "stage", "datasets"]
REQUIRED_EXPECTED = [
    "verdict", "expectations", "hypotheses", "evidence", "comparisons",
    "counterEvidenceSearched", "mayClaimExhaustiveValidation", "requiresCounterEvidence",
    "requiredLimitations", "requiredResearchGaps", "forbiddenClaims",
]


@dataclass(frozen=True)
class Check:
    case_id: str
    invariant: str
    status: str  # PASS | FAIL | WARN
    detail: str


# --- loading -----------------------------------------------------------------------

def load_case(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as handle:
        return json.load(handle)


def load_cases(directory: Path) -> list[dict[str, Any]]:
    return [load_case(p) for p in sorted(directory.glob("*.json"))]


# --- helpers -----------------------------------------------------------------------

def _dataset_index(case: dict[str, Any]) -> dict[str, dict[str, Any]]:
    return {d["id"]: d for d in case.get("input", {}).get("datasets", [])}


def _row_value(dataset: dict[str, Any], key: str) -> Decimal:
    for row in dataset.get("rows", []):
        if row.get("key") == key:
            return Decimal(str(row["value"]))
    raise KeyError(f"dataset {dataset.get('id')!r} has no row {key!r}")


def label_comparability(baseline: dict[str, Any], current: dict[str, Any]) -> str:
    """Same rule as scripts/public_evidence_report.py: derived from metadata, never prose."""
    if baseline.get("populationDefinitionId") != current.get("populationDefinitionId"):
        return "NOT_COMPARABLE"
    if baseline.get("harmonizationMethod", "observed") != "observed" or current.get("harmonizationMethod", "observed") != "observed":
        return "HARMONIZED_REFERENCE"
    return "COMPARABLE"


def pct_change(baseline: Decimal, current: Decimal) -> Decimal:
    return ((current - baseline) / baseline * Decimal(100)).quantize(Decimal("0.1"), rounding=ROUND_HALF_UP)


# --- invariants --------------------------------------------------------------------

def check_schema(case: dict[str, Any]) -> list[Check]:
    cid = case.get("caseId", "?")
    problems: list[str] = []
    for key in REQUIRED_TOP:
        if key not in case:
            problems.append(f"missing top-level key {key!r}")
    inp = case.get("input", {})
    for key in REQUIRED_INPUT:
        if key not in inp:
            problems.append(f"missing input.{key}")
    exp = case.get("expected", {})
    for key in REQUIRED_EXPECTED:
        if key not in exp:
            problems.append(f"missing expected.{key}")
    if inp.get("stage") not in STAGES:
        problems.append(f"input.stage {inp.get('stage')!r} not in {sorted(STAGES)}")
    if exp.get("verdict") not in VERDICTS:
        problems.append(f"expected.verdict {exp.get('verdict')!r} not in {sorted(VERDICTS)}")
    for d in inp.get("datasets", []):
        if d.get("role") not in DATASET_ROLES:
            problems.append(f"dataset {d.get('id')!r} role {d.get('role')!r} invalid")
        if d.get("sourceKind") not in SOURCE_KINDS:
            problems.append(f"dataset {d.get('id')!r} sourceKind {d.get('sourceKind')!r} invalid")
    for e in exp.get("expectations", []):
        if e.get("basis") not in EXPECTATION_BASIS or e.get("provenance") not in EXPECTATION_PROVENANCE:
            problems.append(f"expectation {e.get('id')!r} has invalid basis/provenance")
    for h in exp.get("hypotheses", []):
        if h.get("role") not in HYPOTHESIS_ROLES or h.get("causalStatus") not in CAUSAL_STATUS \
                or h.get("validationStatus") not in VALIDATION_STATUS or h.get("identificationStatus") not in IDENTIFICATION_STATUS:
            problems.append(f"hypothesis {h.get('id')!r} has invalid status vocabulary")
    for ev in exp.get("evidence", []):
        if ev.get("type") not in EVIDENCE_TYPES or ev.get("usage") not in EVIDENCE_USAGE or ev.get("sourceKind") not in SOURCE_KINDS:
            problems.append(f"evidence {ev.get('id')!r} has invalid type/usage/sourceKind")
    for c in exp.get("comparisons", []):
        if c.get("comparabilityLabel") not in COMPARABILITY:
            problems.append(f"comparison {c.get('id')!r} label invalid")
    for g in exp.get("requiredResearchGaps", []):
        if g.get("category") not in GAP_CATEGORIES:
            problems.append(f"research gap category {g.get('category')!r} invalid")
    if problems:
        return [Check(cid, "SCHEMA", "FAIL", "; ".join(problems))]
    return [Check(cid, "SCHEMA", "PASS", f"contract v{CONTRACT_VERSION} shape ok")]


def check_expectation_provenance(case: dict[str, Any]) -> list[Check]:
    cid = case["caseId"]
    problems = []
    for e in case["expected"].get("expectations", []):
        if e.get("basis") == "MODEL_PROPOSED" and e.get("provenance") != "POST_HOC":
            problems.append(f"{e.get('id')}: MODEL_PROPOSED expectation relabeled {e.get('provenance')}")
        if e.get("provenance") == "PRIOR" and not e.get("declaredBeforeDataSeen", False):
            problems.append(f"{e.get('id')}: PRIOR without declaredBeforeDataSeen")
    status = "FAIL" if problems else "PASS"
    return [Check(cid, "EXPECTATION_PROVENANCE", status, "; ".join(problems) or "post-hoc expectations stay POST_HOC")]


def check_stage_separation(case: dict[str, Any]) -> list[Check]:
    cid = case["caseId"]
    datasets = _dataset_index(case)
    problems = []
    for ev in case["expected"].get("evidence", []):
        ds = datasets.get(ev.get("datasetId"))
        if ds is None:
            problems.append(f"{ev.get('id')}: unknown dataset {ev.get('datasetId')!r}")
            continue
        if ev.get("usage") == "independent_validation" and ds.get("role") == "hypothesis_generation":
            problems.append(f"{ev.get('id')}: generation dataset {ds['id']!r} counted as independent validation")
    status = "FAIL" if problems else "PASS"
    return [Check(cid, "STAGE_SEPARATION", status, "; ".join(problems) or "generation data is not reused as validation")]


def check_comparisons(case: dict[str, Any]) -> list[Check]:
    cid = case["caseId"]
    datasets = _dataset_index(case)
    label_problems, calc_problems = [], []
    for c in case["expected"].get("comparisons", []):
        try:
            base_ds, cur_ds = datasets[c["baselineDatasetId"]], datasets[c["currentDatasetId"]]
            base, cur = _row_value(base_ds, c["baselineRowKey"]), _row_value(cur_ds, c["currentRowKey"])
        except KeyError as exc:
            calc_problems.append(f"{c.get('id')}: {exc}")
            continue
        derived = label_comparability(base_ds, cur_ds)
        if c.get("comparabilityLabel") != derived:
            label_problems.append(f"{c['id']}: labeled {c.get('comparabilityLabel')} but metadata derives {derived}")
        delta, pct = cur - base, pct_change(base, cur)
        if Decimal(str(c.get("expectedDelta"))) != delta or Decimal(str(c.get("expectedPctChange"))) != pct:
            calc_problems.append(f"{c['id']}: expected {c.get('expectedDelta')}/{c.get('expectedPctChange')}% but recomputed {delta}/{pct}%")
    return [
        Check(cid, "COMPARABILITY", "FAIL" if label_problems else "PASS", "; ".join(label_problems) or "labels derived from population metadata"),
        Check(cid, "DETERMINISTIC_CALCULATION", "FAIL" if calc_problems else "PASS", "; ".join(calc_problems) or "delta / pct recomputed"),
    ]


def check_confidence_is_not_causal(case: dict[str, Any]) -> list[Check]:
    cid = case["caseId"]
    exp = case["expected"]
    problems = []
    for h in exp.get("hypotheses", []):
        if exp.get("verdict") in NON_CAUSAL_VERDICTS and (h.get("identificationStatus") == "IDENTIFIED" or h.get("causalStatus") == "CAUSALLY_SUPPORTED"):
            problems.append(f"{h.get('id')}: {h.get('causalStatus')}/{h.get('identificationStatus')} under verdict {exp.get('verdict')}")
        if h.get("causalStatus") == "OBSERVED_ASSOCIATION" and h.get("identificationStatus") == "IDENTIFIED":
            problems.append(f"{h.get('id')}: association labeled IDENTIFIED")
    status = "FAIL" if problems else "PASS"
    return [Check(cid, "CONFIDENCE_IS_NOT_CAUSAL", status, "; ".join(problems) or "no hypothesis promoted to identified cause")]


def check_counter_evidence(case: dict[str, Any]) -> list[Check]:
    cid = case["caseId"]
    exp = case["expected"]
    problems = []
    if not exp.get("counterEvidenceSearched") and exp.get("mayClaimExhaustiveValidation"):
        problems.append("no counter-evidence search but exhaustive validation may be claimed")
    counters = [ev for ev in exp.get("evidence", []) if ev.get("type") == "counter"]
    if exp.get("requiresCounterEvidence") and not counters:
        problems.append("requiresCounterEvidence but no counter evidence listed")
    status = "FAIL" if problems else "PASS"
    return [Check(cid, "COUNTER_EVIDENCE", status, "; ".join(problems) or f"{len(counters)} counter evidence item(s)")]


def check_inconclusive_is_valid(case: dict[str, Any]) -> list[Check]:
    cid = case["caseId"]
    exp = case["expected"]
    if exp.get("verdict") != "INCONCLUSIVE":
        return [Check(cid, "INCONCLUSIVE_IS_VALID", "PASS", "not an inconclusive case")]
    problems = []
    if not exp.get("requiredLimitations"):
        problems.append("inconclusive verdict without required limitations")
    if not exp.get("forbiddenClaims"):
        problems.append("inconclusive verdict without forbidden stronger-narrative claims")
    status = "FAIL" if problems else "PASS"
    return [Check(cid, "INCONCLUSIVE_IS_VALID", status, "; ".join(problems) or "inconclusive kept as a valid, bounded result")]


def check_identification_gap_visible(case: dict[str, Any]) -> list[Check]:
    cid = case["caseId"]
    exp = case["expected"]
    unresolved = [h["id"] for h in exp.get("hypotheses", []) if h.get("identificationStatus") in {"NOT_IDENTIFIED", "UNKNOWN"}]
    if unresolved and not exp.get("requiredResearchGaps"):
        return [Check(cid, "IDENTIFICATION_GAP_VISIBLE", "FAIL", f"unresolved identification for {unresolved} but no research gaps required")]
    return [Check(cid, "IDENTIFICATION_GAP_VISIBLE", "PASS", f"{len(exp.get('requiredResearchGaps', []))} gap(s) required for {len(unresolved)} unresolved hypothesis(es)")]


def check_external_artifact_not_primary(case: dict[str, Any]) -> list[Check]:
    cid = case["caseId"]
    datasets = _dataset_index(case)
    problems = []
    for ev in case["expected"].get("evidence", []):
        external = ev.get("sourceKind") in {"external_artifact", "ai_output"}
        if not external:
            continue
        ds = datasets.get(ev.get("datasetId"), {})
        if not ev.get("traceableToPrimary", False):
            if ev.get("usage") == "independent_validation":
                problems.append(f"{ev.get('id')}: untraceable {ev.get('sourceKind')} used as independent validation")
            if ds.get("role") == "validation":
                problems.append(f"{ev.get('id')}: untraceable {ev.get('sourceKind')} in a validation dataset")
    status = "FAIL" if problems else "PASS"
    return [Check(cid, "EXTERNAL_ARTIFACT_NOT_PRIMARY", status, "; ".join(problems) or "external / AI artifacts not promoted to primary evidence")]


def check_human_review(case: dict[str, Any]) -> list[Check]:
    cid = case["caseId"]
    review = case.get("humanReview")
    if not isinstance(review, dict) or review.get("status") not in HUMAN_REVIEW_STATUS:
        return [Check(cid, "HUMAN_REVIEW", "FAIL", f"humanReview.status must be one of {sorted(HUMAN_REVIEW_STATUS)}")]
    if review["status"] == "PENDING":
        return [Check(cid, "HUMAN_REVIEW", "WARN", "human review not yet recorded")]
    return [Check(cid, "HUMAN_REVIEW", "PASS", f"human review {review['status']} by {review.get('reviewer') or 'unknown'}")]


INVARIANTS = [
    check_expectation_provenance,
    check_stage_separation,
    check_comparisons,
    check_confidence_is_not_causal,
    check_counter_evidence,
    check_inconclusive_is_valid,
    check_identification_gap_visible,
    check_external_artifact_not_primary,
    check_human_review,
]


def _shared_human_review(case: dict[str, Any]) -> dict[str, Any]:
    review = case.get("humanReview") or {}
    status = review.get("status")
    mapped = {
        "PENDING": "NOT_REVIEWED",
        "PASS": "PASS",
        "FAIL": "FAIL",
    }.get(status, "NOT_REVIEWED")
    return {
        "required": True,
        "reviewer": review.get("reviewer") or "",
        "result": mapped,
        "notes": review.get("notes") or "",
    }


def to_shared_eval_contract(case: dict[str, Any], checks: list[Check] | None = None) -> dict[str, Any]:
    """Map an Insight-owned golden case into Shared Eval Contract v1.

    This is an exchange-format adapter only; Insight keeps its domain fixtures and
    semantic checks local and does not take a runtime dependency on downstream applications.
    """
    if checks is None:
        checks = []
    failures = [c for c in checks if c.status == "FAIL"]
    warnings = [c for c in checks if c.status == "WARN"]
    outcome = "FAIL" if failures else ("PARTIAL" if warnings else "PASS")
    expected = case.get("expected", {})
    evidence = expected.get("evidence", [])
    return {
        "schemaVersion": SHARED_EVAL_CONTRACT_VERSION,
        "caseId": case.get("caseId", ""),
        "domain": "insight",
        "input": {
            "references": [
                d.get("id", "")
                for d in case.get("input", {}).get("datasets", [])
                if d.get("id")
            ],
            "contextReferences": [
                f"stage:{case.get('input', {}).get('stage', '')}",
                f"kind:{case.get('kind', '')}",
            ],
        },
        "expected": {
            "properties": list(case.get("dimensions", [])),
            "requiredEvidence": [e.get("id", "") for e in evidence if e.get("id")],
            "prohibitedClaims": list(expected.get("forbiddenClaims", [])),
            "schemaChecks": ["SCHEMA", "SHARED_EVAL_CONTRACT"],
            "qualityChecks": [c.invariant for c in checks if c.invariant != "SHARED_EVAL_CONTRACT"],
        },
        "humanReview": _shared_human_review(case),
        "outcome": {
            "status": outcome,
            "reasons": [c.detail for c in failures + warnings],
        },
    }


def check_shared_eval_contract(envelope: dict[str, Any], case_id: str) -> Check:
    problems: list[str] = []
    if envelope.get("schemaVersion") != SHARED_EVAL_CONTRACT_VERSION:
        problems.append("schemaVersion must be 1")
    if not envelope.get("caseId"):
        problems.append("caseId is required")
    inp = envelope.get("input")
    if not isinstance(inp, dict) or not isinstance(inp.get("references"), list):
        problems.append("input.references must be an array")
    expected = envelope.get("expected")
    if not isinstance(expected, dict):
        problems.append("expected must be an object")
    else:
        for key in ("properties", "requiredEvidence", "prohibitedClaims", "schemaChecks", "qualityChecks"):
            if key in expected and not isinstance(expected[key], list):
                problems.append(f"expected.{key} must be an array")
    review = envelope.get("humanReview")
    if not isinstance(review, dict) or review.get("result") not in SHARED_HUMAN_REVIEW_STATUS:
        problems.append("humanReview.result is invalid")
    outcome = envelope.get("outcome")
    if not isinstance(outcome, dict) or outcome.get("status") not in SHARED_OUTCOME_STATUS:
        problems.append("outcome.status is invalid")
    return Check(
        case_id,
        "SHARED_EVAL_CONTRACT",
        "FAIL" if problems else "PASS",
        "; ".join(problems) or "maps to Shared Eval Contract v1",
    )


def check_case(case: dict[str, Any]) -> list[Check]:
    checks = check_schema(case)
    if checks[0].status == "FAIL":
        return checks
    for invariant in INVARIANTS:
        checks.extend(invariant(case))
    envelope = to_shared_eval_contract(case, checks)
    checks.append(check_shared_eval_contract(envelope, case["caseId"]))
    return checks


# --- report-facing hook ------------------------------------------------------------

def scan_report_for_forbidden_claims(case: dict[str, Any], report_text: str) -> list[str]:
    """Return forbidden phrases from the case that appear verbatim in a report."""
    return [phrase for phrase in case["expected"].get("forbiddenClaims", []) if phrase in report_text]


# --- CLI ---------------------------------------------------------------------------

def _print_table(checks: Iterable[Check]) -> None:
    for c in checks:
        print(f"{c.case_id:6} {c.status:4} {c.invariant:30} {c.detail}")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Check Insight Lab golden cases against the eval contract invariants")
    parser.add_argument("--cases", type=Path, default=CASES_DIR, help="directory of case JSON files")
    parser.add_argument("--case", help="restrict to one caseId")
    parser.add_argument("--report", type=Path, help="report markdown to scan for forbidden claims (requires --case)")
    parser.add_argument("--json", action="store_true", help="emit JSON instead of a table")
    args = parser.parse_args(argv)

    cases = load_cases(args.cases)
    if args.case:
        cases = [c for c in cases if c.get("caseId") == args.case]
        if not cases:
            print(f"no case {args.case!r} in {args.cases}", file=sys.stderr)
            return 2

    checks: list[Check] = []
    for case in cases:
        checks.extend(check_case(case))
        if args.report:
            hits = scan_report_for_forbidden_claims(case, args.report.read_text(encoding="utf-8"))
            status = "FAIL" if hits else "PASS"
            checks.append(Check(case["caseId"], "REPORT_FORBIDDEN_CLAIMS", status, "; ".join(hits) or "no forbidden phrase in report"))

    if args.json:
        print(json.dumps([asdict(c) for c in checks], ensure_ascii=False, indent=2))
    else:
        _print_table(checks)
    return 1 if any(c.status == "FAIL" for c in checks) else 0


if __name__ == "__main__":
    sys.exit(main())
