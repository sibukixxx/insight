# 運用Runbook

ビルド・起動・設定・デプロイ・トラブルシューティングのリファレンス。プロダクトの背景は [blueprint.md](./blueprint.md)、コードの構造は [architecture.md](./architecture.md) を参照。

---

## 1. 前提条件

- Go 1.25以上（`go.mod`で指定。`go`コマンドが古い場合は自動でツールチェインをダウンロードする）
- Node.js・npm は**不要**（フロントエンドはビルドステップなしの素のHTML/CSS/JSを直接埋め込んでいる）
- CGO不要（`modernc.org/sqlite`はpure Go実装。クロスコンパイルに追加ツールチェイン不要）

## 2. ビルド・実行

```bash
# 依存取得（初回のみ）
go mod download

# デモ用ビルド（架空の請求書SaaSインタビュー20件を同梱）
make build-demo
./bin/insight-lab-demo --demo

# 納品用ビルド（デモデータはコンパイル時に一切含まれない）
make build-delivery
./bin/insight-lab

# 両方 × 4プラットフォーム（macOS arm64/amd64, Linux amd64, Windows amd64）を一括生成
make cross-compile

# go vet + go test（両ビルドタグ）
make vet
make test
```

起動すると `http://127.0.0.1:8787` でブラウザが自動で開く（`--no-browser`で抑制可）。

## 3. CLIフラグ・環境変数リファレンス

| フラグ | 環境変数 | デフォルト | 説明 |
|---|---|---|---|
| `--host` | - | `127.0.0.1` | バインドアドレス。外部公開は非推奨 |
| `--port` | - | `8787` | HTTPポート |
| `--db` | - | OS標準のアプリデータディレクトリ | SQLiteファイルパス |
| `--demo` | - | `false` | デモデータセットをロードして起動（デモビルドのみ有効。納品ビルドで指定するとエラー終了） |
| `--no-browser` | - | `false` | ブラウザ自動起動を抑制 |
| `--api-key` | `INSIGHT_LAB_API_KEY` | 空 | LLM APIキー（起動時の初期値。設定画面から上書き可能） |
| `--model` | - | 空 | LLMモデル名（例: `gpt-5`） |
| `--base-url` | - | 空 | OpenAI互換エンドポイント（例: `https://api.openai.com/v1`） |
| `--client` | `INSIGHT_LAB_CLIENT_NAME` | 空 | 納品ビルドの確認バナーに出す顧客名 |

DBの保存先（`--db`未指定時）:

| OS | パス |
|---|---|
| macOS | `~/Library/Application Support/InsightLab/insight.db` |
| Windows | `%APPDATA%\InsightLab\insight.db` |
| Linux | `${XDG_DATA_HOME:-~/.local/share}/insight-lab/insight.db` |

## 4. LLM設定

起動時にフラグ/環境変数で設定するか、起動後にブラウザの「⚙ 設定」画面から Base URL / Model / API Key を入力する（**APIキーはディスク・DBに一切保存されない**。プロセスメモリのみ）。設定画面の「接続テスト」ボタンで疎通確認ができ、`json_schema`/`json_object`のどちらのモードで応答が返ったかが表示される（対応表は`internal/llm/openai.go`参照）。

未検証事項として、**実際のOpenAI/Azure/Ollama APIキーでの動作確認はまだ行っていない**（フェイクサーバーでの検証のみ）。実キーで最初に試す際は、まず接続テストで疎通を確認してから解析を実行すること。

## 5. デプロイ

### GitHub Actions

- `.github/workflows/ci.yml`: push/PR毎に `gofmt` チェック、`go vet`、`go test`（両ビルドタグ）、両ビルドの生成、**納品ビルドにデモデータが混入していないことのgrep検証**を実行
- `.github/workflows/release.yml`: `v*` タグをpushすると `make cross-compile` で全プラットフォーム×両ビルドを生成し、アーカイブしてGitHub Releaseに添付

**既知の制約**: このリポジトリを最初にセットアップしたセッションのGitHub Appトークンには `workflows` OAuth スコープがなく、`.github/workflows/*` へのpushがGitHub側で拒否された。そのため現状これらのファイルは `.gitignore` で明示的に除外され、リポジトリには反映されていない可能性がある。**引き継ぎ後の最初の作業として**、以下のいずれかを行うこと。

