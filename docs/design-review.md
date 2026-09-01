# Insight Lab 詳細設計 検証レポート

対象: Insight Lab / Hidden Needs Finder 詳細設計（v0 ドラフト）
検証日: 2026-09-01

結論: **設計の骨格（単一バイナリ + 埋め込みUI + SQLite + 段階的パイプライン + Evidence必須表示）は妥当で、そのまま採用してよい。** ただし、実装に入る前に修正が必要な問題が P0 で 5 件、P1 で 8 件ある。いずれも設計レベルで解決可能で、方向転換は不要。

---

## 1. 技術選定の検証結果

| 項目 | 判定 | 備考 |
|------|------|------|
| Go 1.25 + net/http + chi v5 | ✅ 妥当 | chi は routing のみの薄い依存。標準ライブラリ中心の方針と整合 |
| modernc.org/sqlite (CGO不要) | ✅ 妥当・条件付き | pure-Go ゆえ mattn/go-sqlite3 より遅いが、本用途（数十ドキュメント）では問題なし。**WAL モード + busy_timeout 必須**（後述 P1-1） |
| embed.FS でUI埋め込み | ✅ 妥当・注意点あり | `//go:embed dist/*` は `_` や `.` で始まるファイルを除外する。**`//go:embed all:dist` を使うこと**。また dist が存在しないと `go build` 自体が失敗するため、Makefile で web ビルドを先行させるか placeholder を commit する |
| Preact + Vite + TS + Tailwind | ✅ 妥当 | React 比でバンドルが 1/10 程度。Tailwind v4 は `@tailwindcss/vite` プラグインで Vite に統合 |
| SSE（WebSocket不使用） | ✅ 妥当・注意点あり | ローカルなのでプロキシバッファリング問題は無い。ただし `EventSource` はカスタムヘッダを送れないため、**API 認証をヘッダに置く設計にすると SSE だけ認証できなくなる**（後述 P1-3） |
| OpenAI 互換 + Structured Output | ⚠️ 条件付き | `response_format: json_schema` は OpenAI / Azure / LM Studio / OpenRouter(モデル依存) / Ollama(≥0.5, `format` パラメータ) で使えるが、**「OpenAI互換」を名乗る全エンドポイントで使えるわけではない**。フォールバック戦略が必要（後述 P0-4） |
| 単一 goroutine ワーカー + メモリ内 JobManager | ✅ 妥当 | デモ用途では永続キュー不要。ただし状態遷移は DB にも記録する（後述 P0-3） |
| クロスコンパイル 4 ターゲット | ✅ 妥当 | CGO 不要構成なので `GOOS`/`GOARCH` 指定のみで成立する |

## 2. P0 — 実装前に設計を修正すべき問題

### P0-1: Evidence の StartOffset/EndOffset を LLM に出させる前提が成立しない

設計 §8 は `StartOffset/EndOffset` を持つが、§11 以降のパイプラインでは LLM が quote を返す。**LLM は元テキスト中の正確な文字オフセットを返せない**（トークナイズの都合で恒常的にずれる）。

**修正方針**: オフセットは LLM に要求しない。LLM は quote 文字列のみ返し、**アプリ側で元 Document の content に対して照合してオフセットを確定する**。

```
1. 完全一致検索
2. 失敗したら正規化一致（空白・改行・全角半角・句読点を正規化して検索）
3. それでも失敗 → その quote は「照合不能」として破棄し、Evidence にしない
```

これは同時に §35 の Hallucination 対策の実効化でもある。「Evidence ID しか参照させない」だけでは、Observation 抽出段階で LLM が発言を捏造すると防げない。**quote の実在性検証（grounding check）が本プロダクトの信頼性の根幹**であり、パイプラインの必須ステップとして明記する。

### P0-2: EvidenceCoverage（Confidence の 25%）が計算不能

§16 の Confidence 式は「対象ユーザーの何％で確認できたか」を使うが、ドメインモデルに**参加者（Participant）の概念がない**。Document と参加者は 1:1 とは限らない（同一人物のインタビューが複数回、レビューは匿名等）。

**修正方針（MVP）**: 参加者モデルは導入せず、**Document 単位のカバレッジに定義を変更**する。`EvidenceCoverage = 支持Evidenceを持つDocument数 / プロジェクト内Document数`。CSV の metadata に `participant_id` 列があれば将来それで集計する拡張余地を残す。

### P0-3: analyses（解析実行）テーブルがスキーマに存在しない

§23 は `analysisId` と status を返し、§25 は SSE で進捗を流すが、§18 のスキーマに解析実行を記録するテーブルがない。プロセス再起動でメモリ内 Job が消えると、UI からは永久に "running" に見える。

**修正方針**: `analyses` テーブルを追加（id, project_id, status, current_step, progress, error, started_at, finished_at）。ワーカーは状態遷移のたびに DB を更新する。起動時に `running`/`queued` のまま残っている行を `failed`（reason: interrupted）に倒す。

### P0-4: Structured Output のフォールバックが未定義

「OpenAI 互換ならどこでも動く」は json_schema については成立しない。デモ先で顧客の LLM エンドポイントに繋ぐ場面で最も壊れやすい箇所。

**修正方針**: 3 段フォールバックを LLM クライアント層に実装する。

