#!/usr/bin/env python3
"""Generate a reproducible public-evidence report from normalized aggregate data.

Deterministic arithmetic, comparability labelling and provenance stay outside the LLM.
The narrative is a fixed analytical contract for the first report. P1 adds a harmonized
2024 reference comparison; when the harmonized row is absent the P0 report is produced
unchanged, so the P0 result stays reproducible from the P0 dataset.
"""

from __future__ import annotations

import argparse
import csv
import json
from dataclasses import asdict, dataclass
from decimal import Decimal, ROUND_HALF_UP
from pathlib import Path
from typing import Iterable

SCOPE_SOURCE_URL = "https://www.stat.go.jp/data/e-census/2024/kekka.html"
REFERENCE_TABLE_URL = "https://www.e-stat.go.jp/stat-search/files?layout=dataset&stat_infid=000040389376"
REFERENCE_TABLE_LIST_URL = "https://www.e-stat.go.jp/stat-search/files?page=1&layout=datalist&lid=000001473659"
REFERENCE_NOTES_URL = "https://www.e-stat.go.jp/stat-search/file-download?statInfId=000040389310&fileKind=2"

# Comparability labels. Only COMPARABLE means "same population definition, both sides observed".
COMPARABLE = "COMPARABLE"
HARMONIZED_REFERENCE = "HARMONIZED_REFERENCE"
NOT_COMPARABLE = "NOT_COMPARABLE"
NOT_APPLICABLE = "NOT_APPLICABLE"
_COMPARABILITY_RANK = {NOT_COMPARABLE: 0, HARMONIZED_REFERENCE: 1, COMPARABLE: 2}

OBSERVED = "observed"

REQUIRED_COLUMNS = {
    "metric_id",
    "period",
    "geographic_scope",
    "measure",
    "value",
    "unit",
    "population_scope",
    "source_name",
    "source_url",
    "publisher",
    "retrieved_at",
    "coverage_period",
    "license_or_terms",
    "known_limitations",
}

# Optional population-definition metadata (P1). Defaults keep P0 datasets loadable.
OPTIONAL_COLUMNS = {"population_definition_id", "harmonization_method"}


@dataclass(frozen=True)
class Record:
    metric_id: str
    period: str
    geographic_scope: str
    measure: str
    value: int
    unit: str
    population_scope: str
    population_definition_id: str
    harmonization_method: str
    source_name: str
    source_url: str
    publisher: str
    retrieved_at: str
    coverage_period: str
    license_or_terms: str
    known_limitations: str


@dataclass(frozen=True)
class Comparison:
    metric_id: str
    baseline: Record
    current: Record
    delta: int
    pct_change: Decimal
    comparability: str

    @property
    def comparable_population(self) -> bool:
        return self.comparability == COMPARABLE

    @property
    def dataset_refs(self) -> list[str]:
        return [
            f"{self.baseline.metric_id}:{self.baseline.period}",
            f"{self.current.metric_id}:{self.current.period}",
        ]

    @property
    def calculation(self) -> str:
        return (
            f"{self.current.value} - {self.baseline.value} = {self.delta}; "
            f"{self.delta} / {self.baseline.value} * 100 = {self.pct_change}%"
        )


@dataclass(frozen=True)
class ClaimLedgerEntry:
    claim_id: str
    claim: str
    source: list[str]
    dataset: list[str]
    calculation: str
    confidence: str
    counter_evidence: list[str]
    limitations: list[str]
    comparison_ids: list[str]
    comparability: str


def load_records(path: Path) -> list[Record]:
    with path.open("r", encoding="utf-8", newline="") as handle:
        reader = csv.DictReader(handle)
        fieldnames = set(reader.fieldnames or [])
        missing = REQUIRED_COLUMNS.difference(fieldnames)
        if missing:
            raise ValueError(f"normalized dataset is missing columns: {', '.join(sorted(missing))}")
        records: list[Record] = []
        for row_number, row in enumerate(reader, start=2):
            try:
                value = int(row["value"].replace(",", ""))
            except (TypeError, ValueError) as exc:
                raise ValueError(f"row {row_number}: value must be an integer") from exc
            if value < 0:
                raise ValueError(f"row {row_number}: value must be non-negative")
            data = {key: (row.get(key) or "").strip() for key in REQUIRED_COLUMNS | OPTIONAL_COLUMNS}
            data["population_definition_id"] = data["population_definition_id"] or data["population_scope"]
            data["harmonization_method"] = data["harmonization_method"] or OBSERVED
            records.append(Record(value=value, **{k: v for k, v in data.items() if k != "value"}))
    if not records:
        raise ValueError("normalized dataset has no records")
    return records


def select_record(records: Iterable[Record], metric_id: str, period: str) -> Record:
    matches = [r for r in records if r.metric_id == metric_id and r.period == period]
    if len(matches) != 1:
        raise ValueError(f"expected exactly one record for {metric_id!r} {period}, found {len(matches)}")
    return matches[0]


def has_record(records: Iterable[Record], metric_id: str, period: str) -> bool:
    return any(r.metric_id == metric_id and r.period == period for r in records)


def label_comparability(baseline: Record, current: Record) -> str:
    """Derive the comparability label from population metadata. Never from prose."""
    if baseline.population_definition_id != current.population_definition_id:
        return NOT_COMPARABLE
    if baseline.harmonization_method != OBSERVED or current.harmonization_method != OBSERVED:
        return HARMONIZED_REFERENCE
    return COMPARABLE