1. GitHubのWeb UIから `.github/workflows/ci.yml` と `.github/workflows/release.yml` を手動で作成する（内容はこのリポジトリのローカルファイル、またはgit historyのコミット `9adca77`/`90ff7ff`以前を参照）
2. Claude Code の GitHub App に `workflows` 権限を付与してから、Claude Code セッションでpushし直す（claude.ai の Connectors 設定、または組織であれば admin-settings/claude-tag）

`.gitignore` から `.github/workflows/` の行を削除するのを忘れないこと。

### 手動デプロイ

単一バイナリなので、`make cross-compile` で生成した実行ファイルを対象マシンにコピーするだけで動く。追加のランタイム・インストーラは不要。

## 6. トラブルシューティング

| 症状 | 原因 | 対処 |
|---|---|---|
| `go build` が `internal/web/dist` 関連で失敗する | `//go:embed all:dist` は対象ディレクトリが空だと失敗する | `internal/web/dist/` に `index.html`/`app.js`/`style.css` が存在するか確認（削除しない） |
| 納品ビルドで `--demo` を指定するとエラー終了する | 意図した挙動。納品ビルドにはデモデータがコンパイルされていない | デモを試すには `make build-demo` でビルドしたバイナリを使う |
| 解析が `failed` になり `LLMが設定されていません` と出る | Base URL / Model が未設定 | 設定画面かフラグで設定する |
| 解析がプロセス再起動を挟んで永久に `running` のまま | 通常は起動時の `FailInterrupted` で自動的に `failed` に倒される | 直っていない場合は `internal/repository/sqlite/analysis.go` の `FailInterrupted` 周りにバグがある可能性。DBを直接確認（§7） |
| SSEで進捗が全く流れない | 解析がフェイク/実LLMで即座に完了し、購読前にストリームが終わっている、または途中でLLM呼び出しがエラーになっている | `GET /api/analysis/{id}` でポーリングしてstatus/errorを確認 |
| `.github/workflows/*` をpushすると `refusing to allow a GitHub App to create or update workflow` エラー | GitHub Appに`workflows`スコープがない | §5参照 |
| CSVインポートで全行スキップされる | ヘッダーが `id,source,title,content` と完全一致していない、またはBOM付きでない想定のエンコーディング | `internal/service/csv_import.go` の `headerMatches` はBOM自動除去済みなので、列名・列順を確認 |

## 7. DB操作

```bash
# DBの場所を確認（起動ログに出るほか、defaultDataDir()のロジック通り）
sqlite3 ~/.local/share/insight-lab/insight.db ".tables"

# 中身を見る
sqlite3 ~/.local/share/insight-lab/insight.db "SELECT id, name FROM projects;"

# リセットしたい場合（開発中のみ。本番相当データがある場合は先にバックアップ）
rm ~/.local/share/insight-lab/insight.db*   # .db-wal / .db-shm も含めて削除
```

WALモードで動いているため、`.db`本体だけでなく`.db-wal`/`.db-shm`も一緒に扱うこと。

## 8. 今後の変更で守るべきルール

### マイグレーションの追加

現状（v1.0未タグ付け）は `internal/repository/sqlite/migrations/001_init.sql` を直接編集してスキーマ変更している（実運用ユーザーがまだ存在しないため）。**このリポジトリがどこかにリリース・配布された後は、この方式をやめること。** 新しいスキーマ変更は `002_xxx.sql` のような新規ファイルを追加する（`internal/repository/sqlite/sqlite.go` の `migrate()` はファイル名の連番プレフィックスで適用済みかどうかを判定するので、そのまま動く）。既存の `001_init.sql` は絶対に書き換えない。

### 新しいLLMプロバイダ対応

`internal/llm.OpenAIClient` はOpenAI Chat Completions API互換のエンドポイントであれば追加コード不要で動く（Base URLを変えるだけ）。プロトコルが異なるプロバイダ（Anthropic Messages API等）を追加する場合は、`internal/llm.Client` インターフェースの新しい実装を追加し、`internal/service.DefaultLLMClientFactory` の分岐を増やす。

### 新しい分析ステップの追加

[architecture.md §8](./architecture.md#8-したいときはここを見るチートシート) のチートシートを参照。**ユーザーに見せる引用を扱うなら必ず `internal/service/grounding.go` の `Ground()` を通すこと。**
