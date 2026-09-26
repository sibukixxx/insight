# 実LLM評価の記録

決定的な発見能力のベンチマーク（仕込んだズレを拾うか、何もないときに黙るか、偽の関連を関連止まりにするか）は [Discovery Benchmark](discovery-benchmark.md) を参照。ここに置くのはモデルを使った評価の記録である。

`make eval-demo`（`scripts/eval-demo.sh`）の出力を、`<日付>-<モデル>/` ごとに保存する。現在の架空政策デモでは通常のInsight結果に加え、`research-run.json`と`research-report.md`も生成する。Human EvaluationはLLMに自己評価させず、人間が別途入力する。

| ファイル | 内容 |
|---|---|
| `summary.md` | 評価指標、全 Insight の推論の連鎖（予想 → ズレ → 仮説 → 説明）と品質フラグ、痕跡・パターン一覧 |
| `metrics.json` | `GET /api/projects/{id}/evaluation` の生データ |
| `insights.json` | 各 Insight の詳細（Evidence・Pattern 込み） |
| `patterns.json` | 痕跡（`kind: deviation`）と繰り返し（`kind: repetition`） |

見るべき指標:

- **Unsupported Claim Rate** — 高いほどモデルが引用を捏造している（アプリが破棄した割合）
- **Trace-backed Insights** — 「予想とのズレ」を根拠に持つ Insight の割合。低いと繰り返しの要約に留まっている
- **Quality Flagged** — 顕在ニーズの言い換え・抽象語などの警告付き Insight の割合。低いほど良い
