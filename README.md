# Insight Lab – Hidden Needs Finder

顧客インタビュー・レビュー・問い合わせ履歴などのテキストから、明示されていない潜在ニーズ・JTBD・改善仮説を抽出するローカル実行型 AI 分析ツール。Go 単一バイナリ + 埋め込み Web UI + SQLite + OpenAI 互換 LLM API。

Insight は単独で提示せず、必ず **Insight → Evidence（原文照合済み引用）→ 反証 → Confidence** のセットで表示する。

## クイックスタート

```bash
# デモ用ビルド（架空の請求書SaaSインタビュー20件を同梱）
make build-demo
./bin/insight-lab-demo --demo

# 納品用ビルド（デモデータはバイナリに一切含まれない）
make build-delivery
./bin/insight-lab --client "顧客企業名"
```

起動すると `http://127.0.0.1:8787` でブラウザが自動で開く。

## デモビルドと納品ビルドの分離

顧客への納品物にサンプルデータ（自社の営業用テキスト）が混入しないよう、ビルド時点で分離している。

| コマンド | 用途 | デモデータ |
|---|---|---|
| `make build-demo` | 商談・営業デモ | 埋め込み済み。`--demo` で自動ロード、UIの「デモを試す」でも可 |
| `make build-delivery`（デフォルト） | 顧客納品 | **コンパイル時にバイナリへ一切リンクされない**（`internal/sampledata` の Go build tag で分岐） |

納品ビルドで `--demo` を指定するとエラーで起動を拒否する。`make cross-compile` で両ビルド × 4 プラットフォーム（macOS arm64/amd64, Linux amd64, Windows amd64）を一括生成できる。

## 開発

```bash
make vet    # go vet（デモ/納品タグ両方）
make test   # go test（デモ/納品タグ両方）
```

## ドキュメント

| ドキュメント | 内容 |
|---|---|
| [docs/detailed-design.md](docs/detailed-design.md) | 確定版詳細設計 v1（アーキテクチャ / ドメインモデル / パイプライン / DDL / API / ビルド分離） |
| [docs/design-review.md](docs/design-review.md) | ドラフト設計の検証レポート（P0/P1/P2 の指摘と修正方針） |
| [docs/implementation-plan.md](docs/implementation-plan.md) | フェーズ別実装プラン（Phase 1〜6、完了条件、リスク） |

## ステータス

Phase 1（単一バイナリ骨格・デモ/納品ビルド分離）実装完了。Phase 2（LLM接続・Observation抽出・Grounding Check）に着手予定。
