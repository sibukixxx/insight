#!/usr/bin/env python3
"""Generate the 2012→2021 national decline / Tokyo concentration public-evidence report.

Reuses the normalized-CSV contract, comparability labelling and Decimal rounding of
public_evidence_report.py. Arithmetic stays deterministic and outside the LLM; the
narrative is a fixed analytical contract for this research question.
"""

from __future__ import annotations

import argparse
import json
from dataclasses import asdict, dataclass
from decimal import Decimal, ROUND_HALF_UP
from pathlib import Path
from typing import Iterable

from public_evidence_report import (
    ClaimLedgerEntry,
    Comparison,
    Record,
    compare_records,
    load_records,
    signed_int,
    signed_pct,
    validate_ledger,
    weakest_comparability,
)

RESEARCH_QUESTION = "日本の企業数は2012年から2021年に減ったのか。減少の一方で東京への集中は強まったのか？"
NATIONAL = "全国"
TOKYO = "東京都"
BASELINE_PERIOD = "2012"
CURRENT_PERIOD = "2021"
DATA_PATH = "reports/japan-company-concentration-2012-2021/normalized.csv"
OUT_PATH = "reports/japan-company-concentration-2012-2021/generated"


@dataclass(frozen=True)
class ConcentrationAnalysis:
    national: Comparison
    tokyo: Comparison
    tokyo_share_baseline: Decimal
    tokyo_share_current: Decimal
    tokyo_share_delta_pp: Decimal

    @property
    def comparisons(self) -> dict[str, Comparison]:
        return {"national": self.national, "tokyo": self.tokyo}


def select_geography(records: Iterable[Record], geography: str, period: str) -> Record:
    matches = [r for r in records if r.geographic_scope == geography and r.period == period]
    if len(matches) != 1:
        raise ValueError(f"expected exactly one record for {geography!r} {period}, found {len(matches)}")
    return matches[0]


def _share(part: int, whole: int) -> Decimal:
    return Decimal(part) / Decimal(whole) * Decimal(100)


def _round_2(value: Decimal) -> Decimal:
    return value.quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)


def build_analysis(records: list[Record]) -> ConcentrationAnalysis:
    national = compare_records(
        select_geography(records, NATIONAL, BASELINE_PERIOD), select_geography(records, NATIONAL, CURRENT_PERIOD)
    )
    tokyo = compare_records(
        select_geography(records, TOKYO, BASELINE_PERIOD), select_geography(records, TOKYO, CURRENT_PERIOD)
    )
    share_baseline = _share(tokyo.baseline.value, national.baseline.value)
    share_current = _share(tokyo.current.value, national.current.value)
    return ConcentrationAnalysis(
        national=national,
        tokyo=tokyo,
        tokyo_share_baseline=_round_2(share_baseline),
        tokyo_share_current=_round_2(share_current),
        # Difference of unrounded shares, rounded once, so regeneration never drifts.
        tokyo_share_delta_pp=_round_2(share_current - share_baseline),
    )


def source_limitations(record: Record) -> list[str]:
    return [f"{sentence.strip()}。" for sentence in record.known_limitations.split("。") if sentence.strip()]


def signed_pp(value: Decimal) -> str:
    return f"{value:+.2f}"


def _entry(
    claim_id: str,
    claim: str,
    *,
    analysis: ConcentrationAnalysis,
    dataset: list[str],
    calculation: str,
    confidence: str,
    counter_evidence: list[str],
    comparison_ids: list[str],
) -> ClaimLedgerEntry:
    source = analysis.national.baseline
    return ClaimLedgerEntry(
        claim_id=claim_id,
        claim=claim,
        source=[source.source_url],
        dataset=dataset,
        calculation=calculation,
        confidence=confidence,
        counter_evidence=counter_evidence,
        limitations=source_limitations(source)
        + [
            "2012年と2021年の離散時点比較であり、その間の年次経路を直接示さない。",
            "記述統計であり、企業数変化や地域差の因果要因を識別していない。",
        ],
        comparison_ids=comparison_ids,
        comparability=weakest_comparability(analysis.comparisons[cid] for cid in comparison_ids),
    )


