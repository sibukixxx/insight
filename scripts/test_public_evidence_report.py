#!/usr/bin/env python3
from decimal import Decimal
from pathlib import Path
import csv
import json
import tempfile
import unittest

from public_evidence_report import (
    COMPARABLE,
    HARMONIZED_REFERENCE,
    NOT_COMPARABLE,
    Record,
    build_ledger,
    compare,
    compare_metrics,
    label_comparability,
    load_records,
    select_record,
    validate_ledger,
    write_outputs,
)

ROOT = Path(__file__).resolve().parents[1]
DATA = ROOT / "reports" / "japan-company-count-2021-2024" / "normalized.csv"

P0_METRIC_IDS = {"enterprise_equivalents", "company_enterprises"}
P0_COLUMNS = [
    "metric_id", "period", "geographic_scope", "measure", "value", "unit", "population_scope",
    "source_name", "source_url", "publisher", "retrieved_at", "coverage_period",
    "license_or_terms", "known_limitations",
]


def make_record(metric_id: str, period: str, value: int, definition_id: str, method: str = "observed") -> Record:
    return Record(
        metric_id=metric_id,
        period=period,
        geographic_scope="Japan",
        measure="test",
        value=value,
        unit="units",
        population_scope=f"scope {definition_id}",
        population_definition_id=definition_id,
        harmonization_method=method,
        source_name="test",
        source_url="https://example.invalid/test",
        publisher="test",
        retrieved_at="2026-09-19",
        coverage_period=f"{period}-06-01",
        license_or_terms="test",
        known_limitations="test",
    )


def write_p0_only_csv(path: Path) -> None:
    """Re-materialize the original P0 dataset: four rows, original 14 columns only."""
    with DATA.open("r", encoding="utf-8", newline="") as handle:
        rows = [row for row in csv.DictReader(handle) if row["metric_id"] in P0_METRIC_IDS]
    with path.open("w", encoding="utf-8", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=P0_COLUMNS, extrasaction="ignore")
        writer.writeheader()
        writer.writerows(rows)


class PublicEvidenceReportP0Test(unittest.TestCase):
    """The P0 claims must stay reproducible after the P1 harmonization."""

    @classmethod
    def setUpClass(cls):
        cls.records = load_records(DATA)

    def test_enterprise_naive_delta_is_deterministic(self):
        result = compare(self.records, "enterprise_equivalents", "2021", "2024")
        self.assertEqual(result.delta, -1_134_222)
        self.assertEqual(result.pct_change, Decimal("-30.8"))
        self.assertFalse(result.comparable_population)

    def test_company_slice_points_in_opposite_direction(self):
        result = compare(self.records, "company_enterprises", "2021", "2024")
        self.assertEqual(result.delta, 18_514)
        self.assertEqual(result.pct_change, Decimal("1.1"))

    def test_naive_top_line_comparison_is_labeled_not_comparable(self):
        result = compare(self.records, "enterprise_equivalents", "2021", "2024")
        self.assertEqual(result.comparability, NOT_COMPARABLE)

    def test_p0_only_dataset_still_reproduces_p0_report(self):
        with tempfile.TemporaryDirectory() as tmp:
            data = Path(tmp) / "p0.csv"
            write_p0_only_csv(data)
            out = Path(tmp) / "out"
            write_outputs(data, out)
            report = (out / "report.md").read_text(encoding="utf-8")
            ledger = json.loads((out / "evidence-ledger.json").read_text(encoding="utf-8"))
        self.assertIn("現時点では、**「日本の会社は減っている」とは結論できない**", report)
        self.assertIn("-30.8%", report)
        self.assertIn("+1.1%", report)
        self.assertNotIn("-9.0%", report)
        self.assertEqual([entry["claim_id"] for entry in ledger], ["C1", "C2", "C3", "C4"])

    def test_missing_optional_columns_default_to_observed_population_scope(self):
        with tempfile.TemporaryDirectory() as tmp:
            data = Path(tmp) / "p0.csv"
            write_p0_only_csv(data)
            records = load_records(data)
        record = select_record(records, "enterprise_equivalents", "2021")
        self.assertEqual(record.harmonization_method, "observed")
        self.assertEqual(record.population_definition_id, record.population_scope)


