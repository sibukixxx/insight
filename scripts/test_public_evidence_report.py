#!/usr/bin/env python3
from decimal import Decimal
from pathlib import Path
import tempfile
import unittest

from public_evidence_report import compare, load_records, write_outputs

ROOT = Path(__file__).resolve().parents[1]
DATA = ROOT / "reports" / "japan-company-count-2021-2024" / "normalized.csv"


class PublicEvidenceReportTest(unittest.TestCase):
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

    def test_generates_all_distribution_outputs_and_ledger(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp)
            write_outputs(DATA, out)
            self.assertEqual(
                {p.name for p in out.iterdir()},
                {"report.md", "evidence-ledger.json", "note-draft.md", "sns-summary.md"},
            )
            report = (out / "report.md").read_text(encoding="utf-8")
            self.assertIn("現時点では、**「日本の会社は減っている」とは結論できない**", report)
            self.assertIn("-30.8%", report)
            self.assertIn("+1.1%", report)


if __name__ == "__main__":
    unittest.main()