def build_ledger(analysis: ConcentrationAnalysis) -> list[ClaimLedgerEntry]:
    national, tokyo = analysis.national, analysis.tokyo
    arithmetic = "HIGH for arithmetic and source transcription"
    ledger = [
        _entry(
            "C1",
            (
                f"全国の企業数は{national.baseline.period}年の{national.baseline.value:,}から"
                f"{national.current.period}年の{national.current.value:,}へ{signed_int(national.delta)}"
                f"（{signed_pct(national.pct_change)}）変化した。"
            ),
            analysis=analysis,
            dataset=_geo_refs(national),
            calculation=national.calculation,
            confidence=arithmetic,
            counter_evidence=["この集計だけでは、減少が法人の減少だけを意味するとは言えない。対象には個人事業者を含む。"],
            comparison_ids=["national"],
        ),
        _entry(
            "C2",
            (
                f"東京都の企業数も{tokyo.baseline.value:,}から{tokyo.current.value:,}へ"
                f"{signed_int(tokyo.delta)}（{signed_pct(tokyo.pct_change)}）減少した。"
            ),
            analysis=analysis,
            dataset=_geo_refs(tokyo),
            calculation=tokyo.calculation,
            confidence=arithmetic,
            counter_evidence=["東京都でも絶対数は増えていないため、「東京で企業が増えた」という説明はこのデータに反する。"],
            comparison_ids=["tokyo"],
        ),
        _entry(
            "C3",
            (
                f"東京都の全国企業数に占める比率は{analysis.tokyo_share_baseline}%から{analysis.tokyo_share_current}%へ上昇し、"
                f"差は{signed_pp(analysis.tokyo_share_delta_pp)}ポイントだった。"
            ),
            analysis=analysis,
            dataset=_geo_refs(tokyo) + _geo_refs(national),
            calculation=(
                f"{tokyo.baseline.value} / {national.baseline.value} * 100 = {analysis.tokyo_share_baseline}%; "
                f"{tokyo.current.value} / {national.current.value} * 100 = {analysis.tokyo_share_current}%; "
                f"difference = {signed_pp(analysis.tokyo_share_delta_pp)}pp (computed before rounding)"
            ),
            confidence=arithmetic,
            counter_evidence=[
                "東京都の企業数自体は同期間に減少している（C2）。",
                "全国シェア上昇だけでは、本社移転・創業・廃業・人口移動などの因果メカニズムを特定できない。",
            ],
            comparison_ids=["national", "tokyo"],
        ),
        _entry(
            "C4",
            "この期間については、「全国で企業数が減る」と「東京の相対的な比重が高まる」は同時に成立している。東京は企業数が増えたのではなく、全国より減り方が小さかった。",
            analysis=analysis,
            dataset=["C1", "C2", "C3"],
            calculation=(
                f"tokyo pct_change {signed_pct(tokyo.pct_change)} > national pct_change {signed_pct(national.pct_change)} "
                "implies a rising Tokyo share while both counts fall."
            ),
            confidence="HIGH for the arithmetic relationship; LOW for any causal reading",
            counter_evidence=["2014年・2016年を含む経路や、他の都道府県との比較はこの主張の範囲外である。"],
            comparison_ids=["national", "tokyo"],
        ),
    ]
    validate_ledger(ledger, analysis.comparisons)
    return ledger


def _geo_refs(c: Comparison) -> list[str]:
    return [
        f"{c.baseline.metric_id}:{c.baseline.geographic_scope}:{c.baseline.period}",
        f"{c.current.metric_id}:{c.current.geographic_scope}:{c.current.period}",
    ]


