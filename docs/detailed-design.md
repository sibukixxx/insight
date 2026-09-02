# Insight Lab / Hidden Needs Finder 詳細設計（確定版 v1）

本書はドラフト設計を検証（[design-review.md](./design-review.md)）した結果を反映した、実装着手可能な確定版設計である。ドラフトから変更した箇所には **[変更]** を付す。

---

## 1. プロダクト概要

**Insight Lab – Hidden Needs Finder** は、顧客インタビュー・問い合わせ履歴・レビュー・商談ログなどのテキストから、ユーザーが明示していない潜在ニーズ・矛盾・JTBD・改善仮説を抽出するローカル実行型 AI 分析ツール。

核となる原則:

> Insight → Evidence → Alternative Interpretation → Confidence を必ずセットで表示する。
> Evidence の quote は必ず一次データ上に実在することをアプリが検証する。**[変更: grounding 検証を原則に昇格]**

第一目的は SaaS 化ではなく、商談の場で `./insight-lab --demo` を起動して「顧客データからインサイトを抽出するシステムを設計・品質管理できる」ことを短時間で証明すること。

### 1.1 ビルド分離（デモ用 / 納品用）**[追加]**

同じソースから **2 種類のバイナリ**をビルドできることを要件とする。

| ビルド種別 | 用途 | デモデータ |
|---|---|---|
| デモビルド（`make build-demo`） | 商談・営業デモ | 架空インタビュー約20件を埋め込み。`--demo` 起動可 |
| 納品ビルド（`make build` / `build-delivery`） | 顧客への納品・実データでの利用 | **一切埋め込まない**（ランタイムのフラグ制御ではなく、コンパイル時に対象ファイルへの参照自体が存在しない） |

これは Go の build tag（`//go:build demo`）で実現する。`internal/sampledata` パッケージを、デモデータを `//go:embed` する実装（`demo` タグ）と、何も埋め込まない空実装（`!demo` タグ = デフォルト）に分割し、納品ビルドの成果物にはサンプルインタビューのテキストがバイナリレベルで含まれない。顧客の秘密保持契約下で納品するビルドに、営業用のサンプルデータ（＝自社の別テキスト）が混入するリスクを構造的に排除する。

納品ビルドで `--demo` を指定した場合、またはブラウザから「デモを試す」を押した場合は、起動時 / API 応答時に明示的なエラーを返す（サイレントに無視しない）。任意で `--client "<顧客名>"` を渡すと、UI に "Confidential — <顧客名> 様向け納品版" のバナーを表示できる。

## 2. 方針（変更なし）

| 項目 | 方針 |
|------|------|
| 配布 | Go 単一バイナリ |
| 起動 | コマンド 1 つ / ダブルクリック |
| UI | ブラウザ（自動起動） |
| DB | SQLite（modernc.org/sqlite、CGO 不要） |
| LLM | OpenAI 互換 API（差し替え可能） |
| データ保存 | ローカルのみ。外部送信は LLM API への解析テキストのみ |
| バインド | 127.0.0.1:8787 固定デフォルト。外部公開しない |
| UI 言語 | **日本語**（デモ相手が日本企業のため）**[変更: 明文化]** |

## 3. 技術スタック

- **Backend**: Go 1.25 / net/http + `github.com/go-chi/chi/v5`（ルーティングのみ）
- **DB**: `modernc.org/sqlite`。接続時に必ず `PRAGMA journal_mode=WAL; busy_timeout=5000; foreign_keys=ON` **[変更]**
- **Frontend**: TypeScript + Vite + Preact + Tailwind CSS v4（`@tailwindcss/vite`）
- **埋め込み**: `//go:embed all:dist` **[変更: `all:` プレフィックス必須。`dist/*` は `_`/`.` 始まりファイルを取り込まない]**
- **LLM**: 自前の薄い抽象化（§12）。SDK 依存なし

## 4. ディレクトリ構成

ドラフト §6 の構成を採用。HTTP から永続化を直接呼ばず、ユースケース層を境界にする。
依存方向は `http/handler → usecase → repository(interface) ← repository/sqlite` とし、
具体実装の組み立ては `internal/app` だけが担当する。