def compare_records(baseline: Record, current: Record, metric_id: str | None = None) -> Comparison:
    delta = current.value - baseline.value
    pct_change = (Decimal(delta) / Decimal(baseline.value) * Decimal(100)).quantize(
        Decimal("0.1"), rounding=ROUND_HALF_UP
    )
    return Comparison(
        metric_id=metric_id or baseline.metric_id,
        baseline=baseline,
        current=current,
        delta=delta,
        pct_change=pct_change,
        comparability=label_comparability(baseline, current),
    )


def compare(records: Iterable[Record], metric_id: str, baseline_period: str, current_period: str) -> Comparison:
    rows = list(records)
    if not has_record(rows, metric_id, baseline_period) or not has_record(rows, metric_id, current_period):
        raise ValueError(f"metric {metric_id!r} requires {baseline_period} and {current_period}")
    return compare_records(select_record(rows, metric_id, baseline_period), select_record(rows, metric_id, current_period))


def compare_metrics(records: Iterable[Record], baseline: tuple[str, str], current: tuple[str, str]) -> Comparison:
    rows = list(records)
    return compare_records(
        select_record(rows, *baseline),
        select_record(rows, *current),
        metric_id=f"{baseline[0]}->{current[0]}",
    )


def weakest_comparability(comparisons: Iterable[Comparison]) -> str:
    labels = [c.comparability for c in comparisons]
    if not labels:
        return NOT_APPLICABLE
    return min(labels, key=_COMPARABILITY_RANK.__getitem__)


def validate_ledger(ledger: Iterable[ClaimLedgerEntry], comparisons: dict[str, Comparison]) -> None:
    """Guardrail: a claim's comparability label must be derived from its comparisons."""
    for entry in ledger:
        unknown = [cid for cid in entry.comparison_ids if cid not in comparisons]
        if unknown:
            raise ValueError(f"{entry.claim_id}: unknown comparison ids {unknown}")
        derived = weakest_comparability(comparisons[cid] for cid in entry.comparison_ids)
        if entry.comparability != derived:
            raise ValueError(
                f"{entry.claim_id}: comparability {entry.comparability!r} does not match derived {derived!r}"
            )


def signed_int(value: int) -> str:
    return f"{value:+,d}"


def signed_pct(value: Decimal) -> str:
    return f"{value:+.1f}%"


def pct_share(part: int, whole: int) -> Decimal:
    return (Decimal(abs(part)) / Decimal(abs(whole)) * Decimal(100)).quantize(Decimal("0.1"), rounding=ROUND_HALF_UP)


def _entry(
    claim_id: str,
    claim: str,
    *,
    source: list[str],
    dataset: list[str],
    calculation: str,
    confidence: str,
    counter_evidence: list[str],
    limitations: list[str],
    comparison_ids: list[str],
    comparisons: dict[str, Comparison],
) -> ClaimLedgerEntry:
    return ClaimLedgerEntry(
        claim_id=claim_id,
        claim=claim,
        source=source,
        dataset=dataset,
        calculation=calculation,
        confidence=confidence,
        counter_evidence=counter_evidence,
        limitations=limitations,
        comparison_ids=comparison_ids,
        comparability=weakest_comparability(comparisons[cid] for cid in comparison_ids),
    )


