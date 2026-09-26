#!/usr/bin/env python3
"""Discovery Benchmark v1 case checker (issue #120).

What this is
------------
A stdlib-only checker for the discovery benchmark cases in ../discovery/cases and the
measured baseline in ../discovery/baseline.json. The measurements themselves come from
the Go golden test (internal/goldenset/discovery_benchmark_test.go), which runs the real
deterministic Core over each case. This checker keeps the case set honest:

- every category (positive discovery, valid no-discovery, false-association guard) has
  at least three cases;
- mode inputs are consistent (DATASET_ANALYSIS has a question, RESEARCH_REVIEW a claim);
- known failures are declared with a reason and show up in the baseline as such;
- the human rubric (novelty, grounding, ...) is never filled in by a tool: while human
  review is PENDING every rubric item stays PENDING and maps to NOT_REVIEWED.

Usage
-----
    python3 testdata/golden/harness/discovery_benchmark.py            # table, exit 1 on FAIL
    python3 testdata/golden/harness/discovery_benchmark.py --json
    python3 testdata/golden/harness/discovery_benchmark.py --case DB-F01 --report report.md
"""

from __future__ import annotations

import argparse
import json
import sys
from dataclasses import asdict
from pathlib import Path
from typing import Any

from golden_eval import Check, SHARED_EVAL_CONTRACT_VERSION, check_shared_eval_contract, load_cases, scan_report_for_forbidden_claims

HERE = Path(__file__).resolve().parent
DISCOVERY_DIR = HERE.parent / "discovery"
CASES_DIR = DISCOVERY_DIR / "cases"
BASELINE_PATH = DISCOVERY_DIR / "baseline.json"

CATEGORIES = {"POSITIVE_DISCOVERY", "VALID_NO_DISCOVERY", "FALSE_ASSOCIATION_GUARD"}
MIN_CASES_PER_CATEGORY = 3
MODES = {"DISCOVERY", "DATASET_ANALYSIS", "RESEARCH_REVIEW"}
RUBRIC_ITEMS = [
    "novelty",
    "grounding",
    "surpriseValidity",
    "hypothesisDiversity",
    "falsifiability",
    "missingEvidenceSpecificity",
    "reproducibility",
    "usefulnessForContinuedResearch",
]
RUBRIC_VALUES = {"PENDING", "PASS", "PARTIAL", "FAIL"}
HUMAN_REVIEW_STATUS = {"PENDING", "PASS", "FAIL"}
BASELINE_OUTCOMES = {"PASS", "KNOWN_FAILURE"}
REQUIRED_TOP = ["caseId", "version", "title", "category", "mode", "trap", "input", "expected", "forbiddenClaims", "humanRubric", "humanReview"]


def check_schema(case: dict[str, Any]) -> Check:
    cid = case.get("caseId", "?")
    problems = [f"missing top-level key {k!r}" for k in REQUIRED_TOP if k not in case]
    if case.get("category") not in CATEGORIES:
        problems.append(f"category {case.get('category')!r} not in {sorted(CATEGORIES)}")
    if case.get("mode") not in MODES:
        problems.append(f"mode {case.get('mode')!r} not in {sorted(MODES)}")
    inp = case.get("input", {})
    if not inp.get("series") and not inp.get("documents"):
        problems.append("input needs series or documents")
    expected = case.get("expected", {})
    if not isinstance(expected.get("discovery"), bool):
        problems.append("expected.discovery must be a boolean")
    if not expected.get("assertions"):
        problems.append("expected.assertions must not be empty")
    if not case.get("forbiddenClaims"):
        problems.append("forbiddenClaims must not be empty")
    return Check(cid, "SCHEMA", "FAIL" if problems else "PASS", "; ".join(problems) or "discovery case shape ok")


def check_mode_input(case: dict[str, Any]) -> Check:
    cid = case["caseId"]
    mode, question, claim = case["mode"], (case.get("researchQuestion") or "").strip(), case.get("claim")
    problems = []
    if mode == "DATASET_ANALYSIS" and not question:
        problems.append("DATASET_ANALYSIS needs a researchQuestion")
    if mode == "DISCOVERY" and question:
        problems.append("DISCOVERY is question-free; move the question to DATASET_ANALYSIS")
    if mode == "RESEARCH_REVIEW" and not (isinstance(claim, dict) and claim.get("id") and claim.get("statement")):
        problems.append("RESEARCH_REVIEW needs a claim with id and statement")
    if mode != "RESEARCH_REVIEW" and claim:
        problems.append("only RESEARCH_REVIEW cases carry an external claim")
    return Check(cid, "MODE_INPUT", "FAIL" if problems else "PASS", "; ".join(problems) or f"{mode} input consistent")


