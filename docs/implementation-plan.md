# Insight Lab 実装プラン

[detailed-design.md](./detailed-design.md)（確定版 v1）に基づく実装フェーズ計画。各フェーズは「その時点でデモ可能な状態」で終わることを条件とする。

## フェーズ構成

### Phase 1 — 骨格（単一バイナリで画面が出る）✅ 完了

**ゴール**: `./insight-lab` 一発でブラウザが開き、埋め込み UI にサンプルデータが表示される。

- [x] リポジトリ初期化: `go.mod`, Makefile, ディレクトリ構成
- [x] SQLite 接続層（modernc.org/sqlite, WAL/busy_timeout/FK PRAGMA）+ マイグレーション機構（schema_migrations + embed SQL）
- [x] スキーマ 001_init.sql（確定版 DDL 全テーブル）
- [x] ドメインモデル + repository（projects / documents）
- [x] HTTP サーバ + chi ルーティング + logging / recover ミドルウェア + Host/Origin 検証
- [x] 埋め込み UI（Phase 1 は素の HTML/CSS/JS を `internal/web/dist` に直接コミットし `//go:embed all:dist`。Vite/Preact への移行は複雑化が必要になった時点で行う）
- [x] CLI フラグ（--port --host --db --demo --no-browser --client）+ ブラウザ自動起動（3 OS 分岐）
- [x] サンプルデータ（請求書 SaaS インタビュー20件、表面「操作が面倒」/ 深層「誤請求への恐怖」）の作成と `--demo` 冪等ロード
- [x] projects / documents API + 一覧・詳細 UI（プロジェクト作成、テキスト貼り付け）
- [x] **デモ/納品ビルド分離**（`internal/sampledata` を `//go:build demo` / `!demo` で分割。納品ビルドはサンプルテキストがバイナリに一切リンクされないことを確認済み。`make build-demo` / `make build-delivery` / `make cross-compile`）
- [x] unit test（domain, repository/sqlite の CRUD・カスケード削除・マイグレーション冪等性、sampledata の build tag 分岐）+ `go vet`（両タグ）

**完了条件の検証**: デモビルドを起動 → `/api/projects` に20件ドキュメントの入ったデモプロジェクトが自動生成される → ブラウザで一覧・本文が閲覧できる → 納品ビルドは `--demo` 指定時にエラー終了し、バイナリに `経理担当` 等のサンプル文言が一切含まれないことを `grep` で確認済み。

**完了条件**: バイナリ起動 → Try Demo → 20 件のインタビューが閲覧できる。

### Phase 2 — LLM 接続と Observation 抽出 ✅ 完了

**ゴール**: LLM 設定 → 解析実行 → Observation が抽出・検証・保存される。

- [x] LLM Client 抽象 + OpenAI 互換実装（`internal/llm`。timeout 120s / 429・5xx 指数バックオフ最大3回 / json_schema→json_object フォールバック / バリデーション失敗時フィードバック付き再試行最大2回）
- [x] Structured Output 3 段フォールバック + レスポンスバリデーション（`httptest` でフォールバック遷移・リトライ・コードフェンス除去まで単体テスト済み）
- [x] 設定ストア・設定画面（Base URL / Model / API Key）。**\[変更\]** HttpOnly Cookie セッションではなく、プロセス内シングルトンの `SettingsStore`（`internal/service/settings.go`）に簡略化。ローカル単一操作者向けツールという前提でセッション分離の複雑さを避けた。ディスク/DBには一切保存しない点は設計どおり
- [x] `POST /api/settings/test` 接続テスト（フォールバック段数 `mode` を返す）
- [x] Normalize + Chunk（8,000字上限、段落境界優先分割。`internal/service/textproc.go`）
- [x] Observation Extraction プロンプト + JSON Schema（`internal/service/prompts.go` / `pipeline_types.go`）
- [x] **Grounding Check**（完全一致→正規化一致（全角半角・句読点・空白を除去）→破棄。`internal/service/grounding.go`。ルーンオフセット。単体テストで捏造引用の拒否・部分一致拒否・オフセット復元を検証）
- [x] JobManager（キュー + worker×2 + `analyses` テーブル書き込み + 起動時 `FailInterrupted`）
- [x] SSE エンドポイント（progress / error / completed）+ 解析中 UI（進捗バー・ステップメッセージ）

**完了条件の検証**: フェイクLLMサーバー（OpenAI互換）を使い、実HTTP経由で解析を実行 → Observationが原文照合され保存されることを確認済み。ブラウザE2Eテスト（Playwright）でも解析実行→SSE進捗→完了までを確認。

### Phase 3 — Insight 生成（プロダクトの核） ✅ 完了

**ゴール**: Hidden Need が Evidence・反証・Confidence 付きで表示される。

