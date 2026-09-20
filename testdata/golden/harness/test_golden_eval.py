#!/usr/bin/env python3
"""Tests for the Golden Set harness skeleton (issue #14).

The harness checks each golden case for internal consistency against the shared eval
contract invariants. Cases live in ../cases; this file also acts as the regression test
for the checked-in cases.
"""
from copy import deepcopy
from pathlib import Path
import json
import unittest

from golden_eval import (
    CASES_DIR,
    Check,
    check_case,
    load_case,
    load_cases,
    scan_report_for_forbidden_claims,
    to_shared_eval_contract,
    check_shared_eval_contract,
)


def failing(checks: list[Check]) -> set[str]:
    return {c.invariant for c in checks if c.status == "FAIL"}


class GoldenCasesAreConsistentTest(unittest.TestCase):
    def test_checked_in_cases_load_and_have_unique_ids(self):
        cases = load_cases(CASES_DIR)
        ids = [c["caseId"] for c in cases]
        self.assertEqual(sorted(ids), ["GS-01", "GS-02", "GS-03", "GS-05", "GS-06", "GS-07"])
        self.assertEqual(len(ids), len(set(ids)))

    def test_every_checked_in_case_passes_all_invariants(self):
        for case in load_cases(CASES_DIR):
            with self.subTest(case=case["caseId"]):
                self.assertEqual(failing(check_case(case)), set())

    def test_every_checked_in_case_records_a_human_review_block(self):
        for case in load_cases(CASES_DIR):
            with self.subTest(case=case["caseId"]):
                self.assertIn(case["humanReview"]["status"], {"PENDING", "PASS", "FAIL"})

    def test_every_checked_in_case_maps_to_shared_eval_contract_v1(self):
        for case in load_cases(CASES_DIR):
            with self.subTest(case=case["caseId"]):
                checks = check_case(case)
                envelope = to_shared_eval_contract(case, checks)
                self.assertEqual(envelope["schemaVersion"], "1")
                self.assertEqual(envelope["caseId"], case["caseId"])
                self.assertEqual(envelope["domain"], "insight")
                self.assertEqual(check_shared_eval_contract(envelope, case["caseId"]).status, "PASS")

    def test_pending_human_review_maps_to_not_reviewed_not_fake_pass(self):
        case = load_case(CASES_DIR / "01-synthetic-association-only.json")
        envelope = to_shared_eval_contract(case, check_case(case))
        self.assertEqual(envelope["humanReview"]["result"], "NOT_REVIEWED")
        self.assertEqual(envelope["outcome"]["status"], "PARTIAL")


