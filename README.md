# Insight Lab – Hidden Needs Finder

顧客インタビュー・レビュー・問い合わせ履歴などのテキストから、明示されていない潜在ニーズ・JTBD・改善仮説を抽出するローカル実行型 AI 分析ツール。Go 単一バイナリ + 埋め込み Web UI + SQLite + OpenAI 互換 LLM API。

Insight は単独で提示せず、必ず **Insight → Evidence（原文照合済み引用）→ 反証 → Confidence** のセットで表示する。「複数の声を突き合わせて『ここが違う』と気づく」というインサイト抽出特有のブラックボックスな推論過程も、Observation（引用）→ Pattern（繰り返しへの気づき）→ Rationale（なぜその仮説に至ったか）→ Insight という連鎖として、Insight詳細画面と `#/projects/:id/patterns` ページでたどれるようにしている。

抽出エンジンはドメイン非依存。顧客インタビューだけでなく、案件サイトの募集文や伸びているSNS投稿を貼り付けても同じロジックで「隠れたニーズ」を見つけられる。各Insightには `Product Opportunity`（対象企業向けの改善提案）に加えて **`Monetization Angle`**（そのニーズを自分自身が商品・サービス化するなら何ができるか）も出力される。他社に売り込むデモにも、自分で機会を見つけて自分で作る用途にも使える。

## クイックスタート

```bash
# デモ用ビルド（架空の請求書SaaSインタビュー20件を同梱）
make build-demo
./bin/insight-lab-demo --demo

# 納品用ビルド（デモデータはバイナリに一切含まれない）
make build-delivery
./bin/insight-lab --client "顧客企業名"
```

起動すると `http://127.0.0.1:8787` でブラウザが自動で開く。ブラウザの「⚙ 設定」から OpenAI 互換の Base URL / Model / API Key を設定すると（`--api-key` `--model` `--base-url` フラグでも可）、プロジェクト画面の「解析を実行」で Hidden Needs 抽出が動く。

## 使い方（ローカルで一通り試す）

```bash
make build-demo
./bin/insight-lab-demo --demo --base-url https://api.openai.com/v1 --model gpt-5 --api-key sk-...
```

1. ブラウザで「デモを試す」→ 請求書SaaSインタビュー20件のプロジェクトが開く
2. 「解析を実行」→ SSEで進捗が流れ、Hidden Need が Evidence・反証・Confidence 付きで表示される
3. Insight詳細の「推論の過程」で、元になった Pattern（繰り返しの気づき）とその Rationale（なぜこの仮説に至ったか）を確認できる。Evidence をクリックすると、元ドキュメントの該当箇所がハイライトされる（grounding 検証済みの引用のみを表示）
4. 「検出されたパターン一覧」「評価指標を見る」で、最終的な Insight に至らなかった Pattern や、Evidence Coverage / Unsupported Claim Rate などを確認できる
5. CSVインポート（`id,source,title,content` 固定列）や設定画面からの接続テストも利用可能

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

Phase 1〜3 実装完了（単一バイナリ骨格、デモ/納品ビルド分離、LLM接続、Observation抽出、Grounding Check、Hidden Needs パイプライン、Evidence/Confidence、SSE進捗、評価画面、CSVインポート、GitHub Actions CI/Release）。詳細は [docs/implementation-plan.md](docs/implementation-plan.md) を参照。