class PublicEvidenceReportP1HarmonizationTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.records = load_records(DATA)

    def test_harmonized_delta_is_deterministic(self):
        result = compare_metrics(
            self.records,
            baseline=("enterprise_equivalents", "2021"),
            current=("enterprise_equivalents_harmonized", "2024"),
        )
        self.assertEqual(result.delta, -332_030)
        self.assertEqual(result.pct_change, Decimal("-9.0"))

    def test_harmonized_comparison_is_reference_not_observed_comparable(self):
        result = compare_metrics(
            self.records,
            baseline=("enterprise_equivalents", "2021"),
            current=("enterprise_equivalents_harmonized", "2024"),
        )
        self.assertEqual(result.comparability, HARMONIZED_REFERENCE)
        self.assertFalse(result.comparable_population)

    def test_reference_company_count_is_kept_as_separate_metric(self):
        result = compare_metrics(
            self.records,
            baseline=("company_enterprises", "2021"),
            current=("company_enterprises_reference", "2024"),
        )
        self.assertEqual(result.delta, -45_864)
        self.assertEqual(result.pct_change, Decimal("-2.6"))
        self.assertEqual(result.comparability, NOT_COMPARABLE)

    def test_harmonized_report_outputs_include_reference_delta(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp)
            write_outputs(DATA, out)
            self.assertEqual(
                {p.name for p in out.iterdir()},
                {"report.md", "evidence-ledger.json", "note-draft.md", "sns-summary.md"},
            )
            report = (out / "report.md").read_text(encoding="utf-8")
            note = (out / "note-draft.md").read_text(encoding="utf-8")
            sns = (out / "sns-summary.md").read_text(encoding="utf-8")
            ledger = json.loads((out / "evidence-ledger.json").read_text(encoding="utf-8"))
        self.assertIn("現時点では、**「日本の会社は減っている」とは結論できない**", report)
        self.assertIn("-30.8%", report)
        self.assertIn("-9.0%", report)
        self.assertIn("-9.0%", note)
        self.assertIn("-9.0%", sns)
        self.assertGreaterEqual(len(ledger), 6)
        self.assertTrue(all("comparability" in entry for entry in ledger))
        self.assertNotIn(COMPARABLE, {entry["comparability"] for entry in ledger})


class ComparabilityGuardrailTest(unittest.TestCase):
    def test_different_population_definitions_are_never_comparable(self):
        baseline = make_record("m", "2021", 100, "incl_no_employee_individual")
        current = make_record("m", "2024", 90, "excl_no_employee_individual")
        self.assertEqual(label_comparability(baseline, current), NOT_COMPARABLE)

    def test_same_definition_with_observed_values_is_comparable(self):
        baseline = make_record("m", "2021", 100, "incl_no_employee_individual")
        current = make_record("m", "2024", 90, "incl_no_employee_individual")
        self.assertEqual(label_comparability(baseline, current), COMPARABLE)

    def test_same_definition_reached_by_carry_forward_is_only_a_harmonized_reference(self):
        baseline = make_record("m", "2021", 100, "incl_no_employee_individual")
        current = make_record("m", "2024", 90, "incl_no_employee_individual", method="carry_forward_2021_no_employee_individual")
        self.assertEqual(label_comparability(baseline, current), HARMONIZED_REFERENCE)

    def test_ledger_cannot_relabel_a_mismatched_comparison_as_comparable(self):
        records = load_records(DATA)
        comparisons = {
            "naive": compare(records, "enterprise_equivalents", "2021", "2024"),
            "company": compare(records, "company_enterprises", "2021", "2024"),
        }
        ledger = build_ledger(comparisons)
        validate_ledger(ledger, comparisons)  # checked-in ledger is consistent
        tampered = ledger[0].__class__(**{**ledger[0].__dict__, "comparability": COMPARABLE})
        with self.assertRaises(ValueError):
            validate_ledger([tampered, *ledger[1:]], comparisons)


if __name__ == "__main__":
    unittest.main()
