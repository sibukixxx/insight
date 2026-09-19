#!/usr/bin/env python3
import json
from pathlib import Path
import tempfile
import unittest

from validate_public_source import validate

ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "reports" / "japan-company-count-2021-2024" / "sources" / "2024-reference-table.json"


class PublicSourceContractTest(unittest.TestCase):
    def test_reference_table_is_pinned_but_not_fabricated(self):
        data = validate(SOURCE)
        self.assertEqual(data["stat_infid"], "000040389375")
        self.assertEqual(data["status"], "source-identified-value-not-yet-normalized")
        self.assertNotIn("normalized_value", data)
        self.assertEqual(data["target_cell"]["region"], "全国")

    def test_unverified_contract_cannot_smuggle_a_value(self):
        data = json.loads(SOURCE.read_text(encoding="utf-8"))
        data["normalized_value"] = 123
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "source.json"
            path.write_text(json.dumps(data, ensure_ascii=False), encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "must not carry"):
                validate(path)


if __name__ == "__main__":
    unittest.main()