def build_ledger(comparisons: dict[str, Comparison]) -> list[ClaimLedgerEntry]:
    """Build the claim ledger. Keys: naive, company (P0); harmonized, company_reference (P1, optional)."""
    naive = comparisons["naive"]
    company = comparisons["company"]
    harmonized = comparisons.get("harmonized")
    company_reference = comparisons.get("company_reference")

    ledger = [
        _entry(
            "C1",
            (
                f"公表された『企業等数』は{naive.baseline.period}年の{naive.baseline.value:,}から"
                f"{naive.current.period}年の{naive.current.value:,}へ{signed_int(naive.delta)}"
                f"（{signed_pct(naive.pct_change)}）異なる。"
            ),
            source=[naive.baseline.source_url, naive.current.source_url],
            dataset=naive.dataset_refs,
            calculation=naive.calculation,
            confidence="HIGH for arithmetic; LOW for interpreting the difference as a real population trend",
            counter_evidence=["C2", "C3"] + (["C5"] if harmonized else []),
            limitations=[
                "The 2021 and 2024 enterprise-equivalent populations are not definitionally identical.",
                "Two cross-sectional snapshots do not identify causes of change.",
            ],
            comparison_ids=["naive"],
            comparisons=comparisons,
        ),
        _entry(
            "C2",
            "2024年の甲調査は雇用者のいない個人経営の事業所を対象外としており、2021年活動調査との単純比較には対象範囲差がある。",
            source=[naive.current.source_url, SCOPE_SOURCE_URL],
            dataset=[f"{naive.current.metric_id}:{naive.current.period}"],
            calculation=(
                f"population_definition_id(2021)={naive.baseline.population_definition_id} != "
                f"population_definition_id(2024)={naive.current.population_definition_id}"
            ),
            confidence="HIGH for the documented scope mismatch",
            counter_evidence=[],
            limitations=(
                ["The harmonized reference table (C5) closes the definitional gap only by carrying the 2021 no-employee individual values forward."]
                if harmonized
                else ["A harmonized 2024 reference table including no-employee individual establishments is still required for a like-for-like total."]
            ),
            comparison_ids=[],
            comparisons=comparisons,
        ),
        _entry(
            "C3",
            (
                f"会社企業の公表値は{company.baseline.period}年の{company.baseline.value:,}から"
                f"{company.current.period}年の{company.current.value:,}へ{signed_int(company.delta)}"
                f"（{signed_pct(company.pct_change)}）となり、『企業等数』の見かけ上の減少と逆方向である。"
            ),
            source=[company.baseline.source_url, company.current.source_url],
            dataset=company.dataset_refs,
            calculation=company.calculation,
            confidence="HIGH for arithmetic; MEDIUM for cross-survey interpretation",
            counter_evidence=["C1"] + (["C7"] if company_reference else []),
            limitations=[
                company.baseline.known_limitations,
                company.current.known_limitations,
                "The 2021 and 2024 surveys are different census programs and should not be treated as a fully harmonized panel without table-definition validation.",
            ],
            comparison_ids=["company"],
            comparisons=comparisons,
        ),
    ]

    if harmonized is None:
        ledger.append(
            _entry(
                "C4",
                "現時点の4つの公表集計値だけでは『日本の会社は減っている』とは判断できない。",
                source=[naive.baseline.source_url, naive.current.source_url, company.baseline.source_url, company.current.source_url],
                dataset=["C1", "C2", "C3"],
                calculation="Interpretation requires scope comparability; C2 invalidates a naive C1 trend claim and C3 points in the opposite direction.",
                confidence="MEDIUM; this is an evidence-bounded interpretation rather than a population estimate",
                counter_evidence=["C1 is consistent with a decline if a future harmonized comparison confirms it."],
                limitations=[
                    "The 2024 harmonized reference table has not yet been normalized into this P0 dataset.",
                    "National Tax Agency longitudinal registry data has not yet been ingested for this report.",
                ],
                comparison_ids=["naive", "company"],
                comparisons=comparisons,
            )
        )
        return ledger

    excluded_segment = harmonized.current.value - naive.current.value
    excluded_share = pct_share(excluded_segment, naive.delta)
    within_share = pct_share(harmonized.delta, naive.delta)

    ledger.append(
        _entry(
            "C4",
            (
                "参考表を含めた公表集計値でも『日本の会社は減っている』とは結論できない。"
                f"母集団定義を揃えた参考値では減少幅は{signed_pct(harmonized.pct_change)}に縮まるが、"
                "雇用者のいない個人経営の2024年の実測値は存在せず、会社企業数の増減方向は参照する表によって変わる。"
            ),
            source=[
                naive.baseline.source_url,
                naive.current.source_url,
                company.baseline.source_url,
                company.current.source_url,
                harmonized.current.source_url,
            ],
            dataset=["C1", "C2", "C3", "C5", "C6"] + (["C7"] if company_reference else []),
            calculation=(
                "Interpretation requires scope comparability; C2 invalidates a naive C1 trend claim; "
                "C5 bounds the harmonized-reference change; C6 shows most of the naive gap is the excluded segment by construction; "
                "C3/C7 disagree on the company-slice direction."
            ),
            confidence="MEDIUM; this is an evidence-bounded interpretation rather than a population estimate",
            counter_evidence=[
                "C5 is consistent with a moderate decline within the surveyed (employer / corporate) population.",
            ],
            limitations=[
                "The harmonized 2024 value is a reference value that carries the 2021 no-employee individual segment forward; it is not an observed 2024 population.",
                "National Tax Agency longitudinal registry data has not yet been ingested for this report.",
                "Prefecture / municipality breakdowns in the same reference table have not yet been normalized.",
            ],
            comparison_ids=["naive", "company", "harmonized"] + (["company_reference"] if company_reference else []),
            comparisons=comparisons,
        )
    )
    ledger.append(
        _entry(
            "C5",
            (
                f"雇用者のいない個人経営を含めた参考表の企業等数は{harmonized.current.period}年に{harmonized.current.value:,}であり、"
                f"{harmonized.baseline.period}年の{harmonized.baseline.value:,}との差は{signed_int(harmonized.delta)}（{signed_pct(harmonized.pct_change)}）である。"
            ),
            source=[harmonized.baseline.source_url, harmonized.current.source_url, REFERENCE_NOTES_URL],
            dataset=harmonized.dataset_refs,
            calculation=harmonized.calculation,
            confidence="HIGH for arithmetic; MEDIUM for reading it as a population change, because the no-employee individual segment is carried forward from 2021",
            counter_evidence=["C6", "C3"],
            limitations=[
                harmonized.current.known_limitations,
                "The reference table applies the JSIC 14th revision regrouping; the 2021 published total does not.",
            ],
            comparison_ids=["harmonized"],
            comparisons=comparisons,
        )
    )
    ledger.append(
        _entry(
            "C6",
            (
                f"単純差{signed_int(naive.delta)}のうち{excluded_segment:,}（{excluded_share}%）は、参考表が2021年値で補った"
                f"雇用者のいない個人経営の企業数に一致し、構造上その部分は2024年の変化を含まない。残る{signed_int(harmonized.delta)}（{within_share}%）が調査対象内の変化である。"
            ),
            source=[harmonized.current.source_url, REFERENCE_NOTES_URL],
            dataset=naive.dataset_refs + [f"{harmonized.current.metric_id}:{harmonized.current.period}"],
            calculation=(
                f"{harmonized.current.value} - {naive.current.value} = {excluded_segment}; "
                f"{excluded_segment} / {abs(naive.delta)} * 100 = {excluded_share}%; "
                f"{abs(harmonized.delta)} / {abs(naive.delta)} * 100 = {within_share}%"
            ),
            confidence="HIGH for arithmetic; the decomposition follows from how the reference table is constructed, not from an estimate",
            counter_evidence=["The carried-forward segment could have changed in either direction; the decomposition says nothing about that."],
            limitations=[
                "The derived no-employee individual count is implied by the table construction and is not a published figure.",
                "Changes of management organization (e.g. individual to company) move enterprises between segments and are not separated here.",
            ],
            comparison_ids=["naive", "harmonized"],
            comparisons=comparisons,
        )
    )
    if company_reference is not None:
        ledger.append(
            _entry(
                "C7",
                (
                    f"同じ参考表の会社企業数は{company_reference.current.value:,}であり、{company_reference.baseline.period}年の"
                    f"{company_reference.baseline.value:,}に対して{signed_int(company_reference.delta)}（{signed_pct(company_reference.pct_change)}）となる。"
                    f"結果の概要 表II-1の{company.current.value:,}（{signed_pct(company.pct_change)}）と符号が異なり、会社企業数の増減方向は表の定義に依存する。"
                ),
                source=[company_reference.baseline.source_url, company_reference.current.source_url, company.current.source_url],
                dataset=company_reference.dataset_refs + [f"{company.current.metric_id}:{company.current.period}"],
                calculation=(
                    f"{company_reference.calculation}; "
                    f"table gap: {company.current.value} - {company_reference.current.value} = {company.current.value - company_reference.current.value}"
                ),
                confidence="HIGH for arithmetic; LOW for either direction as a company-count trend",
                counter_evidence=["C3"],
                limitations=[company_reference.current.known_limitations],
                comparison_ids=["company_reference", "company"],
                comparisons=comparisons,
            )
        )
    validate_ledger(ledger, comparisons)
    return ledger


