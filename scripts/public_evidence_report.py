#!/usr/bin/env python3
"""Generate a reproducible public-evidence report from normalized aggregate data.

P0 intentionally keeps deterministic arithmetic and provenance outside the LLM.
The narrative is a fixed analytical contract for the first report; future P1/P2 work
can generalize the report spec without changing the numeric source-of-truth rule.
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


@dataclass(frozen=True)
class Record:
    metric_id: str
    period: str
    geographic_scope: str
    measure: str
    value: int
    unit: str
    population_scope: str
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
    comparable_population: bool


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


def load_records(path: Path) -> list[Record]:
    with path.open("r", encoding="utf-8", newline="") as handle:
        reader = csv.DictReader(handle)
        missing = REQUIRED_COLUMNS.difference(reader.fieldnames or [])
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
            data = {key: (row.get(key) or "").strip() for key in REQUIRED_COLUMNS}
            records.append(Record(value=value, **{k: v for k, v in data.items() if k != "value"}))
    if not records:
        raise ValueError("normalized dataset has no records")
    return records


def compare(records: Iterable[Record], metric_id: str, baseline_period: str, current_period: str) -> Comparison:
    rows = [r for r in records if r.metric_id == metric_id]
    index = {r.period: r for r in rows}
    if baseline_period not in index or current_period not in index:
        raise ValueError(f"metric {metric_id!r} requires {baseline_period} and {current_period}")
    baseline = index[baseline_period]
    current = index[current_period]
    delta = current.value - baseline.value
    pct_change = (Decimal(delta) / Decimal(baseline.value) * Decimal(100)).quantize(
        Decimal("0.1"), rounding=ROUND_HALF_UP
    )
    return Comparison(
        metric_id=metric_id,
        baseline=baseline,
        current=current,
        delta=delta,
        pct_change=pct_change,
        comparable_population=baseline.population_scope == current.population_scope,
    )


def signed_int(value: int) -> str:
    return f"{value:+,d}"


def signed_pct(value: Decimal) -> str:
    return f"{value:+.1f}%"


def build_ledger(enterprise: Comparison, company: Comparison) -> list[ClaimLedgerEntry]:
    return [
        ClaimLedgerEntry(
            claim_id="C1",
            claim=(
                f"公表された『企業等数』は{enterprise.baseline.period}年の{enterprise.baseline.value:,}から"
                f"{enterprise.current.period}年の{enterprise.current.value:,}へ{signed_int(enterprise.delta)}"
                f"（{signed_pct(enterprise.pct_change)}）異なる。"
            ),
            source=[enterprise.baseline.source_url, enterprise.current.source_url],
            dataset=[f"{enterprise.metric_id}:{enterprise.baseline.period}", f"{enterprise.metric_id}:{enterprise.current.period}"],
            calculation=f"{enterprise.current.value} - {enterprise.baseline.value} = {enterprise.delta}; {enterprise.delta} / {enterprise.baseline.value} * 100 = {enterprise.pct_change}%",
            confidence="HIGH for arithmetic; LOW for interpreting the difference as a real population trend",
            counter_evidence=["C2", "C3"],
            limitations=[
                "The 2021 and 2024 enterprise-equivalent populations are not definitionally identical.",
                "Two cross-sectional snapshots do not identify causes of change.",
            ],
        ),
        ClaimLedgerEntry(
            claim_id="C2",
            claim="2024年の甲調査は雇用者のいない個人経営の事業所を対象外としており、2021年活動調査との単純比較には対象範囲差がある。",
            source=[enterprise.current.source_url, SCOPE_SOURCE_URL],
            dataset=[f"{enterprise.metric_id}:{enterprise.current.period}"],
            calculation="population_scope(2021) != population_scope(2024)",
            confidence="HIGH for the documented scope mismatch",
            counter_evidence=[],
            limitations=["A harmonized 2024 reference table including no-employee individual establishments is still required for a like-for-like total."],
        ),
        ClaimLedgerEntry(
            claim_id="C3",
            claim=(
                f"会社企業の公表値は{company.baseline.period}年の{company.baseline.value:,}から"
                f"{company.current.period}年の{company.current.value:,}へ{signed_int(company.delta)}"
                f"（{signed_pct(company.pct_change)}）となり、『企業等数』の見かけ上の減少と逆方向である。"
            ),
            source=[company.baseline.source_url, company.current.source_url],
            dataset=[f"{company.metric_id}:{company.baseline.period}", f"{company.metric_id}:{company.current.period}"],
            calculation=f"{company.current.value} - {company.baseline.value} = {company.delta}; {company.delta} / {company.baseline.value} * 100 = {company.pct_change}%",
            confidence="HIGH for arithmetic; MEDIUM for cross-survey interpretation",
            counter_evidence=["C1"],
            limitations=[
                company.baseline.known_limitations,
                company.current.known_limitations,
                "The 2021 and 2024 surveys are different census programs and should not be treated as a fully harmonized panel without table-definition validation.",
            ],
        ),
        ClaimLedgerEntry(
            claim_id="C4",
            claim="現時点の4つの公表集計値だけでは『日本の会社は減っている』とは判断できない。",
            source=[enterprise.baseline.source_url, enterprise.current.source_url, company.baseline.source_url, company.current.source_url],
            dataset=["C1", "C2", "C3"],
            calculation="Interpretation requires scope comparability; C2 invalidates a naive C1 trend claim and C3 points in the opposite direction.",
            confidence="MEDIUM; this is an evidence-bounded interpretation rather than a population estimate",
            counter_evidence=["C1 is consistent with a decline if a future harmonized comparison confirms it."],
            limitations=[
                "The 2024 harmonized reference table has not yet been normalized into this P0 dataset.",
                "National Tax Agency longitudinal registry data has not yet been ingested for this report.",
            ],
        ),
    ]


def render_report(enterprise: Comparison, company: Comparison, ledger: list[ClaimLedgerEntry], retrieved_at: str) -> str:
    naive = f"{enterprise.baseline.value:,} → {enterprise.current.value:,}（{signed_int(enterprise.delta)}, {signed_pct(enterprise.pct_change)}）"
    company_change = f"{company.baseline.value:,} → {company.current.value:,}（{signed_int(company.delta)}, {signed_pct(company.pct_change)}）"
    return f"""# 日本の会社は本当に減っているのか？