def render_report(analysis: ConcentrationAnalysis) -> str:
    national, tokyo = analysis.national, analysis.tokyo
    source = national.baseline
    limitations = "\n".join(f"- {line}" for line in source_limitations(source))
    return f"""# 日本の企業数は減ったのか。東京への集中は強まったのか？

更新・取得日: {source.retrieved_at}

## Research Question

{RESEARCH_QUESTION}

## 結論

中小企業庁の同じ付属統計表に掲載された企業数では、全国の企業数は{national.baseline.period}年の{national.baseline.value:,}から{national.current.period}年の{national.current.value:,}へ{signed_int(national.delta)}（{signed_pct(national.pct_change)}）変化した。
東京都も{tokyo.baseline.value:,}から{tokyo.current.value:,}へ{signed_int(tokyo.delta)}（{signed_pct(tokyo.pct_change)}）減っており、東京だけ企業数が増えたわけではない。
一方、東京都の全国シェアは{analysis.tokyo_share_baseline}%から{analysis.tokyo_share_current}%へ{signed_pp(analysis.tokyo_share_delta_pp)}ポイント上昇した。
この期間については、「全国で企業数が減る」と「東京の相対的な比重が高まる」は同時に成立している。

> **Insight Labによる分析の範囲**: 元の企業数は中小企業庁の公表値であり、Insight Lab独自取得値ではない。このレポートの一次分析は、正規化、差分・変化率・全国シェアの決定的計算、比較可能性ラベル、Evidence / Counter Evidence整理である。

## 使用データ

- データ: {source.source_name}
- 公表者: {source.publisher}
- 単位: 企業（会社及び個人事業者を含む原表の企業数定義）
- 取得日: {source.retrieved_at}
- 出典URL: {source.source_url}
- 比較可能性: {weakest_comparability(analysis.comparisons.values())}（同一表・同一の population_definition_id。調査間の同等性は下記の制約を参照）

## まず何が起きているか

**DATA / OBSERVATION**

| 指標 | {national.baseline.period} | {national.current.period} | 変化 |
|---|---:|---:|---:|
| 全国企業数 | {national.baseline.value:,} | {national.current.value:,} | {signed_int(national.delta)} ({signed_pct(national.pct_change)}) |
| 東京都企業数 | {tokyo.baseline.value:,} | {tokyo.current.value:,} | {signed_int(tokyo.delta)} ({signed_pct(tokyo.pct_change)}) |
| 東京都の全国シェア | {analysis.tokyo_share_baseline}% | {analysis.tokyo_share_current}% | {signed_pp(analysis.tokyo_share_delta_pp)}ポイント |

直接観測できるのは、全国と東京都の企業数がともに減っていること、そして東京都の減少率が全国より小さかったため全国シェアが上昇したことまでである。

## 予想と違ったこと

**MISMATCH / SURPRISE**: 「東京への企業集中が進む」と聞くと、東京の企業数そのものが増えている状態を想像しやすい。しかしこのデータでは東京都の絶対数も減っている。Mismatchは、**絶対数の減少と相対シェアの上昇が同時に起きている**点にある。

## 考えられる説明

1. H1: 全国的な企業数減少の中で、東京都は他地域より減少が緩やかだった。
2. H2: 産業構成、人口・就業者構成、事業規模構成などの地域差が減少率の差に関係している。
3. H3: 本社移転、開廃業、組織再編など複数のフローが同時に作用し、ストックの地域差として表れている。
4. H4: 調査年・集計定義・対象範囲など統計上の条件差が一部に影響している可能性がある。

H2〜H4はこのデータだけでは検証していない競合仮説であり、結論ではない。

## それを支持する証拠

- 全国: {national.baseline.value:,} → {national.current.value:,}、{signed_int(national.delta)}（{signed_pct(national.pct_change)}）。
- 東京都: {tokyo.baseline.value:,} → {tokyo.current.value:,}、{signed_int(tokyo.delta)}（{signed_pct(tokyo.pct_change)}）。
- 東京都の全国シェア: {analysis.tokyo_share_baseline}% → {analysis.tokyo_share_current}%、{signed_pp(analysis.tokyo_share_delta_pp)}ポイント。
- 全国の減少率より東京都の減少率が小さいため、東京都の比率上昇は算術的に再現できる。

## 反対の証拠

- **COUNTER EVIDENCE**: 東京都の企業数自体は{signed_int(tokyo.delta)}減っている。「東京だけ企業数が増加した」という説明とは整合しない。
- シェア上昇だけでは、企業が東京へ移転したことや、東京で創業が特に増えたことは証明できない。
- この集計はストック比較であり、開業・廃業・移転というフローを直接観測していない。

## この分析では分からないこと

{limitations}

加えて、{national.baseline.period}年と{national.current.period}年の比較だけでは途中の変動経路は分からない。相関・構成比の変化から政策や人口移動などの因果効果を断定することもできない。

## 現時点で言えること

**INSIGHT**: このデータに基づく最も限定的なInsightは、**「企業数の全国的減少」と「東京の相対的比重の上昇」を分けて見る必要がある**ことだ。東京集中を議論するとき、絶対数と構成比を混同すると実態を誤って説明する。{national.current.period}年時点までの比較では、東京は「増えた」のではなく「全国より減り方が小さかった」。

## 次に検証すること

1. 47都道府県すべてで{national.baseline.period}→{national.current.period}の変化率を算出し、上位集中度や分布変化を確認する。
2. 開業・廃業・本社移転を区別できるデータを追加し、ストック変化の内訳を検証する。
3. 人口、就業者、産業構成を追加し、単純な地域シェア以外の説明を比較する。
4. 調査方法・定義変更を原典で精査し、時点比較の同等性を確認する。

## Methodology

数値計算はLLMではなく `scripts/company_concentration_report.py` で決定的に行う（Decimal、四捨五入）。入力は `{DATA_PATH}`。出力はFull Report、Evidence Ledger、note Draft、SNS Summaryである。

```bash
python3 scripts/company_concentration_report.py \\
  --input {DATA_PATH} \\
  --out-dir {OUT_PATH}
```

検証用テスト:

```bash
python3 scripts/test_company_concentration_report.py
```

Evidence Ledgerの各claimは入力行、source URL、計算式、比較可能性ラベル、反証、limitationsへ遡れる。LLMは数値のsource of truthではない。

## Sources

- {source.source_name} — {source.source_url}
- Retrieved: {source.retrieved_at}
- License / terms note: {source.license_or_terms}
"""