def _fmt_change(c: Comparison) -> str:
    return f"{c.baseline.value:,} → {c.current.value:,}（{signed_int(c.delta)}, {signed_pct(c.pct_change)}）"


def render_report(comparisons: dict[str, Comparison], ledger: list[ClaimLedgerEntry], retrieved_at: str) -> str:
    naive = comparisons["naive"]
    company = comparisons["company"]
    harmonized = comparisons.get("harmonized")
    company_reference = comparisons.get("company_reference")
    naive_change = _fmt_change(naive)
    company_change = _fmt_change(company)

    if harmonized is None:
        return _render_report_p0(naive, company, naive_change, company_change, retrieved_at)

    excluded_segment = harmonized.current.value - naive.current.value
    excluded_share = pct_share(excluded_segment, naive.delta)
    within_share = pct_share(harmonized.delta, naive.delta)
    harmonized_change = _fmt_change(harmonized)
    reference_row = ""
    reference_bullets = ""
    if company_reference is not None:
        reference_row = (
            f"| 会社企業数（参考表 第2表） | {company_reference.baseline.value:,} | {company_reference.current.value:,} | "
            f"{signed_int(company_reference.delta)} ({signed_pct(company_reference.pct_change)}) | {company_reference.comparability}。"
            f"表II-1の{company.current.value:,}と{company.current.value - company_reference.current.value:,}差。方向が表に依存 |\n"
        )
        reference_bullets = (
            f"- **COUNTER EVIDENCE to H3 as \"companies are growing\"**: 参考表 第2表の会社企業数は {_fmt_change(company_reference)} で、"
            f"表II-1の{signed_pct(company.pct_change)}と符号が逆である。会社企業の増減はどの表を引くかで変わる。\n"
        )

    return f"""# 日本の会社は本当に減っているのか？

更新・取得日: {retrieved_at}

## 結論

現時点では、**「日本の会社は減っている」とは結論できない**。公表された「企業等数」は2021年から2024年で {naive_change} と大きく減って見えるが、2024年調査は雇用者のいない個人経営の事業所を対象外としており、母集団が同一ではない。母集団定義を揃えた2024年参考表（雇用者のいない個人経営を含む）では {harmonized_change} となり、減少幅は約3分の1に縮まる。ただしこの参考値は、対象外となった個人経営の部分を2021年値で補ったものであり、2024年の実測ではない。会社企業数は結果の概要 表II-1では {company_change} と増加、参考表では {_fmt_change(company_reference) if company_reference else "（参考表値なし）"} と減少で、引く表によって方向が変わる。したがって、言えるのは「31%減という見かけの数字は母集団差の産物であり、定義を揃えた参考値では約9%減にとどまるが、それも実測ではない」までである。

> **Insight Labによる分析の範囲**: 下表の元の件数は総務省統計局の公表値であり、Insight Lab独自取得値ではない。このレポートの一次分析は、正規化、差分・変化率・寄与分解の決定的計算、母集団定義の比較可能性判定、Evidence / Counter Evidence整理である。

## 使用データ

| 指標 | 2021 | 2024 | 差分 | 比較可能性と注意 |
|---|---:|---:|---:|---|
| 企業等数（公表トップライン） | {naive.baseline.value:,} | {naive.current.value:,} | {signed_int(naive.delta)} ({signed_pct(naive.pct_change)}) | {naive.comparability}。2024年は雇用者のいない個人経営事業所を除外 |
| 企業等数（参考表・雇なし個人を含む） | {harmonized.baseline.value:,} | {harmonized.current.value:,} | {signed_int(harmonized.delta)} ({signed_pct(harmonized.pct_change)}) | {harmonized.comparability}。雇なし個人の部分は2021年値の繰越し（参考値） |
| 会社企業数（結果の概要 表II-1） | {company.baseline.value:,} | {company.current.value:,} | {signed_int(company.delta)} ({signed_pct(company.pct_change)}) | {company.comparability}。調査・表定義の差を残す補助的な反証材料 |
{reference_row}
対象地域は全国（全産業、公務を除く）。2021年は令和3年経済センサス‐活動調査、2024年は令和6年経済センサス‐基礎調査（甲調査）の公表値および参考表を利用した。各行のsource URL、publisher、population_definition_id、harmonization_method、coverage period、利用条件、既知の制約は `normalized.csv` に保持している。比較可能性ラベルは `population_definition_id` と `harmonization_method` から決定的に導出し、文章側で上書きできない。

- `COMPARABLE`: 母集団定義が同一で、両年とも実測値
- `HARMONIZED_REFERENCE`: 母集団定義は同一だが、片方が繰越し等で補われた参考値
- `NOT_COMPARABLE`: 母集団定義が異なる

## まず何が起きているか

**DATA / OBSERVATION**: 「企業等数」の公表値だけを引けば、2021年 {naive.baseline.value:,} から2024年 {naive.current.value:,} へ {abs(naive.delta):,} 少なく、変化率は {naive.pct_change}% である。計算そのものは再現できる。

雇用者のいない個人経営を含めた参考表では、2024年の企業等数は {harmonized.current.value:,} で、2021年との差は {signed_int(harmonized.delta)}（{signed_pct(harmonized.pct_change)}）である。

「会社企業数」は表II-1で {company_change}、参考表 第2表で {_fmt_change(company_reference) if company_reference else "（参考表値なし）"} と、同じ2024年調査の中でも表によって値と方向が違う。

## 予想と違ったこと

**EXPECTATION (MODEL_PROPOSED, POST_HOC)**: P0レポートは「参考表を取り込めば結論が変わるかもしれない」と次の検証を予告したが、数値の予想は記録していなかった。P1で参考表を取得した後に置いた予想は「見かけの31%減の主因が対象範囲変更なら、定義を揃えた差はゼロ近傍か逆方向になる」である。データを見た後に書いた予想であるため、事前予想（PRIOR）としては扱わない。

**MISMATCH / SURPRISE**: 定義を揃えても差はゼロにはならず、{signed_int(harmonized.delta)}（{signed_pct(harmonized.pct_change)}）が残った。単純差 {abs(naive.delta):,} のうち {excluded_segment:,}（{excluded_share}%）は参考表が2021年値で補った部分に一致し、構造上2024年の変化を含まない。残り {abs(harmonized.delta):,}（{within_share}%）は調査対象内（雇用者のいる個人経営・法人）の変化である。さらに、会社企業数は表を変えると符号が反転した。

## 考えられる説明

1. **H1: 調査対象内（雇用者のいる個人経営・法人）で実体として企業数が減少した。** 参考表の差 {signed_int(harmonized.delta)} はこの範囲の変化である。
2. **H2: 見かけ上の大幅減の主因は調査対象範囲の変更である。** 単純差の {excluded_share}% は対象外となった個人経営の部分に一致する。
3. **H3: 「企業等」「会社企業」という統計概念と表定義の違いが、同じ“会社の数”という日常語に混同されている。** 会社企業数は表によって増減方向が変わる。
4. **H4: 経営組織の変更（個人経営→会社等）や集計条件（必要事項が得られた企業のみ）の違いが、会社企業数の表間差と一部の増減を生んでいる。** 表II-1と参考表 第2表の差 {company.current.value - company_reference.current.value if company_reference else 0:,} の理由は取得した資料には記載がない。

## それを支持する証拠

- H1を支持する証拠: 参考表の企業等数は {harmonized_change}。対象外部分を固定した上でも減少が残る。
- H2を支持する証拠: 2024年調査の公式説明では雇用者のいない個人経営の事業所が甲調査の対象外であり、参考表はその部分を2021年値で補っている。単純差の {excluded_share}% がこの部分に一致する。
- H3を支持する証拠: 会社企業数は表II-1で {signed_pct(company.pct_change)}、参考表 第2表で {signed_pct(company_reference.pct_change) if company_reference else "n/a"}。
- H4を支持する証拠: 参考表の利用上の注意は、経営組織の変更（個人経営・会社・会社以外の法人等の間）を事業所の新設・廃業判定に含めると明記している。

## 反対の証拠

- **COUNTER EVIDENCE to H1 as a national total**: 参考値の雇なし個人部分は2021年値の繰越しであり、その部分が2024年に増えたか減ったかは観測されていない。全体の増減は確定できない。
- **COUNTER EVIDENCE to H2 alone**: 対象範囲変更を補正しても {signed_int(harmonized.delta)} が残るため、範囲変更だけで単純差の全てを説明することはできない。
- **COUNTER EVIDENCE to naive trend**: 2024年調査の対象範囲変更により、2021年の企業等数との単純差分は同一母集団の時系列変化ではない（{naive.comparability}）。
{reference_bullets}
## この分析では分からないこと

- 雇用者のいない個人経営の企業数が2024年に実際にどうなったか（参考表は2021年値の繰越し）。
- 調査対象内の {signed_int(harmonized.delta)} のうち、廃業・新設・経営組織変更・回答充足条件の変化がそれぞれどの程度か。
- 会社企業数の表間差 {company.current.value - company_reference.current.value if company_reference else 0:,} の原因。
- 2022年・2023年を含む連続的な企業新設・閉鎖のフロー。
- 法人番号の指定・変更・閉鎖イベントが、実際の事業開始・廃業とどこまで一致するか。
- 景気、人口、産業構成、制度変更などが変化へ与えた因果効果。

## 現時点で言えること

**INSIGHT**: 「会社が減ったか」を調べるとき、最初に必要なのは高度なモデルではなく**“何を会社として数えたのか”を固定すること**である。母集団を揃えるだけで見かけの減少幅は約3分の1になったが、揃え方そのもの（2021年値の繰越し）が新しい限界を持ち込む。「比較可能」を文章で宣言せず、母集団定義と補正方法から機械的にラベルを導くことが、公開レポートで最も再現しやすい安全装置になる。

## 次に検証すること

1. 同じ参考表の都道府県・市区町村別の企業等数を正規化し、全国集計だけでは見えない地域差を確認する。
2. 参考表「事業所の活動状態に関する集計」（存続・新設・廃業）を取り込み、ストック差ではなくフローとして変化を分解する。
3. 会社企業数の表間差の原因を、集計事項一覧と各表の注記から特定する。
4. 外部registry adapterを使い、corporate-event dataのASSIGNED / CHANGED / CLOSED等を**設立・廃業と同一視せず**イベント系列として集計する。
5. 複数年・複数データソースで同じ方向が確認できてから、企業新陳代謝に関するResearch Questionへ進む。

## Methodology

数値計算はLLMではなく `scripts/public_evidence_report.py` で決定的に行う。入力は `reports/japan-company-count-2021-2024/normalized.csv`。出力はFull Report、Evidence Ledger、note Draft Source、SNS Summaryである。参考表の取得手順・ファイルのチェックサム・読み取り位置は `ACQUISITION.md` に記録している。

```bash
python3 scripts/public_evidence_report.py \\
  --input reports/japan-company-count-2021-2024/normalized.csv \\
  --out-dir reports/japan-company-count-2021-2024/generated
```

検証用テスト:

```bash
python3 scripts/test_public_evidence_report.py
```

Evidence Ledgerの各claimは入力行、source URL、計算式、比較可能性ラベル、反証、limitationsへ遡れる。LLMは数値のsource of truthではない。

## Sources

- 総務省統計局「令和3年経済センサス‐活動調査 結果の概要」: {naive.baseline.source_url}
- 総務省統計局「令和6年経済センサス‐基礎調査 結果の概要」: {naive.current.source_url}
- 2024年調査結果・利用上の注意: {SCOPE_SOURCE_URL}
- 2024年参考表 一覧（雇用者のいない個人経営を含む）: {REFERENCE_TABLE_LIST_URL}
- 2024年参考表 第2表（企業等に関する集計）: {REFERENCE_TABLE_URL}
- 2024年参考表 利用上の注意（PDF）: {REFERENCE_NOTES_URL}

公表値を引用する場合は各公開元の利用条件・出典表記に従う。参考表の数値は参考値である旨を併記する。
"""