def check_category_expectation(case: dict[str, Any]) -> Check:
    cid = case["caseId"]
    want = case["category"] == "POSITIVE_DISCOVERY"
    if case["expected"]["discovery"] != want:
        return Check(cid, "CATEGORY_EXPECTATION", "FAIL", f"{case['category']} must expect discovery={want}")
    return Check(cid, "CATEGORY_EXPECTATION", "PASS", f"discovery={want}")


def check_known_failure(case: dict[str, Any]) -> Check:
    cid = case["caseId"]
    known = case["expected"].get("knownFailure")
    if known is None:
        return Check(cid, "KNOWN_FAILURE_DECLARED", "PASS", "no known failure")
    problems = []
    if not (known.get("reason") or "").strip():
        problems.append("known failure needs a reason")
    n = len(case["expected"]["assertions"])
    indices = known.get("assertions") or []
    if not indices:
        problems.append("known failure must name the failing assertions")
    problems += [f"assertion index {i} out of range" for i in indices if not (isinstance(i, int) and 0 <= i < n)]
    return Check(cid, "KNOWN_FAILURE_DECLARED", "FAIL" if problems else "WARN", "; ".join(problems) or f"known failure: {known['reason']}")


def check_human_rubric(case: dict[str, Any]) -> Check:
    cid = case["caseId"]
    rubric, review = case.get("humanRubric") or {}, case.get("humanReview") or {}
    problems = []
    missing = [k for k in RUBRIC_ITEMS if k not in rubric]
    if missing:
        problems.append(f"rubric items missing: {missing}")
    problems += [f"rubric {k}={v!r} invalid" for k, v in rubric.items() if v not in RUBRIC_VALUES]
    status = review.get("status")
    if status not in HUMAN_REVIEW_STATUS:
        problems.append(f"humanReview.status {status!r} invalid")
    if status == "PENDING":
        filled = [k for k, v in rubric.items() if v != "PENDING"]
        if filled:
            problems.append(f"rubric filled without a human review: {filled}")
    elif status in {"PASS", "FAIL"} and not review.get("reviewer"):
        problems.append("a recorded human review needs a reviewer")
    if problems:
        return Check(cid, "HUMAN_RUBRIC", "FAIL", "; ".join(problems))
    if status == "PENDING":
        return Check(cid, "HUMAN_RUBRIC", "WARN", "human rubric not yet recorded")
    return Check(cid, "HUMAN_RUBRIC", "PASS", f"human review {status} by {review['reviewer']}")


def check_baseline(case: dict[str, Any], baseline: dict[str, Any] | None) -> Check:
    cid = case["caseId"]
    if baseline is None:
        return Check(cid, "BASELINE", "FAIL", f"no baseline at {BASELINE_PATH}")
    measured = {m.get("caseId"): m for m in baseline.get("cases", [])}.get(cid)
    if measured is None:
        return Check(cid, "BASELINE", "FAIL", "case has no baseline measurement; run the Go golden test with GOLDEN_UPDATE=1")
    problems = []
    if measured.get("version") != case["version"]:
        problems.append(f"baseline measured version {measured.get('version')} but case is {case['version']}")
    if not measured.get("inputFingerprint") or not measured.get("executionFingerprint"):
        problems.append("baseline must keep input and execution fingerprints separate")
    outcome = measured.get("outcome")
    if outcome not in BASELINE_OUTCOMES:
        problems.append(f"baseline outcome {outcome!r}")
    declared = case["expected"].get("knownFailure") is not None
    if declared != (outcome == "KNOWN_FAILURE"):
        problems.append(f"known failure declared={declared} but baseline outcome is {outcome}")
    return Check(cid, "BASELINE", "FAIL" if problems else "PASS", "; ".join(problems) or f"baseline {outcome}")


def check_category_coverage(cases: list[dict[str, Any]]) -> Check:
    counts = {c: 0 for c in CATEGORIES}
    for case in cases:
        if case.get("category") in counts:
            counts[case["category"]] += 1
    short = {k: v for k, v in counts.items() if v < MIN_CASES_PER_CATEGORY}
    detail = ", ".join(f"{k}={counts[k]}" for k in sorted(counts))
    if short:
        return Check("*", "CATEGORY_COVERAGE", "FAIL", f"need >= {MIN_CASES_PER_CATEGORY} per category: {detail}")
    return Check("*", "CATEGORY_COVERAGE", "PASS", detail)