```
internal/
├── domain/                 # 依存を持たないエンティティ・値
├── usecase/                # 入力検証、ID採番、複数repositoryのオーケストレーション
├── service/
│   ├── grounding.go        # [追加] quote照合・オフセット確定
│   └── dedupe.go           # [追加] Insight重複統合
├── repository/             # 永続化ポート（interface）
│   └── sqlite/             # SQLiteによるポート実装
├── llm/
│   └── fallback.go         # [追加] structured output 3段フォールバック
└── http/handler/
    └── events.go           # [追加] SSEハンドラ
migrations/
└── 001_init.sql            # embed して起動時適用
```

マイグレーションは外部ツールを使わず、`schema_migrations(version)` テーブル + embed した SQL を起動時に順次適用する方式。

## 5. ドメインモデル

### Project / Document（変更なし）

```go
type Project struct {
    ID        string
    Name      string
    CreatedAt time.Time
}

type Document struct {
    ID        string
    ProjectID string
    Source    SourceType // interview | review | support | sales | survey
    Title     string
    Content   string
    Metadata  map[string]string // 予約キー: participant_id（将来のカバレッジ集計用）
    CreatedAt time.Time
}
```

### Observation **[変更: ドメイン型として明文化]**

```go
type Observation struct {
    ID          string
    DocumentID  string
    Quote       string  // 一次データからの引用（grounding検証済み）
    StartOffset int     // アプリ側照合で確定した値
    EndOffset   int
    Behavior    string  // 観察可能な行動の記述（LLM生成、推論禁止）
    Topic       string
    CreatedAt   time.Time
}
```

### Evidence **[変更: observation_id 追加、オフセットはアプリ確定]**

```go
type Evidence struct {
    ID             string
    InsightID      string
    DocumentID     string
    ObservationID  *string // どのObservation経由か（nullable）
    Quote          string  // grounding検証済み。LLM生成文は不可
    StartOffset    int
    EndOffset      int
    Type           EvidenceType // support | counter | neutral
    RelevanceScore float64
}
```

### Insight（変更なし。Observation=事実 / Interpretation=推論の分離が最重要）

```go
type Insight struct {
    ID, ProjectID, Title           string
    Observation                    string  // 一次データから直接確認できる事実の要約
    StatedNeed, LatentNeed, JTBD   string
    ProductOpportunity             string
    Interpretation                 string  // AIによる推論（UIで明確にラベル分け）
    AlternativeInterpretation      string  // 必須。別解釈
    Confidence                     float64 // アプリ側計算（§10）
    Evidence                       []Evidence
    CreatedAt                      time.Time
}
```

### Analysis **[追加]**

```go
type Analysis struct {
    ID          string
    ProjectID   string
    Status      AnalysisStatus // queued | running | completed | failed
    CurrentStep string
    Progress    int    // 0-100
    Error       string
    Metrics     string // 完了時に評価指標(§15)をJSONで保存
    StartedAt, FinishedAt *time.Time
    CreatedAt   time.Time
}
```

## 6. 分析パイプライン

一発生成はしない。**MVP パイプライン**（Contradiction Detection は Phase 6 で追加。ドラフト §10 の全段構成は最終形）: **[変更: MVP版と最終版を分離]**

```
Raw Documents
  ↓ Normalize（改行・空白・エンコーディング正規化）
  ↓ Chunk（1チャンク最大8,000字。チャンク毎に以下を実行）
  ↓ Observation Extraction（LLM。推論禁止・引用必須）
  ↓ Grounding Check（アプリ。quoteを原文照合、失敗quoteは破棄）★
  ↓ Pattern Detection（LLM。反復行動・反復回避・反復不安の検出）
  ↓ Need Hypothesis Generation（LLM。以後 "Hypothesis" と明示）
  ↓ Evidence Retrieval（LLM。支持と反証の両方を必ず検索）★
  ↓ Grounding Check（再度。Evidence quoteの実在検証）★
  ↓ Insight Generation（LLM。Evidence IDのみ参照可）
  ↓ Insight Dedupe（LLM 1コールで類似Insight統合）[変更: 追加]
  ↓ Confidence Scoring（アプリ側計算。LLMに数値を出させない）★
```