def _render_report_p0(naive: Comparison, company: Comparison, naive_change: str, company_change: str, retrieved_at: str) -> str:
    return f"""# 日本の会社は本当に減っているのか？

更新・取得日: {retrieved_at}

## 結論

現時点では、**「日本の会社は減っている」とは結論できない**。公表された「企業等数」は2021年から2024年で {naive_change} と大きく減って見えるが、2024年調査は雇用者のいない個人経営の事業所を対象外としており、母集団が同一ではない。一方、会社企業の公表値は {company_change} と逆方向である。したがって、まず定義・対象範囲を揃えた比較が必要であり、単純なトップライン差を実体的な会社減少と読むことはできない。

> **Insight Labによる分析の範囲**: 下表の元の件数は総務省統計局の公表値であり、Insight Lab独自取得値ではない。このレポートの一次分析は、正規化、差分・変化率の決定的計算、母集団定義の比較、Evidence / Counter Evidence整理である。

## 使用データ

| 指標 | 2021 | 2024 | 差分 | 解釈上の注意 |
|---|---:|---:|---:|---|
| 企業等数 | {naive.baseline.value:,} | {naive.current.value:,} | {signed_int(naive.delta)} ({signed_pct(naive.pct_change)}) | 2024年は雇用者のいない個人経営事業所を除外。単純比較不可 |
| 会社企業数 | {company.baseline.value:,} | {company.current.value:,} | {signed_int(company.delta)} ({signed_pct(company.pct_change)}) | 調査・表定義の差を残すため、補助的な反証材料として使用 |

対象地域は全国。2021年は令和3年経済センサス‐活動調査、2024年は令和6年経済センサス‐基礎調査（甲調査）の公表値を利用した。各行のsource URL、publisher、coverage period、利用条件、既知の制約は `normalized.csv` に保持している。

## まず何が起きているか

**DATA / OBSERVATION**: 「企業等数」の公表値だけを引けば、2021年 {naive.baseline.value:,} から2024年 {naive.current.value:,} へ {abs(naive.delta):,} 少なく、変化率は {naive.pct_change}% である。計算そのものは再現できる。

しかし「会社企業数」は2021年 {company.baseline.value:,}、2024年 {company.current.value:,} で、差は {signed_int(company.delta)}、変化率は {signed_pct(company.pct_change)} となる。少なくとも「どの企業概念を数えるか」で見える方向が一致していない。

## 予想と違ったこと

**EXPECTATION (MODEL_PROPOSED)**: もし「日本の会社が広く減っている」という説明がそのまま成立するなら、比較可能な母集団で会社企業の指標も同方向に大きく減ると予想した。

**MISMATCH / SURPRISE**: トップラインの「企業等数」は約30.8%減って見える一方、会社企業の公表値は約1.1%増えている。さらに2024年の「企業等」は雇用者のいない個人経営事業所を除くため、最も大きな差が出る指標ほど2021年と母集団が揃っていない。

## 考えられる説明

1. **H1: 実体として企業・事業活動主体が減少した。** 2021→2024のトップライン差の一部は実際の減少を含む可能性がある。
2. **H2: 見かけ上の大幅減の主因は調査対象範囲の変更である。** 2024年に雇用者のいない個人経営事業所が除外された影響が大きい可能性がある。
3. **H3: 「企業等」と「会社企業」という統計概念の違いが、同じ“会社の数”という日常語に混同されている。** 指標を変えると方向が逆転するため、用語・母集団の定義が先に必要である。

## それを支持する証拠

- H1を支持しうる観測: 「企業等数」の公表値は {naive_change} である。ただしC2の母集団差があるため、これだけでは実体減少量を推定できない。
- H2を支持する証拠: 2024年調査の公式説明では、雇用者のいない個人経営の事業所が甲調査の対象外である。
- H3を支持する証拠: 会社企業の公表値は {company_change} で、トップライン「企業等数」と方向が異なる。

## 反対の証拠

- **COUNTER EVIDENCE to H1**: 会社企業の公表値は減少ではなく約1.1%の増加を示す。この数字だけで会社総数の増加を断定することもできないが、「すべての会社概念で一貫して減っている」という説明には反する。
- **COUNTER EVIDENCE to naive trend**: 2024年調査の対象範囲変更により、2021年の企業等数との単純差分は同一母集団の時系列変化ではない。
- **COUNTER EVIDENCE to H2 alone**: 対象範囲変更だけで差の全てが説明できるかは、雇用者なし個人経営を含む2024年参考表との照合前には判断できない。

## この分析では分からないこと

- 2021年と同等の対象範囲に揃えた2024年企業等数で、実際に増減がどうなるか。
- 会社企業数の2021/2024比較が、表定義・回答充足条件まで完全に同一か。
- 2022年・2023年を含む連続的な企業新設・閉鎖のフロー。
- 法人番号の指定・変更・閉鎖イベントが、実際の事業開始・廃業とどこまで一致するか。
- 景気、人口、産業構成、制度変更などが変化へ与えた因果効果。

## 現時点で言えること

**INSIGHT**: 「会社が減ったか」を調べるとき、最初に必要なのは高度なモデルではなく**“何を会社として数えたのか”を固定すること**である。公的統計でも調査年や統計概念によって母集団は変わり得る。大きな差ほどニュース性はあるが、定義差を解消しないまま変化率へ変換すると、最も再現しやすい計算が最も誤解を生む結果になり得る。

## 次に検証すること

1. 2024年の「雇用者のいない個人経営の事業所を含む」参考表を取り込み、2021年と同一に近い母集団へ揃える。
2. 都道府県・市区町村別に同一指標を比較し、全国集計だけでは見えない地域差を確認する。
3. 外部registry adapterを使い、corporate-event dataのASSIGNED / CHANGED / CLOSED等を**設立・廃業と同一視せず**イベント系列として集計する。
4. 複数年・複数データソースで同じ方向が確認できてから、企業新陳代謝に関するResearch Questionへ進む。

## Methodology

数値計算はLLMではなく `scripts/public_evidence_report.py` で決定的に行う。入力は `reports/japan-company-count-2021-2024/normalized.csv`。出力はFull Report、Evidence Ledger、note Draft Source、SNS Summaryである。

```bash
python3 scripts/public_evidence_report.py \\
  --input reports/japan-company-count-2021-2024/normalized.csv \\
  --out-dir reports/japan-company-count-2021-2024/generated
```

検証用テスト:

```bash
python3 scripts/test_public_evidence_report.py
```

Evidence Ledgerの各claimは入力行、source URL、計算式、反証、limitationsへ遡れる。LLMは数値のsource of truthではない。

## Sources

- 総務省統計局「令和3年経済センサス‐活動調査 結果の概要」: {naive.baseline.source_url}
- 総務省統計局「令和6年経済センサス‐基礎調査 結果の概要」: {naive.current.source_url}
- 2024年調査結果・利用上の注意: {SCOPE_SOURCE_URL}
- 2024年参考表（雇用者のいない個人経営を含む）: {REFERENCE_TABLE_URL}

公表値を引用する場合は各公開元の利用条件・出典表記に従う。
"""