更新・取得日: {retrieved_at}

## 結論

現時点では、**「日本の会社は減っている」とは結論できない**。公表された「企業等数」は2021年から2024年で {naive} と大きく減って見えるが、2024年調査は雇用者のいない個人経営の事業所を対象外としており、母集団が同一ではない。一方、会社企業の公表値は {company_change} と逆方向である。したがって、まず定義・対象範囲を揃えた比較が必要であり、単純なトップライン差を実体的な会社減少と読むことはできない。

> **TechVit一次分析の範囲**: 下表の元の件数は総務省統計局の公表値であり、TechVit独自調査値ではない。TechVitの一次分析は、正規化、差分・変化率の決定的計算、母集団定義の比較、Evidence / Counter Evidence整理である。

## 使用データ

| 指標 | 2021 | 2024 | 差分 | 解釈上の注意 |
|---|---:|---:|---:|---|
| 企業等数 | {enterprise.baseline.value:,} | {enterprise.current.value:,} | {signed_int(enterprise.delta)} ({signed_pct(enterprise.pct_change)}) | 2024年は雇用者のいない個人経営事業所を除外。単純比較不可 |
| 会社企業数 | {company.baseline.value:,} | {company.current.value:,} | {signed_int(company.delta)} ({signed_pct(company.pct_change)}) | 調査・表定義の差を残すため、補助的な反証材料として使用 |

対象地域は全国。2021年は令和3年経済センサス‐活動調査、2024年は令和6年経済センサス‐基礎調査（甲調査）の公表値を利用した。各行のsource URL、publisher、coverage period、利用条件、既知の制約は `normalized.csv` に保持している。

## まず何が起きているか

**DATA / OBSERVATION**: 「企業等数」の公表値だけを引けば、2021年 {enterprise.baseline.value:,} から2024年 {enterprise.current.value:,} へ {abs(enterprise.delta):,} 少なく、変化率は {enterprise.pct_change}% である。計算そのものは再現できる。

しかし「会社企業数」は2021年 {company.baseline.value:,}、2024年 {company.current.value:,} で、差は {signed_int(company.delta)}、変化率は {signed_pct(company.pct_change)} となる。少なくとも「どの企業概念を数えるか」で見える方向が一致していない。

## 予想と違ったこと

**EXPECTATION (MODEL_PROPOSED)**: もし「日本の会社が広く減っている」という説明がそのまま成立するなら、比較可能な母集団で会社企業の指標も同方向に大きく減ると予想した。

**MISMATCH / SURPRISE**: トップラインの「企業等数」は約30.8%減って見える一方、会社企業の公表値は約1.1%増えている。さらに2024年の「企業等」は雇用者のいない個人経営事業所を除くため、最も大きな差が出る指標ほど2021年と母集団が揃っていない。