★ = 本プロダクトの差別化ポイント。LLM の出力を信頼せず、アプリ側で検証・計算する箇所。

### 6.1 Grounding Check（quote 実在性検証）**[追加: P0-1 対応]**

LLM にオフセットを要求しない。照合はアプリ側で行う:

1. **完全一致**: `strings.Index(content, quote)` → オフセット確定
2. **正規化一致**: 双方を正規化（空白/改行除去、全角半角統一、句読点正規化）した上で照合し、元テキスト上のオフセットへ逆写像
3. **失敗**: その quote を破棄し、破棄件数を評価指標（Unsupported Claim Rate の分子）として記録

これにより「AI が存在しない顧客発言を引用する」ことを構造的に防ぐ。UI に表示される quote はすべて DB 上の検証済み Evidence から取得する（LLM 出力を直接表示しない）。

### 6.2 Observation Extraction

プロンプトで推論を禁止（`Do not infer motivation. Only extract directly observable behavior with exact quotes.`）。出力は JSON Schema 制約（§13）。

### 6.3 Evidence Retrieval / Counter Evidence

仮説ごとに、プロジェクト内の全 Observation（検証済み quote 付き）を渡し、**支持と反証の両方**を選ばせる。反証ゼロの場合も `counterSearched: true` を記録する（Counter Evidence Coverage の分母管理）。MVP では埋め込みベクトル検索は使わず、Observation 集合を直接 LLM に渡す（20〜30 ドキュメント規模なら十分収まる）。規模拡大時に SQLite FTS5 → 埋め込み検索へ段階拡張する。

## 7. Confidence Score（アプリ側計算）

LLM に数値を出させない。**[変更: EvidenceCoverage を Document 単位に再定義（P0-2 対応）]**

```
Confidence = EvidenceStrength × 0.35   // 支持Evidenceのrelevance平均
           + EvidenceCoverage × 0.25   // 支持Evidenceを持つDocument数 / プロジェクト内Document数
           + SourceDiversity  × 0.20   // 支持EvidenceのSourceType種類数 / 5（上限1.0）
           + PatternFrequency × 0.20   // 該当パターンの出現Document数を正規化（例: min(n/5, 1.0)）
```

反証 Evidence が存在する場合は係数減衰を掛ける（例: `× (1 - 0.1 × min(counterCount, 3))`）。重みは初期値であり、Golden Dataset（§16）での回帰評価で調整する。

## 8. DB スキーマ（確定版 DDL）

**[変更: FK・インデックス・CHECK・analyses・schema_migrations 追加、evidence.observation_id 追加]**

```sql
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at DATETIME NOT NULL
);

CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE TABLE documents (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source TEXT NOT NULL CHECK (source IN ('interview','review','support','sales','survey')),
    title TEXT,
    content TEXT NOT NULL,
    metadata TEXT,               -- JSON
    created_at DATETIME NOT NULL
);
CREATE INDEX idx_documents_project ON documents(project_id);

CREATE TABLE observations (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    quote TEXT NOT NULL,
    start_offset INTEGER NOT NULL,
    end_offset INTEGER NOT NULL,
    behavior TEXT NOT NULL,
    topic TEXT,
    created_at DATETIME NOT NULL
);
CREATE INDEX idx_observations_document ON observations(document_id);

CREATE TABLE analyses (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('queued','running','completed','failed')),
    current_step TEXT,
    progress INTEGER NOT NULL DEFAULT 0,
    error TEXT,
    metrics TEXT,                -- JSON: 評価指標(§15)
    started_at DATETIME,
    finished_at DATETIME,
    created_at DATETIME NOT NULL
);
CREATE INDEX idx_analyses_project ON analyses(project_id);

CREATE TABLE insights (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    analysis_id TEXT REFERENCES analyses(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    observation TEXT,
    stated_need TEXT,
    latent_need TEXT,
    jtbd TEXT,
    interpretation TEXT,
    alternative_interpretation TEXT,
    product_opportunity TEXT,
    confidence REAL,
    created_at DATETIME NOT NULL
);
CREATE INDEX idx_insights_project ON insights(project_id);

CREATE TABLE evidence (
    id TEXT PRIMARY KEY,
    insight_id TEXT NOT NULL REFERENCES insights(id) ON DELETE CASCADE,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    observation_id TEXT REFERENCES observations(id) ON DELETE SET NULL,
    quote TEXT NOT NULL,
    evidence_type TEXT NOT NULL CHECK (evidence_type IN ('support','counter','neutral')),
    relevance_score REAL,
    start_offset INTEGER NOT NULL,
    end_offset INTEGER NOT NULL
);
CREATE INDEX idx_evidence_insight ON evidence(insight_id);
```