def render_note(comparisons: dict[str, Comparison]) -> str:
    naive = comparisons["naive"]
    company = comparisons["company"]
    harmonized = comparisons.get("harmonized")
    company_reference = comparisons.get("company_reference")
    if harmonized is None:
        return f"""# 「日本の会社は減っている」は、数字を見る前に定義を疑った方がいい

公開データを使って「日本の会社は本当に減っているのか」を調べ始めた。

最初に見つかる数字だけなら、かなり強い。経済センサスの「企業等数」は2021年の{naive.baseline.value:,}から2024年の{naive.current.value:,}へ、単純計算で{signed_pct(naive.pct_change)}になる。

ところが、ここで止めると危ない。2024年調査は雇用者のいない個人経営事業所を対象外としていて、2021年と母集団が同じではないからだ。

さらに会社企業という別の切り口を見ると、{company.baseline.value:,}から{company.current.value:,}へ{signed_pct(company.pct_change)}。方向まで逆になる。

今回の面白さは「会社が減った／増えた」という結論ではない。むしろ、公開データでは**集計できることと、比較してよいことは別**だという点にある。

次は2024年の雇用者なし個人経営を含む参考表を取り込み、同じ定義へ近づけて検証する。結論が変わるなら、それ自体が重要な結果になる。

※元の件数は総務省統計局の公表値。差分・変化率と比較可能性の整理がこのレポートによる一次分析。
"""
    excluded_segment = harmonized.current.value - naive.current.value
    excluded_share = pct_share(excluded_segment, naive.delta)
    reference_line = (
        f"会社企業数も、結果の概要の表では{signed_pct(company.pct_change)}、参考表では{signed_pct(company_reference.pct_change)}。同じ調査の中で、どの表を引くかで符号が変わる。"
        if company_reference
        else ""
    )
    return f"""# 「日本の会社は31%減った」は、定義を揃えると9%減になり、それも実測ではなかった

公開データで「日本の会社は本当に減っているのか」を調べている。前回は、経済センサスの「企業等数」が2021年の{naive.baseline.value:,}から2024年の{naive.current.value:,}へ{signed_pct(naive.pct_change)}に見えるが、2024年調査が雇用者のいない個人経営を対象外にしているので比較できない、というところで止まった。

今回は、統計局が「過去との比較用」に公開している参考表を取り込んだ。雇用者のいない個人経営を含めた2024年の企業等数は{harmonized.current.value:,}。2021年との差は{signed_int(harmonized.delta)}（{signed_pct(harmonized.pct_change)}）になる。

31%減が9%減になった。単純差のうち{excluded_share}%は、対象外になった個人経営の部分だった。

ただし、ここにも落とし穴がある。参考表の「雇用者のいない個人経営」の数は、2024年に測ったものではなく2021年の値をそのまま足したものだ。つまり9%減は「調査対象に残った企業（雇用者のいる個人経営と法人）が減った」ことしか言っていない。対象外の部分が増えたか減ったかは、誰も測っていない。

{reference_line}

結論は前回と同じで、「日本の会社は減っている」とはまだ言えない。変わったのは、見かけの31%が母集団差の産物だと数字で示せたこと、そして「定義を揃える」という操作自体が新しい限界を持ち込むと分かったことだ。

次は同じ参考表の都道府県別と、事業所の存続・新設・廃業の集計を取り込む。

※元の件数は総務省統計局の公表値。参考表の数値は参考値。差分・変化率・寄与分解と比較可能性の判定がこのレポートによる一次分析。
"""