def render_note(analysis: ConcentrationAnalysis) -> str:
    national, tokyo = analysis.national, analysis.tokyo
    source = national.baseline
    return f"""# 「会社が減っている」と「東京一極集中」は矛盾しない

公開データで「日本の企業数は減ったのか、東京への集中は強まったのか」を調べた。

中小企業白書の付属統計表では、全国の企業数は{national.baseline.period}年の{national.baseline.value:,}から{national.current.period}年の{national.current.value:,}へ{signed_pct(national.pct_change)}。東京都も{tokyo.baseline.value:,}から{tokyo.current.value:,}へ{signed_pct(tokyo.pct_change)}と、絶対数は減っている。

それでも、東京都が全国に占める比率は{analysis.tokyo_share_baseline}%から{analysis.tokyo_share_current}%へ{signed_pp(analysis.tokyo_share_delta_pp)}ポイント上がった。東京は「増えた」のではなく「全国より減り方が小さかった」。

「東京集中」を絶対数の増加と同義にすると、この違いを見落とす。ただし、東京への移転や創業が原因だとはこのデータからは言えない。開廃業・産業構成・人口などは次の検証課題として扱う。

※元の件数は中小企業庁の公表値で、会社だけでなく個人事業者を含む。差分・変化率・全国シェアの再計算がこのレポートによる一次分析。

Source: {source.source_url}
Retrieved: {source.retrieved_at}
"""


def render_sns(analysis: ConcentrationAnalysis) -> str:
    national, tokyo = analysis.national, analysis.tokyo
    return f"""日本の企業数は減った？東京集中は強まった？

結論:
{national.baseline.period}→{national.current.period}で企業数は全国で{signed_pct(national.pct_change)}。東京都も{signed_pct(tokyo.pct_change)}。

意外だった数字:
東京の絶対数は減っているのに、全国シェアは{analysis.tokyo_share_baseline}%→{analysis.tokyo_share_current}%へ{signed_pp(analysis.tokyo_share_delta_pp)}ポイント上昇。

一言解釈:
「企業数の減少」と「東京の相対的比重の上昇」は同時に起こり得る。東京集中=東京の会社数が増える、ではない。

Full Report: [publish URL]
"""


def write_outputs(input_path: Path, out_dir: Path) -> None:
    analysis = build_analysis(load_records(input_path))
    ledger = build_ledger(analysis)

    out_dir.mkdir(parents=True, exist_ok=True)
    (out_dir / "report.md").write_text(render_report(analysis), encoding="utf-8")
    (out_dir / "evidence-ledger.json").write_text(
        json.dumps([asdict(entry) for entry in ledger], ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    (out_dir / "note-draft.md").write_text(render_note(analysis), encoding="utf-8")
    (out_dir / "sns-summary.md").write_text(render_sns(analysis), encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser(description="Generate the Japan company concentration public evidence report")
    parser.add_argument("--input", type=Path, required=True, help="normalized CSV input")
    parser.add_argument("--out-dir", type=Path, required=True, help="output directory")
    args = parser.parse_args()
    write_outputs(args.input, args.out_dir)


if __name__ == "__main__":
    main()