起動時: `running`/`queued` のまま残っている analyses を `failed`（error='interrupted'）に更新する。**[変更: P0-3 対応]**

## 9. HTTP API

```
GET  /api/health
GET  /api/projects
POST /api/projects
GET  /api/projects/{projectID}
DELETE /api/projects/{projectID}          # ローカルデータ完全削除（§17 セキュリティ）
POST /api/projects/{projectID}/documents  # paste / CSV / TXT
GET  /api/projects/{projectID}/documents
GET  /api/documents/{documentID}          # Evidenceクリック時の原文表示用 [追加]
POST /api/projects/{projectID}/analysis   # → {"analysisId","status":"queued"}
GET  /api/analysis/{analysisID}           # 状態取得（SSE切断時のフォールバック）[追加]
GET  /api/analysis/{analysisID}/events    # SSE
GET  /api/projects/{projectID}/insights
GET  /api/insights/{insightID}
GET  /api/insights/{insightID}/evidence
GET  /api/settings
PUT  /api/settings
POST /api/settings/test                   # LLM接続テスト（フォールバック段数も返す）[追加]
```

- 解析は非同期。HTTP リクエストを長時間保持しない。
- すべてのハンドラで `Host`/`Origin` を検証（localhost 以外からのアクセス拒否）。**[変更: P1-3]**

### SSE イベント **[変更: error イベント追加]**

```
event: progress   data: {"step":"extracting_observations","progress":20,"message":"インタビューを読んでいます..."}
event: progress   data: {"step":"searching_counter_evidence","progress":80,"message":"反証を探しています..."}
event: error      data: {"step":"generating_insights","message":"LLM APIに接続できません（401）"}
event: completed  data: {"progress":100,"insightCount":5}
```

## 10. Job 管理

外部キューなし。メモリ内 JobManager + goroutine ワーカー（2 本）。**[変更: 排他とキャンセルを明記]**

```go
type JobManager struct {
    mu    sync.Mutex               // jobs マップの排他
    jobs  map[string]*Job          // analysisID → Job（SSE購読チャネル含む）
    queue chan *Job
}
```

- 各 Job は `context.Context` を持ち、プロジェクト削除・シャットダウン時にキャンセル可能。
- 状態遷移（queued→running→completed/failed）は毎回 `analyses` テーブルにも書く。SSE はメモリ内チャネル配信、再接続・リロード時は `GET /api/analysis/{id}` で現在状態を取得。

## 11. 設定と API Key

優先順位（変更なし）: 環境変数 `INSIGHT_LAB_API_KEY` → 起動フラグ `--api-key` → ブラウザから設定。

**[変更: P1-3 対応]** ブラウザから設定された Key は:
- サーバ側メモリのセッションストアにのみ保持（DB・ファイル保存しない）
- HttpOnly Cookie のセッション ID で紐付け。localStorage には置かない
- `GET /api/settings` は Key をマスクして返す（`sk-...****`）

OS Keychain 対応は Phase 6 以降。

## 12. LLM クライアント抽象

```go
type Client interface {
    Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
}

type GenerateRequest struct {
    SystemPrompt string
    Messages     []Message
    Schema       *jsonschema.Schema // 必須。自然言語レスポンス禁止
    Temperature  float64
    MaxTokens    int
}
```