def render_sns(comparisons: dict[str, Comparison]) -> str:
    naive = comparisons["naive"]
    company = comparisons["company"]
    harmonized = comparisons.get("harmonized")
    company_reference = comparisons.get("company_reference")
    if harmonized is None:
        return f"""日本の会社は本当に減っている？

結論: 今の2つの経済センサスを単純比較しただけでは判断できない。

企業等数
{naive.baseline.period}: {naive.baseline.value:,}
{naive.current.period}: {naive.current.value:,}
単純差: {signed_int(naive.delta)} ({signed_pct(naive.pct_change)})

でも2024年は「雇用者のいない個人経営」を対象外。
一方、会社企業数は {company.baseline.value:,} → {company.current.value:,} ({signed_pct(company.pct_change)}) と逆方向。

大事なのは計算より先に母集団を揃えること。
Full Report: [publish URL]
"""
    company_line = (
        f"会社企業数は表II-1で {signed_pct(company.pct_change)}、参考表で {signed_pct(company_reference.pct_change)}。表で符号が変わる。"
        if company_reference
        else f"会社企業数は {company.baseline.value:,} → {company.current.value:,} ({signed_pct(company.pct_change)})。"
    )
    return f"""日本の会社は本当に減っている？（続報）

結論: まだ「減っている」とは言えない。

企業等数（公表トップライン）
{naive.baseline.period}: {naive.baseline.value:,}
{naive.current.period}: {naive.current.value:,}
単純差: {signed_int(naive.delta)} ({signed_pct(naive.pct_change)})

母集団を揃えた参考表（雇用者のいない個人経営を含む）
{harmonized.current.period}: {harmonized.current.value:,}
差: {signed_int(harmonized.delta)} ({signed_pct(harmonized.pct_change)})

31%減 → 9%減。でも参考表の個人経営部分は2021年値の繰越し。2024年の実測ではない。
{company_line}

定義を揃えるだけで数字は3分の1に。揃え方にも限界がある。
Full Report: [publish URL]
"""