def to_shared_eval_contract(case: dict[str, Any], checks: list[Check]) -> dict[str, Any]:
    """Map a discovery case to Shared Eval Contract v1. Pending review is NOT_REVIEWED."""
    failures = [c for c in checks if c.status == "FAIL"]
    warnings = [c for c in checks if c.status == "WARN"]
    review = case.get("humanReview") or {}
    return {
        "schemaVersion": SHARED_EVAL_CONTRACT_VERSION,
        "caseId": case.get("caseId", ""),
        "domain": "insight",
        "input": {
            "references": [f"discovery-case:{case.get('caseId', '')}"],
            "contextReferences": [f"mode:{case.get('mode', '')}", f"category:{case.get('category', '')}"],
        },
        "expected": {
            "properties": list(RUBRIC_ITEMS),
            "requiredEvidence": [],
            "prohibitedClaims": list(case.get("forbiddenClaims", [])),
            "schemaChecks": ["SCHEMA", "SHARED_EVAL_CONTRACT"],
            "qualityChecks": [c.invariant for c in checks],
        },
        "humanReview": {
            "required": True,
            "reviewer": review.get("reviewer") or "",
            "result": {"PASS": "PASS", "FAIL": "FAIL"}.get(review.get("status"), "NOT_REVIEWED"),
            "notes": review.get("notes") or "",
        },
        "outcome": {
            "status": "FAIL" if failures else ("PARTIAL" if warnings else "PASS"),
            "reasons": [c.detail for c in failures + warnings],
        },
    }


def check_case(case: dict[str, Any], baseline: dict[str, Any] | None) -> list[Check]:
    checks = [check_schema(case)]
    if checks[0].status == "FAIL":
        return checks
    checks += [
        check_mode_input(case),
        check_category_expectation(case),
        check_known_failure(case),
        check_human_rubric(case),
        check_baseline(case, baseline),
    ]
    checks.append(check_shared_eval_contract(to_shared_eval_contract(case, checks), case["caseId"]))
    return checks


def load_baseline(path: Path = BASELINE_PATH) -> dict[str, Any] | None:
    if not path.exists():
        return None
    with path.open("r", encoding="utf-8") as handle:
        return json.load(handle)


def check_all(cases: list[dict[str, Any]], baseline: dict[str, Any] | None) -> list[Check]:
    checks = [check_category_coverage(cases)]
    ids = [c.get("caseId") for c in cases]
    if len(ids) != len(set(ids)):
        checks.append(Check("*", "UNIQUE_CASE_IDS", "FAIL", f"duplicate case ids in {ids}"))
    for case in cases:
        checks += check_case(case, baseline)
    return checks


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Check Discovery Benchmark cases and baseline")
    parser.add_argument("--cases", type=Path, default=CASES_DIR)
    parser.add_argument("--baseline", type=Path, default=BASELINE_PATH)
    parser.add_argument("--case", help="restrict to one caseId")
    parser.add_argument("--report", type=Path, help="report markdown to scan for forbidden claims (requires --case)")
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args(argv)

    cases, baseline = load_cases(args.cases), load_baseline(args.baseline)
    if args.case:
        cases = [c for c in cases if c.get("caseId") == args.case]
        if not cases:
            print(f"no case {args.case!r} in {args.cases}", file=sys.stderr)
            return 2
        checks = []
        for case in cases:
            checks += check_case(case, baseline)
    else:
        checks = check_all(cases, baseline)
    if args.report:
        text = args.report.read_text(encoding="utf-8")
        for case in cases:
            hits = scan_report_for_forbidden_claims({"expected": {"forbiddenClaims": case["forbiddenClaims"]}}, text)
            checks.append(Check(case["caseId"], "REPORT_FORBIDDEN_CLAIMS", "FAIL" if hits else "PASS", "; ".join(hits) or "no forbidden phrase in report"))

    if args.json:
        print(json.dumps([asdict(c) for c in checks], ensure_ascii=False, indent=2))
    else:
        for c in checks:
            print(f"{c.case_id:7} {c.status:4} {c.invariant:24} {c.detail}")
    return 1 if any(c.status == "FAIL" for c in checks) else 0


if __name__ == "__main__":
    sys.exit(main())