差し替え対象: OpenAI / Azure OpenAI / OpenRouter / Ollama / LM Studio（すべて Base URL + Model の設定のみで切替）。

### Structured Output 3 段フォールバック **[追加: P0-4 対応]**

```
1. response_format: {type:"json_schema", strict:true}
2. 拒否(400等) → response_format: {type:"json_object"} + スキーマをプロンプト末尾に埋め込み
3. 常時: アプリ側で JSON パース + スキーマバリデーション。
   失敗時はバリデーションエラーを添えて最大2回リトライ
```

どの段で動いたかは接続テスト（`POST /api/settings/test`）で可視化する。

### 堅牢化 **[追加: P1-5]**

- ステップ毎タイムアウト 120s
- 429/5xx: 指数バックオフで最大 3 回再試行
- 解析全体のトークン使用量を集計し、完了時に表示

## 13. プロンプト設計

System Prompt 基本方針（ドラフト §36 を踏襲）:

```
You are a customer research analyst.
Separate: 1. Observable facts  2. Interpretation  3. Hypothesis
Never present a hypothesis as fact.
Every insight must reference evidence IDs. Quotes must be verbatim from source.
Search for counter evidence.
Prefer surprising but well-supported insights over generic observations.
```

各ステップの出力は必ず JSON Schema で制約。Insight 生成では **Evidence ID のみ参照可**とし、quote 本文の生成を許可しない。表示時は常に DB の検証済み Evidence から原文を取得する。

## 14. UI 設計

ドラフト §26–28 を踏襲（トップに Try Demo を最も目立たせる / Analysis 画面 / Evidence クリックで右パネルに原文 + ハイライト）。追加事項:

- Observation（事実）と Interpretation（推論）はラベルと色を分け、混同させない
- 右パネルでは `start_offset/end_offset` により**原文中の該当箇所をハイライト**（grounding 検証の成果が最も伝わる箇所）
- 解析中は SSE の step message を日本語で表示（「反証を探しています...」）
- フッターに常時表示: 「アップロードされたデータはローカルに保存されます。解析に必要なテキストのみ設定された AI プロバイダへ送信されます」

## 15. 評価機能

解析完了時に計算し `analyses.metrics` に保存、評価画面で表示:

| 指標 | 定義 |
|------|------|
| Evidence Coverage | 支持 Evidence が 1 件以上ある Insight の割合 |
| Unsupported Claim Rate | grounding 照合に失敗し破棄された quote の割合 **[変更: 定義を grounding と接続]** |
| Counter Evidence Coverage | 反証検索を実施した Insight の割合 |
| Insight Duplication | dedupe ステップで統合された Insight の割合 |
| Avg Evidence / Insight | Insight あたり平均 Evidence 数 |

## 16. Golden Dataset

サンプルインタビュー（請求書作成 SaaS、約 20 件。表面は「操作が面倒」、深層は「誤った請求書を顧客へ送る恐怖」）に対し、期待 Insight・期待 Evidence・許容代替解釈を約 10 ケース手作業定義。`go test -tags=golden` で回帰評価を実行し、モデル・プロンプト変更時の劣化を検出する。

## 17. CLI / 配布 / ビルド

- フラグ: `--port --host --db --demo --no-browser --api-key --model --base-url --client`（`--client` は §1.1 の納品ビルド用バナー表示。環境変数 `INSIGHT_LAB_CLIENT_NAME` でも指定可）
- `--demo` は冪等（既存デモプロジェクトがあれば再利用）**[変更: P1-8]**。納品ビルドで指定した場合はエラーで起動を中止する（§1.1）
- ブラウザ自動起動: `open`(macOS) / `xdg-open`(Linux) / `rundll32 url.dll,FileProtocolHandler`(Windows)
- データ保存先: ドラフト §32 のまま（OS 標準のアプリデータディレクトリ）
- CSV: `id,source,title,content` 固定。UTF-8 のみ、BOM 自動除去 **[変更: P1-7]**
- ビルド: `make build-delivery`（納品用、デフォルト）/ `make build-demo`（デモ用、`-tags demo`）。`make cross-compile` で両方 × 4 ターゲット（darwin-arm64/amd64, linux-amd64, windows-amd64）を生成。GitHub Actions でタグ push 時に Releases へ **[変更: §1.1 のビルド分離を反映]**
- 起動速度目標: Cold Start < 1s、ブラウザ表示 < 2s
- フロントエンドは Phase 1 では素の HTML/CSS/JS（ビルドステップなし）を `internal/web/dist` に直接コミットして埋め込む。Vite/Preact への移行は UI の複雑化が必要になった時点で行う（バックエンドの API 契約に影響しない）**[変更: 実装簡略化]**

