#!/usr/bin/env python3
"""Validate that a public-data source contract is safe to normalize."""

from __future__ import annotations

import argparse
import json
from pathlib import Path

REQUIRED = {
    "source_id", "source_name", "publisher", "source_url", "stat_infid",
    "survey_period", "geographic_scope", "table_title", "target_cell",
    "population_basis", "normalization_target_metric_id", "status",
    "known_limitations",
}
REQUIRED_TARGET = {"measure", "industry", "management_organization", "region", "period"}


def validate(path: Path) -> dict:
    data = json.loads(path.read_text(encoding="utf-8"))
    missing = REQUIRED.difference(data)
    if missing:
        raise ValueError(f"source contract missing fields: {', '.join(sorted(missing))}")
    target_missing = REQUIRED_TARGET.difference(data["target_cell"])
    if target_missing:
        raise ValueError(f"target_cell missing fields: {', '.join(sorted(target_missing))}")
    if not data["source_url"].startswith("https://www.e-stat.go.jp/"):
        raise ValueError("P1 reference-table contract must point to official e-Stat")
    if data["status"] == "normalized" and "normalized_value" not in data:
        raise ValueError("normalized sources require normalized_value")
    if data["status"] != "normalized" and "normalized_value" in data:
        raise ValueError("unverified sources must not carry a normalized_value")
    return data


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("source_contract", type=Path)
    args = parser.parse_args()
    data = validate(args.source_contract)
    print(f"OK {data['source_id']}: {data['status']}")


if __name__ == "__main__":
    main()
