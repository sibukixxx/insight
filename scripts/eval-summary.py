#!/usr/bin/env python3
"""Render docs/evaluation/<run>/summary.md from the JSON an eval run saved."""
import json, sys, datetime
from urllib.parse import urlparse

out, model, base_url = sys.argv[1], sys.argv[2], sys.argv[3]
metrics = json.load(open(f"{out}/metrics.json", encoding="utf-8"))
insights = json.load(open(f"{out}/insights.json", encoding="utf-8"))
patterns = json.load(open(f"{out}/patterns.json", encoding="utf-8"))

FLAG = {"stated_need_echo": "顕在ニーズの言い換え", "generic_term": "抽象語",
        "no_trace": "痕跡なし", "abduction_incomplete": "推論が不完全"}
DEV = {"contradiction": "言行不一致", "excess_effort": "急いでいるのに手間をかける",
       "excess_payment": "予定より多く払う", "persistence": "不満なのに使い続ける",
       "absence": "起きるはずの行動がない", "other": "その他"}

def pct(v): return f"{round((v or 0) * 100)}%"
def flags(fs): return "、".join(FLAG.get(f["code"], f["code"]) + (f"（{f['detail']}）" if f.get("detail") else "") for f in fs) or "なし"
def cell(s): return (s or "-").replace("|", "／").replace("\n", " ")

print(f"# 実LLM評価: {model}\n")
print(f"- 日時: {datetime.date.today().isoformat()}")
print(f"- モデル: `{model}`（エンドポイント: {urlparse(base_url).hostname}）")
print("- データ: 同梱デモデータ（架空の請求書SaaSインタビュー20件）\n")

print("## 評価指標\n")
print("| 指標 | 値 |\n|---|---|")
rows = [("Evidence Coverage", pct(metrics.get("evidenceCoverage"))),
        ("Unsupported Claim Rate", pct(metrics.get("unsupportedClaimRate"))),
        ("Counter Evidence Coverage", pct(metrics.get("counterEvidenceCoverage"))),
        ("Insight Duplication", pct(metrics.get("insightDuplicationRate"))),
        ("Trace-backed Insights", pct(metrics.get("traceBackedInsightRate"))),
        ("Quality Flagged", pct(metrics.get("qualityFlaggedInsightRate"))),
        ("Avg Evidence / Insight", f"{metrics.get('averageEvidencePerInsight', 0):.1f}"),
        ("観察候補 → 原文照合済み", f"{metrics.get('totalObservationCandidates')} → {metrics.get('groundedObservations')}"),
        ("痕跡 / 気づき合計", f"{metrics.get('traceCount', 0)} / {metrics.get('patternCount')}"),
        ("洞察候補 → 最終", f"{metrics.get('totalInsightDrafts')} → {metrics.get('finalInsightCount')}")]
for k, v in rows: print(f"| {k} | {v} |")
fc = metrics.get("qualityFlagCounts") or {}
if fc: print("\n品質フラグ内訳: " + " / ".join(f"{FLAG.get(k, k)} {v}件" for k, v in fc.items()))

print("\n## Insights\n")
for i in sorted(insights, key=lambda x: -x.get("confidence", 0)):
    print(f"### {i['title']}（Confidence {pct(i.get('confidence'))}）\n")
    print(f"- 品質フラグ: {flags(i.get('qualityFlags') or [])}")
    print(f"- ① 予想: {cell(i.get('expectation'))}")
    print(f"- ② ズレ: {cell(i.get('surprisingFact'))}")
    print(f"- ③ 仮説（Latent Need）: {cell(i.get('latentNeed'))}")
    print(f"- ④ 説明: {cell(i.get('rationale'))}")
    print(f"- Stated Need: {cell(i.get('statedNeed'))}")
    print(f"- 別解釈: {cell(i.get('alternativeInterpretation'))}")
    sup = [e for e in i.get("evidence", []) if e["type"] == "support"]
    cnt = [e for e in i.get("evidence", []) if e["type"] == "counter"]
    print(f"- Evidence: 支持 {len(sup)} 件 / 反証 {len(cnt)} 件")
    for e in sup[:3]: print(f"  - 「{cell(e['quote'])}」")
    for e in cnt[:2]: print(f"  - 反証「{cell(e['quote'])}」")
    print()

traces = [p for p in patterns if p.get("kind") == "deviation"]
reps = [p for p in patterns if p.get("kind") != "deviation"]
print(f"## 欲望の痕跡（予想とのズレ） {len(traces)} 件\n")
for p in traces:
    print(f"- **{p['title']}**（{DEV.get(p.get('deviationType'), p.get('deviationType'))}）  ")
    print(f"  予想: {cell(p.get('expectation'))}  ")
    print(f"  実際: {cell(p.get('description'))}（引用 {len(p.get('observations', []))} 件）")
print(f"\n## 繰り返しのパターン {len(reps)} 件\n")
for p in reps:
    print(f"- **{p['title']}** — {cell(p.get('description'))}（引用 {len(p.get('observations', []))} 件）")