```
1. response_format: json_schema（strict）
2. 400等で拒否されたら response_format: json_object + スキーマをプロンプトに埋め込み
3. アプリ側で必ず JSON パース + スキーマバリデーション。失敗時はエラー内容を添えて最大2回リトライ
```

どの段で成功したかは設定画面の「接続テスト」で表示する（デモ前の事前確認用）。

### P0-5: Evidence と Observation のリンク欠落

パイプラインは Document → Observation → Pattern → Hypothesis → Evidence と流れるのに、`evidence` テーブルは `document_id` 直結で observation を参照しない。Evidence Retrieval を「Observation 集合に対する検索」として実装する（§15）なら、追跡可能性のために **`evidence.observation_id`（nullable）を追加**する。Evidence クリック時の右パネル（§28）で「この Evidence がどの Observation 経由で使われたか」を出せるようになり、デモの説得力にも直結する。

## 3. P1 — 実装フェーズ内で対処すべき問題

1. **SQLite 運用設定**: `PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON` を接続時に必須化。書き込みは実質単一ワーカーなので競合はほぼ無いが、HTTP ハンドラとワーカーの同時書き込みはあり得る。
2. **スキーマの制約不足**: FK 制約・インデックス（`documents.project_id`, `observations.document_id`, `insights.project_id`, `evidence.insight_id`）・`CHECK(evidence_type IN ('support','counter','neutral'))`・`schema_migrations` テーブルを追加（修正版 DDL は detailed-design.md §DB）。
3. **SSE と認証の整合**: API Key をブラウザセッションに置く場合、`EventSource` はヘッダを付けられない。**HttpOnly Cookie のセッション ID で紐付け、API Key はサーバ側メモリのセッションストアにのみ保持**する（localStorage への Key 保存は禁止）。localhost バインドとはいえ、他プロセスからの CSRF 的アクセスに対し `Origin`/`Host` 検証も入れる。
4. **失敗系イベント**: SSE に `event: error`（step 名 + ユーザー向けメッセージ）を追加。§25 には completed しかない。デモ中に最も価値があるのは「どこで何故失敗したか」が即座に見えること。
5. **LLM 呼び出しの堅牢化**: タイムアウト（ステップ毎 120s）、429/5xx の指数バックオフ再試行（最大3回）、コンテキスト超過対策として Document のチャンク上限（例: 8,000 字 / チャンク、チャンク毎に Observation 抽出）を仕様化。
6. **Insight 重複排除**: 評価指標に Insight Duplication（§38）があるのに、生成パイプラインに dedupe ステップがない。Insight Generation の直後に「生成済み Insight リストを渡して統合させる」1 コールを追加する。
7. **CSV 実務対応**: Excel 出力の UTF-8 BOM 除去、CRLF、フィールド内改行（encoding/csv は対応済）を仕様に明記。UTF-8 以外は初期版非対応とエラーメッセージで明示。
8. **`--demo` の冪等性**: 既にデモプロジェクトが存在する場合は再作成せず既存を開く（毎回増殖しない）。デモデータは固定 ID で埋め込む。

## 4. P2 — 将来課題（設計から落とさないための記録）

- OS Keychain 対応（§21 に記載済）
- `--local-only`（外部通信ブロック。§42 に記載済。実装は http.Transport の DialContext でホワイトリスト制御）
- **UI 言語の決定**: 設計書はモック UI が英語・データが日本語で混在。デモ相手が日本企業なら **UI 日本語 / データ日本語** を初期版のデフォルトにすべき（i18n 機構は不要、まず日本語で書く）
- **レポートエクスポート（Markdown/PDF）**: 設計に無いが、商談後に「今日の解析結果を置いていく」ができると営業価値が大きい。Phase 5 候補として追加を推奨
- コスト・トークン使用量の表示（解析 1 回あたりの API コスト概算）

## 5. 設計書内部の不整合（軽微・要修正）

| 箇所 | 不整合 | 対処 |
|------|--------|------|
| §10 と §46 | パイプライン図に Contradiction Detection が入っているが、MVP スコープ（§46）には含まれない | MVP パイプラインを縮約版として別図で定義（detailed-design.md §パイプライン） |
| §9 Insight モデル | `Title` があるがスキーマ・出力スキーマの説明で言及が薄い | JSON Schema に title を含める |
| §22 API 一覧 | `GET /api/analysis/{id}/events`（§25）が一覧から漏れている。`GET /api/analysis/{id}` も必要 | API 一覧に追加 |
| §37 評価画面 | Unsupported Claim Rate 等の算出タイミング未定義 | 解析完了時に計算して analyses 行に保存 |
| §20 のモデル例 `gpt-5` | 例示としては可 | デフォルト値はユーザー入力必須とし、ハードコードしない |

## 6. 総評

- 「Insight → Evidence → 反証 → Confidence をセットで出す」というプロダクトの核は、パイプライン分割・Evidence ID 参照方式・アプリ側 Confidence 計算という設計に正しく落ちている。
- 最大のリスクは **quote の実在性検証の欠落（P0-1）** で、ここが無いと「AI が存在しない顧客発言を引用する問題を防ぐ」という売り文句が成立しない。逆にここを固めれば、このツールの差別化は技術的に裏付けられる。
- 2 番目のリスクは **デモ先での LLM 接続失敗（P0-4）**。接続テスト機能とフォールバックは MVP に含めるべき。