- [x] Pattern Detection ステップ（**\[変更\]** 当初は結果を保存せず内部でしか使っていなかったが、「インサイト抽出のブラックボックス化」の指摘を受けて `Pattern` をDBに永続化し、`GET /api/projects/{id}/patterns` と Insight詳細画面の「推論の過程」セクションで可視化するよう変更。§22参照）
- [x] Need Hypothesis Generation（"Hypothesis" 相当のフィールドとして明示。`rationale`（なぜこの仮説に至ったか）と `basedOnPatternIds`（元になったPattern）を Insight まで保存・表示するようにした）
- [x] Evidence Retrieval（支持 + 反証の両方。`counterSearched` 記録）— Evidence は Observation の ID 参照のみで構成されるため grounding 済み
- [x] Insight Generation（**\[変更\]** ドラフト設計は「Evidence IDを参照する」方式だったが、実装では Evidence Retrieval 段階で既に grounding 済み Observation から Evidence 行を直接構築し、Insight Generation は仮説と Observation 要約を洞察の文章に仕上げるだけの「write-up」ステップにした。新しい引用や事実を作れないためハルシネーションの余地がさらに小さい）+ Insight Dedupe（LLM 1コールでグループ化 → アプリ側でマージ）
- [x] Confidence Scoring（アプリ側計算。Evidence Strength 35% + Document Coverage 25% + Source Diversity 20% + Pattern Frequency 20%、反証件数に応じた減衰。単体テストで根拠強度・カバレッジ・多様性・反証減衰それぞれを検証）
- [x] Insight 一覧 / 詳細 UI（Observation=事実 と Interpretation=解釈 を背景色で分離。Latent Need も別枠で強調）
- [x] Evidence クリック → パネルに原文 + `<mark>` オフセットハイライト表示

**完了条件の検証**: パイプライン全体を実SQLite DB + フェイクLLMで通し、捏造引用が `observations` テーブルに一切保存されないこと・Evidenceの支持/反証が正しくリンクされること・Confidenceが0〜1に収まることをテストで確認。ブラウザE2Eで Insight 詳細 → Evidence展開 → 原文中のハイライトまで実際に目視確認済み。ここが最初のリリース = デモ可能ライン。

### Phase 4 — 入力の充実と運用性（一部完了）

- [x] テキスト貼り付け取り込み UI（Phase 1 で実装済み）
- [x] CSV インポート（固定4列 `id,source,title,content`、BOM自動除去、不正行はスキップしてエラー一覧を返す。`internal/service/csv_import.go`）
- [x] プロジェクト管理 UI（作成は実装済み。削除APIは Phase 1 から存在するが削除ボタンのUIは未追加）
- [ ] TXT インポート（CSVインポートと役割が重複するため優先度を下げた。必要になれば追加）
- [ ] トークン使用量の集計・表示（`llm.Usage` は `GenerateResponse` に既に載っており、配線のみ残っている）

### Phase 5 — 評価とリリース整備（一部完了）

- [x] 評価指標 5 種の計算（解析完了時に `internal/service.Metrics` として計算し `analyses.metrics` に保存）+ 評価画面（`#/projects/:id/evaluation`）
- [x] GitHub Actions（`.github/workflows/ci.yml`: gofmt/vet/test 両タグ + 納品ビルドへのデモデータ非混入を毎回検証。`.github/workflows/release.yml`: タグ push で `make cross-compile` → デモ/納品 × 4プラットフォームをアーカイブして Release へ添付）
- [ ] Golden Dataset 約 10 ケース定義 + `go test -tags=golden` 回帰評価（実LLM前提のため未着手）
- [ ] Markdown レポートエクスポート
- [x] README / プライバシー文言（Phase 1 から記載済み）

### Phase 6 — 拡張モジュール

- [ ] Contradiction Detection をパイプラインに追加（Contradiction Finder）
- [ ] OS Keychain 対応
- [ ] `--local-only`（外部通信ホワイトリスト制御、Ollama 等ローカル LLM 限定モード）
- [ ] Churn Analyzer / Segment Discovery / Evidence Map（共通モデル上に追加）

## 実装順の根拠

- **Grounding Check を Phase 2 に前倒し**している。Insight 生成（Phase 3)より先に quote 検証基盤を固めることで、以降のすべての LLM ステップが検証済みデータの上に乗る。ここが本プロダクトの信頼性の根幹（design-review.md P0-1）。
- **接続テストを Phase 2 に含める**。デモ先での最大の事故は LLM 接続失敗であり、フォールバック実装と同時に作るのが最も安い。
- Phase 3 完了 = MVP。Phase 4 以降は商談を重ねながら優先度を入れ替えてよい。

## リスクと対策

| リスク | 影響 | 対策 |
|--------|------|------|
| デモ先ネットワークで LLM API 不通 | デモ失敗 | 接続テスト機能 + Ollama 等ローカル LLM への切替手順を README に用意。最悪ケース用に解析済みデモプロジェクトを DB に同梱 |
| grounding 照合の正規化が日本語で不十分 | quote 破棄過多 | 全角半角・句読点・改行の正規化テストを日本語サンプルで厚く書く。破棄率を評価指標で常時監視 |
| json_schema 非対応エンドポイント | 解析不能 | 3 段フォールバック（設計 §12）。段数を接続テストで可視化 |
| 解析時間が長すぎてデモが間延び | 体験悪化 | チャンク並列化（ワーカー内で LLM 呼び出しのみ並列）、進捗メッセージを日本語で細かく流す |
| dist 未ビルドで go build 失敗 | 開発体験悪化 | Makefile で web → go の順を強制。dist に .gitkeep 相当の placeholder を commit |

## テスト方針（フェーズ横断）

- Phase 1 から CI で `go vet` + unit test を回す
- grounding / confidence / fallback は実装と同 PR でテスト必須
- パイプライン統合テストは LLM モック（固定 JSON 応答）で決定的に実行
- 実 LLM を叩く golden 評価は手動トリガ（コスト管理）
