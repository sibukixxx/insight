# Insight Lab – Hidden Needs Finder

顧客インタビュー・レビュー・問い合わせ履歴などのテキストから、明示されていない潜在ニーズ・JTBD・改善仮説を抽出するローカル実行型 AI 分析ツール。Go 単一バイナリ + 埋め込み Web UI + SQLite + OpenAI 互換 LLM API。

Insight は単独で提示せず、必ず **Insight → Evidence（原文照合済み引用）→ 反証 → Confidence** のセットで表示する。

## ドキュメント

| ドキュメント | 内容 |
|---|---|
| [docs/detailed-design.md](docs/detailed-design.md) | 確定版詳細設計 v1（アーキテクチャ / ドメインモデル / パイプライン / DDL / API） |
| [docs/design-review.md](docs/design-review.md) | ドラフト設計の検証レポート（P0/P1/P2 の指摘と修正方針） |
| [docs/implementation-plan.md](docs/implementation-plan.md) | フェーズ別実装プラン（Phase 1〜6、完了条件、リスク） |

## ステータス

設計フェーズ完了。Phase 1（単一バイナリ骨格）から実装開始予定。