## 18. テスト構成

- unit: grounding 照合（正規化含む）/ confidence 計算 / CSV パース / LLM レスポンスバリデーション / fallback 遷移
- integration: repository(SQLite 実ファイル) / API ハンドラ / 解析パイプライン（LLM はモック）
- golden: §16 の回帰評価（実 LLM、CI ではオプショナル）

## 19. 将来モジュール（変更なし）

Contradiction Finder / Churn Analyzer / Segment Discovery / Evidence Map 等は、共通の Document / Observation / Evidence / Insight モデル上に同一バイナリ内モジュールとして追加する。別アプリにはしない。加えて **Markdown/PDF レポートエクスポート**を Phase 5 候補に追加（商談後に解析結果を置いていける）。

## 20. Phase 2/3 実装時の設計変更点 **[追加]**

実装（`internal/service`, `internal/llm`, `internal/http`）を進める過程で、確定版設計から以下を変更した。理由とあわせて記録する。

1. **設定ストアを HttpOnly Cookie セッションではなくプロセス内シングルトンに簡略化**（§11 関連）。本ツールはローカル1操作者が使うことを前提としており、ブラウザタブをまたいだ設定共有はむしろ自然な挙動である。ディスク/DBに一切保存しないという要件（confidentiality上の要件）は`internal/service/settings.go`の`SettingsStore`（メモリ内mutex保護のstruct）でそのまま満たしている。マルチユーザー同時アクセスが必要になった場合のみCookieセッション化を検討する。
2. **Insight Generation を「Evidence ID を参照する」方式から「grounding 済み Observation から直接 Evidence 行を構築する」方式に変更**（§6.3, §13 関連）。Evidence Retrieval ステップが返すのは Observation ID のリストのみで、これは既に Grounding Check を通過した quote に紐づいている。Insight Generation（write-up）ステップは新しい引用や事実を一切生成せず、確定済みの仮説と Observation 要約を洞察文章にまとめるだけの役割に限定した。LLM が Evidence ID を捏造するリスクそのものを構造的に排除できるため、当初設計より安全性が高い。
3. **Evidence の relevanceScore は LLM に自己申告させず、Evidence Retrieval が返した Observation ID の並び順から機械的に算出**（`1.0 - 0.05×順位`、下限0.5）。Confidence を LLM に自己申告させない方針（design-review.md P0-2）と一貫させるため。
4. **Evidence Retrieval は毎回プロジェクト内の全 Observation を LLM に渡す**（FTS5/埋め込み検索は未実装）。デモ規模（数十件のObservation）では問題にならないが、大規模プロジェクトではトークン予算を圧迫する。Phase 6 でのスケール課題として明記。
5. **フロントエンドは Vite/Preact ではなく素の HTML/CSS/JS のまま**（§17 に既出の変更を Phase 2/3 でも継続）。SSE・Insight詳細・Evidence原文ハイライト・評価画面・設定画面・CSVインポートUIまで含めて、ビルドステップなしで実装できている。

## 21. 用途の追加: 自分用の収益機会発見モード **[追加]**

Hidden Needs Finder のパイプライン（Observation→Grounding→Pattern→Hypothesis→Evidence→Confidence）はデータソースに依存しない。当初設計は「顧客企業への納品・営業デモ」を主用途としていたが、同じエンジンをユーザー自身の収益機会発見にも転用できるよう以下を追加した。