## 考えられる説明

1. **H1: 実体として企業・事業活動主体が減少した。** 2021→2024のトップライン差の一部は実際の減少を含む可能性がある。
2. **H2: 見かけ上の大幅減の主因は調査対象範囲の変更である。** 2024年に雇用者のいない個人経営事業所が除外された影響が大きい可能性がある。
3. **H3: 「企業等」と「会社企業」という統計概念の違いが、同じ“会社の数”という日常語に混同されている。** 指標を変えると方向が逆転するため、用語・母集団の定義が先に必要である。

## それを支持する証拠

- H1を支持しうる観測: 「企業等数」の公表値は {naive} である。ただしC2の母集団差があるため、これだけでは実体減少量を推定できない。
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
3. `ja-company-base` を使い、国税庁法人番号データのASSIGNED / CHANGED / CLOSED等を**設立・廃業と同一視せず**イベント系列として集計する。
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

- 総務省統計局「令和3年経済センサス‐活動調査 結果の概要」: {enterprise.baseline.source_url}
- 総務省統計局「令和6年経済センサス‐基礎調査 結果の概要」: {enterprise.current.source_url}
- 2024年調査結果・利用上の注意: {SCOPE_SOURCE_URL}
- 2024年参考表（雇用者のいない個人経営を含む）: {REFERENCE_TABLE_URL}

公表値を引用する場合は各公開元の利用条件・出典表記に従う。
"""


def render_note(enterprise: Comparison, company: Comparison) -> str:
    return f"""# 「日本の会社は減っている」は、数字を見る前に定義を疑った方がいい

公開データを使って「日本の会社は本当に減っているのか」を調べ始めた。

最初に見つかる数字だけなら、かなり強い。経済センサスの「企業等数」は2021年の{enterprise.baseline.value:,}から2024年の{enterprise.current.value:,}へ、単純計算で{signed_pct(enterprise.pct_change)}になる。

ところが、ここで止めると危ない。2024年調査は雇用者のいない個人経営事業所を対象外としていて、2021年と母集団が同じではないからだ。

さらに会社企業という別の切り口を見ると、{company.baseline.value:,}から{company.current.value:,}へ{signed_pct(company.pct_change)}。方向まで逆になる。

今回の面白さは「会社が減った／増えた」という結論ではない。むしろ、公開データでは**集計できることと、比較してよいことは別**だという点にある。

次は2024年の雇用者なし個人経営を含む参考表を取り込み、同じ定義へ近づけて検証する。結論が変わるなら、それ自体が重要な結果になる。

※元の件数は総務省統計局の公表値。差分・変化率と比較可能性の整理がTechVitによる一次分析。
"""


def render_sns(enterprise: Comparison, company: Comparison) -> str:
    return f"""日本の会社は本当に減っている？

結論: 今の2つの経済センサスを単純比較しただけでは判断できない。

企業等数
{enterprise.baseline.period}: {enterprise.baseline.value:,}
{enterprise.current.period}: {enterprise.current.value:,}
単純差: {signed_int(enterprise.delta)} ({signed_pct(enterprise.pct_change)})

でも2024年は「雇用者のいない個人経営」を対象外。
一方、会社企業数は {company.baseline.value:,} → {company.current.value:,} ({signed_pct(company.pct_change)}) と逆方向。

大事なのは計算より先に母集団を揃えること。
Full Report: [publish URL]
"""


def write_outputs(input_path: Path, out_dir: Path) -> None:
    records = load_records(input_path)
    enterprise = compare(records, "enterprise_equivalents", "2021", "2024")
    company = compare(records, "company_enterprises", "2021", "2024")
    retrieved_at = max(r.retrieved_at for r in records)
    ledger = build_ledger(enterprise, company)

    out_dir.mkdir(parents=True, exist_ok=True)
    (out_dir / "report.md").write_text(render_report(enterprise, company, ledger, retrieved_at), encoding="utf-8")
    (out_dir / "evidence-ledger.json").write_text(
        json.dumps([asdict(entry) for entry in ledger], ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    (out_dir / "note-draft.md").write_text(render_note(enterprise, company), encoding="utf-8")
    (out_dir / "sns-summary.md").write_text(render_sns(enterprise, company), encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser(description="Generate Insight Lab public evidence report outputs")
    parser.add_argument("--input", type=Path, required=True, help="normalized CSV input")
    parser.add_argument("--out-dir", type=Path, required=True, help="output directory")
    args = parser.parse_args()
    write_outputs(args.input, args.out_dir)


if __name__ == "__main__":
    main()