def build_comparisons(records: list[Record]) -> dict[str, Comparison]:
    comparisons = {
        "naive": compare(records, "enterprise_equivalents", "2021", "2024"),
        "company": compare(records, "company_enterprises", "2021", "2024"),
    }
    if has_record(records, "enterprise_equivalents_harmonized", "2024"):
        comparisons["harmonized"] = compare_metrics(
            records,
            baseline=("enterprise_equivalents", "2021"),
            current=("enterprise_equivalents_harmonized", "2024"),
        )
        if has_record(records, "company_enterprises_reference", "2024"):
            comparisons["company_reference"] = compare_metrics(
                records,
                baseline=("company_enterprises", "2021"),
                current=("company_enterprises_reference", "2024"),
            )
    return comparisons


def write_outputs(input_path: Path, out_dir: Path) -> None:
    records = load_records(input_path)
    comparisons = build_comparisons(records)
    retrieved_at = max(r.retrieved_at for r in records)
    ledger = build_ledger(comparisons)

    out_dir.mkdir(parents=True, exist_ok=True)
    (out_dir / "report.md").write_text(render_report(comparisons, ledger, retrieved_at), encoding="utf-8")
    (out_dir / "evidence-ledger.json").write_text(
        json.dumps([asdict(entry) for entry in ledger], ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    (out_dir / "note-draft.md").write_text(render_note(comparisons), encoding="utf-8")
    (out_dir / "sns-summary.md").write_text(render_sns(comparisons), encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser(description="Generate Insight Lab public evidence report outputs")
    parser.add_argument("--input", type=Path, required=True, help="normalized CSV input")
    parser.add_argument("--out-dir", type=Path, required=True, help="output directory")
    args = parser.parse_args()
    write_outputs(args.input, args.out_dir)


if __name__ == "__main__":
    main()
