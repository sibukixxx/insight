#!/usr/bin/env python3
"""Tests for the Discovery Benchmark v1 checker (issue #120).

The checked-in cases must pass, and every check must fire when a case is broken.
"""
from copy import deepcopy
import unittest

from discovery_benchmark import (
    CASES_DIR,
    RUBRIC_ITEMS,
    Check,
    check_all,
    check_case,
    load_baseline,
    load_cases,
    to_shared_eval_contract,
)


def failing(checks: list[Check]) -> set[str]:
    return {c.invariant for c in checks if c.status == "FAIL"}


def case_by_id(case_id: str) -> dict:
    return deepcopy(next(c for c in load_cases(CASES_DIR) if c["caseId"] == case_id))


class DiscoveryCasesAreConsistentTest(unittest.TestCase):
    def test_checked_in_cases_pass_every_check_against_the_baseline(self):
        self.assertEqual(failing(check_all(load_cases(CASES_DIR), load_baseline())), set())

    def test_checked_in_cases_cover_three_cases_per_category(self):
        ids = sorted(c["caseId"] for c in load_cases(CASES_DIR))
        self.assertEqual(ids, ["DB-F01", "DB-F02", "DB-F03", "DB-N01", "DB-N02", "DB-N03", "DB-P01", "DB-P02", "DB-P03"])

    def test_pending_human_review_maps_to_not_reviewed_never_a_pass(self):
        case = case_by_id("DB-P01")
        envelope = to_shared_eval_contract(case, check_case(case, load_baseline()))
        self.assertEqual(envelope["humanReview"]["result"], "NOT_REVIEWED")
        self.assertEqual(envelope["outcome"]["status"], "PARTIAL")
        self.assertEqual(envelope["expected"]["properties"], RUBRIC_ITEMS)


class DiscoveryCheckDetectionTest(unittest.TestCase):
    def setUp(self):
        self.baseline = load_baseline()

    def test_category_coverage_fails_when_a_category_has_fewer_than_three_cases(self):
        cases = [c for c in load_cases(CASES_DIR) if c["caseId"] != "DB-N03"]
        self.assertIn("CATEGORY_COVERAGE", failing(check_all(cases, self.baseline)))

    def test_schema_fails_when_forbidden_claims_are_missing(self):
        broken = case_by_id("DB-P01")
        broken["forbiddenClaims"] = []
        self.assertIn("SCHEMA", failing(check_case(broken, self.baseline)))

    def test_mode_input_fails_when_dataset_analysis_has_no_question(self):
        broken = case_by_id("DB-P02")
        del broken["researchQuestion"]
        self.assertIn("MODE_INPUT", failing(check_case(broken, self.baseline)))

    def test_mode_input_fails_when_research_review_has_no_claim(self):
        broken = case_by_id("DB-F01")
        del broken["claim"]
        self.assertIn("MODE_INPUT", failing(check_case(broken, self.baseline)))

    def test_category_expectation_fails_when_a_guard_case_expects_discovery(self):
        broken = case_by_id("DB-F01")
        broken["expected"]["discovery"] = True
        self.assertIn("CATEGORY_EXPECTATION", failing(check_case(broken, self.baseline)))

    def test_known_failure_fails_when_reason_is_empty(self):
        broken = case_by_id("DB-N02")
        broken["expected"]["knownFailure"]["reason"] = ""
        self.assertIn("KNOWN_FAILURE_DECLARED", failing(check_case(broken, self.baseline)))

    def test_human_rubric_fails_when_filled_without_a_human_review(self):
        broken = case_by_id("DB-P01")
        broken["humanRubric"]["novelty"] = "PASS"
        self.assertIn("HUMAN_RUBRIC", failing(check_case(broken, self.baseline)))

    def test_baseline_fails_when_case_version_changes_without_remeasuring(self):
        broken = case_by_id("DB-P01")
        broken["version"] = "0.2.0"
        self.assertIn("BASELINE", failing(check_case(broken, self.baseline)))

    def test_baseline_fails_when_known_failure_is_removed_but_baseline_still_fails(self):
        broken = case_by_id("DB-N02")
        del broken["expected"]["knownFailure"]
        self.assertIn("BASELINE", failing(check_case(broken, self.baseline)))

    def test_baseline_fails_when_no_measurement_exists(self):
        self.assertIn("BASELINE", failing(check_case(case_by_id("DB-P01"), None)))


if __name__ == "__main__":
    unittest.main()