- `domain.SourceType` に `job_posting`（案件・募集文）と `social_post`（SNS投稿）を追加。他のCoWorkスキル（`market-demand-research` 等）の出力をそのまま貼り付けて解析対象にできる。
- Insight に **`MonetizationAngle`**（収益化の切り口）フィールドを追加。`ProductOpportunity` が「対象企業への改善提案」であるのに対し、`MonetizationAngle` は「このニーズをユーザー自身が新しい商品・サービスとして提供するなら何ができるか（誰が対価を払うか、note/テンプレート/SaaS/コンサル等どの形式が向いているか）」を出力する。空文字列を許容し、該当する切り口がない場合は表示しない。
- DB: `insights.monetization_angle TEXT`（nullable）を追加。まだリリースされていない前提で `001_init.sql` を直接編集した（マイグレーション追加はしていない）。

この変更により、Insight Lab は「他社に売り込むデモツール」と「自分でニーズを見つけて自分で商品化するための分析ツール」の両方として使える。どちらの用途で使うかはデータソース（顧客インタビュー vs 案件サイト/SNS）と読み手（クライアント向け vs 自分向け）が変わるだけで、エンジン自体は共通である。

## 22. 推論過程の可視化（Pattern の永続化） **[追加]**

「インサイトはマーケターが多数の声を突き合わせて『ここが違う』と気づく、本質的にブラックボックスな作業だ。それを可視化したい」という指摘を受けて対応した。

### 問題

実装当初、Pattern Detection（複数 Observation にまたがる繰り返し行動の検出）と Need Hypothesis Generation の `rationale`（なぜその仮説に至ったかという推論）は、パイプライン内部で計算されるだけで **DBに一切保存されず、最終的な Insight の文言にしか反映されていなかった**。つまり「観察をパターンにまとめ、そこから仮説を立てる」というマーケターの中間的な思考過程そのものが不可視化されていた。これは design 上の見落としであり、今回の指摘で顕在化した。

### 対応

- **`Pattern` をドメインモデル・DBテーブルとして新設**（`patterns`, `pattern_observations` テーブル）。どの Observation 群から見つかった繰り返しかを保持する。
- **`Insight.Rationale`** フィールドを追加し、Hypothesis Generation が生成する `rationale`（今まで捨てられていた）を保存する。
- **Hypothesis Generation の出力に `basedOnPatternIds` を追加**。LLM に「どの Pattern を根拠にこの仮説を立てたか」を明示的に引用させる（`insight_patterns` テーブルで多対多リンク）。存在しない Pattern ID の参照は保存時に無視する（grounding と同じ「存在しないものを参照させない」原則を Pattern レベルにも拡張）。
- **Pattern も grounding の対象**: LLM が返す `observationIds` のうち実在しない ID は `buildPatterns`（`internal/service/pipeline.go`）で除外し、支持する Observation が1件もない「Pattern」は保存しない。
- **API**: `GET /api/projects/{id}/patterns`（プロジェクト全体のPattern一覧）、`GET /api/insights/{id}` のレスポンスに `patterns` と `rationale` を追加。
- **UI**: Insight詳細画面の先頭に「推論の過程」セクションを新設。Pattern（タイトル・説明・元になった Observation の引用、原文クリックで確認可能）→ Rationale（なぜこの仮説に至ったか）→ 従来の Insight 本体（Observation要約・StatedNeed・LatentNeed・JTBD・Interpretation等）→ Evidence/Counter Evidence、という順で、一次データから最終的な洞察までの連鎖を上から下にたどれる。プロジェクト単位で全 Pattern を見る `#/projects/:id/patterns` ページも追加（Insightに至らなかった Pattern も含めて確認できる）。

この一連の変更により、「Observation（一次データの引用）→ Pattern（繰り返しへの気づき）→ Rationale（推論の飛躍）→ Hypothesis（LatentNeed/JTBD）→ Evidence（検証）→ Insight（結論）」という、マーケターが頭の中で行う一連の推論が、すべて DB に記録され、UI 上でたどれるようになった。
