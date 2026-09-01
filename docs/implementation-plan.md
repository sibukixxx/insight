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

**Phase 1 で未実装（Phase 2 以降）**: LLM 接続、Observation 抽出、Grounding Check、Insight 生成、SSE、CSV インポート、評価画面、GitHub Actions リリースワークフロー。CI（lint+test の自動実行）は未設定。

**完了条件**: バイナリ起動 → Try Demo → 20 件のインタビューが閲覧できる。

### Phase 2 — LLM 接続と Observation 抽出

**ゴール**: LLM 設定 → 解析実行 → Observation が抽出・検証・保存される。

- [ ] LLM Client 抽象 + OpenAI 互換実装（timeout / retry / backoff）
- [ ] Structured Output 3 段フォールバック + レスポンスバリデーション
- [ ] 設定 API・設定画面（Base URL / Model / API Key、セッションストア + HttpOnly Cookie）
- [ ] `POST /api/settings/test` 接続テスト（フォールバック段数の表示）
- [ ] Normalize + Chunk（8,000 字上限）
- [ ] Observation Extraction プロンプト + スキーマ
- [ ] **Grounding Check**（完全一致 → 正規化一致 → 破棄。単体テストを厚く）
- [ ] JobManager（queue / worker×2 / context キャンセル / analyses テーブル書き込み / 起動時 interrupted 処理）
- [ ] SSE エンドポイント（progress / error / completed）+ 解析中 UI

**完了条件**: デモデータに対して解析を実行し、検証済み quote 付き Observation が一覧表示される。

### Phase 3 — Insight 生成（プロダクトの核）

**ゴール**: Hidden Need が Evidence・反証・Confidence 付きで表示される。

- [ ] Pattern Detection ステップ
- [ ] Need Hypothesis Generation（"Hypothesis" ラベル明示）
- [ ] Evidence Retrieval（支持 + 反証の両方。counterSearched 記録）+ Grounding Check
- [ ] Insight Generation（Evidence ID のみ参照可）+ Insight Dedupe
- [ ] Confidence Scoring（アプリ側計算 + 反証減衰）
- [ ] Insight 一覧 / 詳細 UI（Observation と Interpretation の色・ラベル分離）
- [ ] Evidence クリック → 右パネルに原文 + オフセットハイライト + 「使用されている Insight」逆リンク

**完了条件**: MVP 完成条件（ドラフト §52 の 10 項目）をすべて満たす。ここが最初のリリース = デモ可能ライン。

### Phase 4 — 入力の充実と運用性

- [ ] テキスト貼り付け取り込み UI
- [ ] CSV インポート（固定 4 列、BOM 除去、UTF-8 バリデーション、エラー表示）
- [ ] TXT インポート
- [ ] プロジェクト管理 UI（作成 / 削除 = ローカルデータ完全削除）
- [ ] トークン使用量の集計・表示

### Phase 5 — 評価とリリース整備

- [ ] 評価指標 5 種の計算（解析完了時 → analyses.metrics）+ 評価画面
- [ ] Golden Dataset 約 10 ケース定義 + `go test -tags=golden` 回帰評価
- [ ] Markdown レポートエクスポート（商談後に置いていく用。PDF は後続）
- [ ] GitHub Actions リリースワークフロー（タグ push → 4 ターゲット → Releases）
- [ ] README / 配布手順 / プライバシー文言

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
