#!/usr/bin/env python3
from decimal import Decimal
from pathlib import Path
import json
import tempfile
import unittest

from company_concentration_report import (
    build_analysis,
    build_ledger,
    select_geography,
    write_outputs,
)
from public_evidence_report import COMPARABLE, load_records, validate_ledger

ROOT = Path(__file__).resolve().parents[1]
REPORT_DIR = ROOT / "reports" / "japan-company-concentration-2012-2021"
DATA = REPORT_DIR / "normalized.csv"


class CompanyConcentrationDatasetTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.records = load_records(DATA)

    def test_dataset_keeps_every_transcribed_geography_and_period(self):
        cells = {(r.geographic_scope, r.period) for r in self.records}
        geographies = {"全国", "東京都", "大阪府", "愛知県", "神奈川県", "埼玉県"}
        periods = {"2012", "2014", "2016", "2021"}
        self.assertEqual(cells, {(g, p) for g in geographies for p in periods})
        self.assertEqual(len(self.records), 24)

    def test_select_geography_rejects_missing_cell(self):
        with self.assertRaisesRegex(ValueError, "北海道"):
            select_geography(self.records, "北海道", "2012")


class CompanyConcentrationAnalysisTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.analysis = build_analysis(load_records(DATA))

    def test_national_count_declines_2012_to_2021(self):
        national = self.analysis.national
        self.assertEqual((national.baseline.value, national.current.value), (3_863_530, 3_375_255))
        self.assertEqual(national.delta, -488_275)
        self.assertEqual(national.pct_change, Decimal("-12.6"))

    def test_tokyo_absolute_count_also_declines(self):
        tokyo = self.analysis.tokyo
        self.assertEqual(tokyo.delta, -23_518)
        self.assertEqual(tokyo.pct_change, Decimal("-5.3"))

    def test_tokyo_share_rises_while_its_count_falls(self):
        self.assertEqual(self.analysis.tokyo_share_baseline, Decimal("11.57"))
        self.assertEqual(self.analysis.tokyo_share_current, Decimal("12.55"))
        self.assertEqual(self.analysis.tokyo_share_delta_pp, Decimal("0.98"))

    def test_comparisons_share_one_population_definition(self):
        self.assertEqual(self.analysis.national.comparability, COMPARABLE)
        self.assertEqual(self.analysis.tokyo.comparability, COMPARABLE)

    def test_ledger_labels_are_derived_from_comparisons(self):
        ledger = build_ledger(self.analysis)
        validate_ledger(ledger, self.analysis.comparisons)
        self.assertEqual([e.claim_id for e in ledger], ["C1", "C2", "C3", "C4"])


class CompanyConcentrationOutputTest(unittest.TestCase):
    def test_regeneration_matches_checked_in_outputs(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp)
            write_outputs(DATA, out)
            for name in ["report.md", "evidence-ledger.json", "note-draft.md", "sns-summary.md"]:
                with self.subTest(name=name):
                    expected = (REPORT_DIR / "generated" / name).read_text(encoding="utf-8")
                    self.assertEqual((out / name).read_text(encoding="utf-8"), expected)

    def test_regeneration_is_byte_identical_across_runs(self):
        with tempfile.TemporaryDirectory() as a, tempfile.TemporaryDirectory() as b:
            write_outputs(DATA, Path(a))
            write_outputs(DATA, Path(b))
            for name in ["report.md", "evidence-ledger.json", "note-draft.md", "sns-summary.md"]:
                with self.subTest(name=name):
                    self.assertEqual((Path(a) / name).read_bytes(), (Path(b) / name).read_bytes())

    def test_report_does_not_claim_tokyo_count_grew(self):
        with tempfile.TemporaryDirectory() as tmp:
            write_outputs(DATA, Path(tmp))
            report = (Path(tmp) / "report.md").read_text(encoding="utf-8")
        self.assertIn("東京都も447,113から423,595へ-23,518（-5.3%）減っており", report)
        self.assertNotIn("東京の企業数は増加", report)

    def test_ledger_json_traces_share_calculation(self):
        with tempfile.TemporaryDirectory() as tmp:
            write_outputs(DATA, Path(tmp))
            ledger = json.loads((Path(tmp) / "evidence-ledger.json").read_text(encoding="utf-8"))
        share = next(e for e in ledger if e["claim_id"] == "C3")
        self.assertEqual(
            share["calculation"],
            "447113 / 3863530 * 100 = 11.57%; 423595 / 3375255 * 100 = 12.55%; "
            "difference = +0.98pp (computed before rounding)",
        )


if __name__ == "__main__":
    unittest.main()