class InvariantDetectionTest(unittest.TestCase):
    """Each invariant must fire when a case is deliberately broken."""

    def setUp(self):
        self.case01 = load_case(CASES_DIR / "01-synthetic-association-only.json")
        self.case02 = load_case(CASES_DIR / "02-population-definition-mismatch.json")
        self.case06 = load_case(CASES_DIR / "06-valid-inconclusive.json")
        self.case07 = load_case(CASES_DIR / "07-competing-hypothesis-counter-evidence.json")

    def test_schema_violation_is_reported_when_required_key_is_missing(self):
        broken = deepcopy(self.case01)
        del broken["expected"]["verdict"]
        self.assertIn("SCHEMA", failing(check_case(broken)))

    def test_post_hoc_model_proposed_expectation_cannot_be_relabeled_prior(self):
        broken = deepcopy(self.case01)
        expectation = broken["expected"]["expectations"][0]
        expectation["basis"] = "MODEL_PROPOSED"
        expectation["provenance"] = "PRIOR"
        self.assertIn("EXPECTATION_PROVENANCE", failing(check_case(broken)))

    def test_prior_expectation_must_be_declared_before_data_was_seen(self):
        broken = deepcopy(self.case07)
        expectation = broken["expected"]["expectations"][0]
        expectation["provenance"] = "PRIOR"
        expectation["declaredBeforeDataSeen"] = False
        self.assertIn("EXPECTATION_PROVENANCE", failing(check_case(broken)))

    def test_generation_dataset_cannot_silently_count_as_independent_validation(self):
        broken = deepcopy(self.case01)
        evidence = broken["expected"]["evidence"][0]
        evidence["datasetId"] = "ds-discovery"
        evidence["usage"] = "independent_validation"
        self.assertIn("STAGE_SEPARATION", failing(check_case(broken)))

    def test_comparison_label_is_recomputed_from_population_metadata(self):
        broken = deepcopy(self.case02)
        broken["expected"]["comparisons"][0]["comparabilityLabel"] = "COMPARABLE"
        self.assertIn("COMPARABILITY", failing(check_case(broken)))

    def test_deterministic_delta_regression_is_detected(self):
        broken = deepcopy(self.case02)
        broken["expected"]["comparisons"][0]["expectedDelta"] += 1
        self.assertIn("DETERMINISTIC_CALCULATION", failing(check_case(broken)))

    def test_association_only_verdict_forbids_identified_causal_status(self):
        broken = deepcopy(self.case01)
        broken["expected"]["hypotheses"][0]["identificationStatus"] = "IDENTIFIED"
        self.assertIn("CONFIDENCE_IS_NOT_CAUSAL", failing(check_case(broken)))

    def test_no_counter_evidence_search_forbids_exhaustive_validation_claim(self):
        broken = deepcopy(self.case06)
        broken["expected"]["counterEvidenceSearched"] = False
        broken["expected"]["mayClaimExhaustiveValidation"] = True
        self.assertIn("COUNTER_EVIDENCE", failing(check_case(broken)))

    def test_required_counter_evidence_must_be_present(self):
        broken = deepcopy(self.case07)
        broken["expected"]["evidence"] = [e for e in broken["expected"]["evidence"] if e["type"] != "counter"]
        self.assertIn("COUNTER_EVIDENCE", failing(check_case(broken)))

    def test_inconclusive_verdict_requires_limitations_and_research_gaps(self):
        broken = deepcopy(self.case06)
        broken["expected"]["requiredLimitations"] = []
        self.assertIn("INCONCLUSIVE_IS_VALID", failing(check_case(broken)))

    def test_unresolved_identification_gap_must_stay_visible(self):
        broken = deepcopy(self.case07)
        broken["expected"]["requiredResearchGaps"] = []
        self.assertIn("IDENTIFICATION_GAP_VISIBLE", failing(check_case(broken)))

    def test_untraceable_ai_output_cannot_be_primary_or_validation_evidence(self):
        broken = deepcopy(self.case07)
        broken["expected"]["evidence"].append(
            {
                "id": "EV-AI",
                "type": "support",
                "hypothesisIds": ["H1"],
                "datasetId": "ds-external-summary",
                "usage": "independent_validation",
                "sourceKind": "ai_output",
                "traceableToPrimary": False,
                "summary": "A chat assistant said the policy worked.",
            }
        )
        broken["input"]["datasets"].append(
            {"id": "ds-external-summary", "role": "context", "sourceKind": "ai_output", "rows": []}
        )
        self.assertIn("EXTERNAL_ARTIFACT_NOT_PRIMARY", failing(check_case(broken)))

    def test_human_review_block_with_unknown_status_fails(self):
        broken = deepcopy(self.case01)
        broken["humanReview"]["status"] = "LGTM"
        self.assertIn("HUMAN_REVIEW", failing(check_case(broken)))


class ReportScanTest(unittest.TestCase):
    def test_forbidden_claim_phrase_in_report_is_reported(self):
        case = load_case(CASES_DIR / "01-synthetic-association-only.json")
        report = "売上の増加はキャンペーンが原因であることが証明された。"
        hits = scan_report_for_forbidden_claims(case, report)
        self.assertTrue(hits)

    def test_clean_report_has_no_forbidden_claim_hits(self):
        case = load_case(CASES_DIR / "01-synthetic-association-only.json")
        report = "売上の増加とキャンペーン実施時期には関連が観測されたが、因果は識別できない。"
        self.assertEqual(scan_report_for_forbidden_claims(case, report), [])


if __name__ == "__main__":
    unittest.main()
