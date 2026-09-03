# Insight Lab

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.25%2B-00ADD8.svg)](go.mod)

顧客インタビュー・レビュー・問い合わせなどのテキストから、潜在ニーズと改善仮説を抽出するローカル実行型の分析ツール。Go 単一バイナリ、埋め込み Web UI、SQLite、OpenAI 互換 API で動く。

Insight は必ず原文照合済みの Evidence・反証・Confidence とセットで表示する。推論の過程（常識的な予想 → 予想とのズレ → 仮説 → 説明）は UI からたどれる。Confidence と品質チェック（顕在ニーズの言い換え・抽象語・痕跡なし・推論不完全）は LLM に自己申告させず、アプリ側で計算・判定する。設計の詳細は [docs/detailed-design.md](docs/detailed-design.md) を参照。

## スクリーンショット

| Insight 詳細 | 評価指標 |
|---|---|
| ![Insight 詳細](docs/screenshots/insight-detail.png) | ![評価指標](docs/screenshots/evaluation.png) |

その他: [品質チェック付き Insight](docs/screenshots/insight-quality-flags.png) / [痕跡とパターン一覧](docs/screenshots/traces-and-patterns.png) / [プロジェクト画面](docs/screenshots/project.png)

## クイックスタート

必要なもの: Go 1.25 以上（CGO 不要）、OpenAI 互換の LLM API。

```bash
make build-demo
./bin/insight-lab-demo --demo
```

`http://127.0.0.1:8787` がブラウザで開く。設定画面で Base URL / Model / API Key を入力するか、`--base-url` `--model` `--api-key` フラグで渡す。

## 使い方

1. 「デモを試す」で同梱のサンプル（架空の請求書 SaaS インタビュー 20 件）を開く
2. 「解析を実行」で Insight が Evidence・反証・Confidence 付きで表示される
3. Insight 詳細で推論の過程と品質警告を確認する。Evidence をクリックすると原文の該当箇所がハイライトされる
4. 「痕跡・パターン一覧」「評価指標」で Insight に至らなかった気づきや Evidence Coverage などを見る
5. 「レポートを保存」で Markdown レポートをダウンロードする

自分のデータは貼り付けか CSV（`id,source,title,content`）でインポートする。レポートは API からも取得できる。

```bash
curl -o report.md http://127.0.0.1:8787/api/projects/<projectID>/report.md
```

## ビルド

| コマンド | 内容 |
|---|---|
| `make build` | デモデータを含まないビルド（デフォルト）。`--demo` は拒否される |
| `make build-demo` | デモデータを埋め込んだビルド |
| `make cross-compile` | 両ビルド × macOS / Linux / Windows |

デモデータは build tag で分岐するため、デフォルトビルドのバイナリには一切含まれない。

## 開発

```bash
make vet
make test
```

単体テストと E2E はフェイク LLM で決定的に回す。実 LLM での出力品質は `make eval-demo` で測る。デモデータを解析し、指標と全 Insight を `docs/evaluation/<日付>-<モデル>/` に書き出す。

```bash
INSIGHT_LAB_API_KEY=sk-... INSIGHT_LAB_MODEL=gpt-5 make eval-demo
```

## ドキュメント

| ドキュメント | 内容 |
|---|---|
| [docs/detailed-design.md](docs/detailed-design.md) | 詳細設計（アーキテクチャ / パイプライン / Confidence / 品質チェック / API） |
| [docs/implementation-plan.md](docs/implementation-plan.md) | 実装プランと進捗 |
| [docs/design-review.md](docs/design-review.md) | 設計レビュー |
| [docs/business-strategy.md](docs/business-strategy.md) | 想定する使い道 |

## コントリビュート

Issue / Pull Request を歓迎します。[CONTRIBUTING.md](CONTRIBUTING.md) と [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) を参照してください。脆弱性は [SECURITY.md](SECURITY.md) の手順で報告してください。

## ライセンス

[Apache License 2.0](LICENSE)
