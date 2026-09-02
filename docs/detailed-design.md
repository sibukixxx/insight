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

### Project / Document **[変更: 入力契約の拡張（§24）]**

```go
type Project struct {
    ID            string
    Name          string
    IntakeProfile IntakeProfile // 取り込みの記憶（話者→役割、マスク辞書、列マッピング）
    CreatedAt     time.Time
}

type Document struct {
    ID         string
    ProjectID  string
    Source     SourceType // interview | review | support | sales | survey | job_posting | social_post
    Provenance Provenance // firsthand（本人の発言）| secondhand（第三者のメモ。sales の既定）
    Title      string
    Content    string     // 分析対象のテキスト。マスク後。原文照合・ハイライト・LLM送信はすべてこれに対して行う
    RawContent string     // マスク前の原文。マスクで変化があった場合のみ保持。LLM には送らない
    Spans      []Span     // 話者区間。空 = 全文が回答者の発言
    Metadata   map[string]string // 予約キー: participant_id, role, company_size, segment, plan, volume, date, rating
    CreatedAt  time.Time
}

type Span struct {
    Start, End int         // ルーンオフセット（End は排他）
    Speaker    string      // 元テキストのラベル（"面接官", "Q", "田中"）
    Role       SpeakerRole // customer | interviewer | agent | other。引用できるのは customer のみ
}

type IntakeProfile struct {
    SpeakerRoles  map[string]SpeakerRole // ラベル → 役割
    MaskTerms     []string               // プロジェクト固有のマスク語（担当者名・社名）
    ColumnMapping *ColumnMapping         // 前回使った列マッピング
}
```

`Document.CustomerSpans()` が引用可能領域を返し、`Document.Situation()` が予約メタデータを「経理担当 / 30名 / 月150件発行」のような1行に整形する（§24.5）。

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

### Pattern **[追加: §22, §23]**

```go
type Pattern struct {
    ID, ProjectID, AnalysisID string
    Kind           PatternKind   // repetition（繰り返し）| deviation（予想とのズレ＝欲望の痕跡）
    Title, Description string    // deviation では Description = 実際の行動
    Expectation    string        // deviation のみ: 常識的な予想
    DeviationType  DeviationType // contradiction | excess_effort | excess_payment | persistence | absence | other
    ObservationIDs []string      // 実在する Observation のみ（保存時に検証）
}
```

### Insight **[変更: アブダクション項目と品質フラグを追加（§23）]**

```go
type Insight struct {
    ID, ProjectID, Title           string
    Observation                    string  // 一次データから直接確認できる事実の要約
    StatedNeed, LatentNeed, JTBD   string
    Expectation                    string  // ① 常識的な予想
    SurprisingFact                 string  // ② 予想を裏切った事実（痕跡）
    Rationale                      string  // ④ LatentNeed が真なら SurprisingFact が当然になる説明
    ProductOpportunity             string
    MonetizationAngle              string
    Interpretation                 string  // AIによる推論（UIで明確にラベル分け）
    AlternativeInterpretation      string  // 必須。別解釈
    Confidence                     float64 // アプリ側計算（§7）
    QualityFlags                   []QualityFlag // アプリ側判定（§23.3）: stated_need_echo | generic_term | no_trace | abduction_incomplete | secondhand_only
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
  ↓ Trace Detection（LLM。常識的予想とのズレ＝欲望の痕跡の検出。§23）[変更: 追加]
  ↓ Pattern Detection（LLM。反復行動・反復回避・反復不安の検出）
  ↓ Need Hypothesis Generation（LLM。アブダクション形式。以後 "Hypothesis" と明示）
  ↓ Evidence Retrieval（LLM。支持と反証の両方を必ず検索）★
  ↓ Grounding Check（再度。Evidence quoteの実在検証）★
  ↓ Insight Generation（LLM。Evidence IDのみ参照可）
  ↓ Insight Dedupe（LLM 1コールで類似Insight統合）[変更: 追加]
  ↓ Confidence Scoring（アプリ側計算。LLMに数値を出させない）★
  ↓ Quality Gate（アプリ側判定。顕在ニーズの言い換え / 抽象語 / 痕跡なし を警告。§23）★ [変更: 追加]
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
POST /api/projects/{projectID}/documents  # 貼り付け。provenance / spans / speakerRoles / detectSpeakers / skipMask を受け付ける
GET  /api/projects/{projectID}/documents
POST /api/projects/{projectID}/documents/import          # CSV/TSV。mapping フィールド（JSON ColumnMapping）で任意の列構成 [変更: §24]
POST /api/projects/{projectID}/documents/import/preview  # 列・サンプル・提案マッピング・前回のマッピング [追加: §24]
POST /api/projects/{projectID}/intake/preview            # 話者分離・マスク結果のプレビュー [追加: §24]
GET  /api/projects/{projectID}/intake-profile            # 取り込みプロファイル [追加: §24]
PUT  /api/projects/{projectID}/intake-profile
GET  /api/projects/{projectID}/patterns                  # 痕跡・繰り返しパターン一覧 [追加: §22]
GET  /api/projects/{projectID}/evaluation                # 評価指標 [追加: §15]
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
| Quotes Outside Customer Speech | 原文には存在するが質問者・担当者の区間にあったため破棄した引用の件数（取り込み品質の指標）**[追加: §24]** |
| Trace Count / Trace-backed Insight Rate | 痕跡の件数 / 痕跡を根拠に持つ Insight の割合 **[追加: §23]** |
| Quality Flagged Insight Rate / Quality Flag Counts | 品質警告付き Insight の割合 / フラグ別件数 **[追加: §23]** |

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

## 23. インサイト検出法の整理: 予想 → ズレ → アブダクション **[追加]**

松本健太郎「インサイトの見つけ方」（日経COMEMO, 2026-09-02）を参考に、「LLM にインサイトを出させる」のではなく、熟練リサーチャーの作法そのものをパイプラインとアプリ側の検証に落とし込んだ。

### 23.1 定義と、これまでの実装の問題

> インサイトとは「人を動かす無自覚な欲求」。「コスパ」「安心」は無自覚ではなく（顕在ニーズ）、「自分らしさ」「承認欲求」は抽象的すぎて人を動かさない。どちらもインサイトではない。

§6 のパイプラインは Observation → **繰り返し**（Pattern）→ Hypothesis という流れしか持っていなかった。繰り返しは「多くの人が言っている / やっていること」であり、そこから立てた仮説は本人が既に自覚しているニーズの言い換えになりやすい。記事の指摘する「粗悪品」は、まさにこの経路から生まれる。また、生成された LatentNeed がインサイトの定義を満たしているかを判定する仕組みがなく、モデルの出力をそのまま表示していた。

### 23.2 方法論の3ステップと実装の対応

| 記事の方法 | 実装 |
|---|---|
| ① 常識に照らして「人間はこう動くはずだ」と予想する | **Trace Detection** ステップ（`traceDetectionPrompt`）。各 Observation 群に対し、まず `expectation`（常識的予想）を立てさせる |
| ② 予想と異なる行動を、見えない力が残した痕跡として捉える | 同ステップで `actualBehavior` と `deviationType`（言行不一致 / 急いでいるのに手間をかける / 予定より多く払う / 不満なのに使い続ける / 起きるはずの行動がない＝「吠えなかった犬」/ その他）を出力。`Pattern` として `kind = deviation` で永続化し、`expectation` と実際の行動を並べて UI に表示する |
| ③ アブダクションで、痕跡を説明する無自覚な欲求を仮説立てる | **Hypothesis Generation** の出力を「驚くべき事実 C（`surprisingFact`）」「常識的予想（`expectation`）」「仮説 H（`latentNeed`）」「H が真なら C は当然になる説明（`rationale`）」の4項目に構造化。`Insight` に `Expectation` / `SurprisingFact` を追加し、詳細画面の「推論の過程」を ①予想 → ②ズレ → ③仮説 → ④説明 の順で表示する |
| 「良いアブダクションは事実を加えず、別の意味を与える」 | `surprisingFact` に新しい事実を書くことを禁止し、Observation にある事実のみを使わせる。Pattern 参照は grounding と同じく「存在しない ID は保存時に捨てる」 |
| インサイトを便益に結びつけないと「どれでも良い」になる | Writeup の `productOpportunity` に「なぜこの製品のこの便益でなければならないか」を要求 |

Trace Detection は Pattern Detection より前に実行する。痕跡に根ざした仮説を優先させるためであり、Hypothesis Generation には両方の Pattern（`kind` 付き）を渡し、deviation を優先して根拠にするよう指示する。

### 23.3 Quality Gate（アプリ側の粗悪品判定）

「単なる AI 利用」に終わらせないため、インサイトの定義を満たしているかの判定は LLM の自己申告ではなく、`internal/service/quality.go` の決定的なチェックで行う。結果は `Insight.QualityFlags`（`insights.quality_flags` JSON）に保存し、一覧・詳細・評価画面に警告として表示する。**警告であって却下ではない**。最終判断はリサーチャーが行う。

| フラグ | 判定 | 根拠 |
|---|---|---|
| `stated_need_echo` | LatentNeed と StatedNeed を grounding と同じ正規化（空白・句読点除去・全半角統一）にかけ、包含関係または文字 bigram の Jaccard 係数 ≥ 0.5 | 本人が口にしているニーズは無自覚ではない |
| `generic_term` | LatentNeed に「コスパ / 安心 / 便利 / 効率 / 時短 / 手軽 / お得 / 承認欲求 / 自己実現 / 自分らしさ / 帰属意識 / 満足感」等の語を含む（該当語を detail に記録） | 顕在ニーズ語・抽象語はインサイトではない |
| `no_trace` | 仮説が引用した Pattern に `deviation` が1つもない | 繰り返しだけから導かれた仮説は当たり前の観察に留まりやすい |
| `abduction_incomplete` | `expectation` または `surprisingFact` が空 | 予想 → ズレ → 仮説 の連鎖を読者が検証できない |

評価指標（§15）に **Trace Count**（痕跡の件数）、**Trace-backed Insight Rate**（痕跡を根拠に持つ Insight の割合）、**Quality Flagged Insight Rate**（警告付き Insight の割合、低いほど良い）と、フラグ別件数 `qualityFlagCounts` を追加した。モデルやプロンプトを変えた際に「粗悪品率」が上がっていないかを、このダッシュボードで監視できる。

### 23.4 Confidence との関係

Confidence（§7）は「その仮説がどれだけ Evidence に支持されているか」を表し、Quality Flags は「そもそもインサイトの定義を満たしているか」を表す。両者は独立した軸である。繰り返しに強く支持された顕在ニーズの言い換えは、Confidence が高く Quality Flag 付き、という形で現れる。Confidence の式は変更していない。

### 23.5 DB

`002_traces_and_quality.sql` で `patterns.kind / expectation / deviation_type` と `insights.expectation / surprising_fact / quality_flags` を追加。既存行の `kind` は DEFAULT `'repetition'` となる。

## 24. 入力契約と取り込み（前処理） **[追加]**

「評価はできるが、ユーザーの声を集約・正規化・構造化する前処理の方が大変」という指摘に対応した。汎用 ETL は作らず、**分析エンジンと同じ「機械が提案し、アプリが検証し、人が確定する」型を入口にも適用**する。

### 24.1 問題

- 書き起こしには質問者の発言が混ざる。従来は質問者の言葉も Observation として抽出でき、原文照合も通ってしまった（捏造引用は防げるが、質問者の引用は防げなかった）。
- 商談メモは営業担当の解釈であり、本人の発言と同格の Evidence にすると Confidence が実態より上がる。
- 痕跡検出（§23）は「その状況の人ならこう動くはず」から始まるが、役職・規模・利用量が渡っていないと予想が一般論になる。
- 納品案件では氏名・連絡先をマスクしてから LLM に送る必要がある。
- 取り込み形式が客ごとに違い、固定4列 CSV への手作業整形がボトルネックになる。

### 24.2 入力契約

Document に `Provenance`、`Spans`（話者区間）、`RawContent`（マスク前原文）と予約メタデータキーを追加した（§5）。以降のすべての取り込み方法（貼り付け、CSV/TSV、将来の LLM 構造化）は、この契約への変換として実装する。

### 24.3 話者区間と原文照合

- `GroundWithin(content, quote, customerSpans)`: 引用は回答者の区間の内側にある場合のみ採用。質問者・担当者の発言を逐語的に引用しても、捏造引用と同じ扱いで破棄する。件数は `quotesOutsideCustomerSpeech` として捏造（`unsupportedClaimRate`）とは別に集計する（前者は取り込みの問題、後者はモデルの問題）。
- LLM には `【回答者】【質問者】【担当者】` のラベル付きで渡し、質問は文脈として読ませる。ラベルは `Document.Content` には存在しないので照合には影響しない。

### 24.4 取り込み器（決定的・LLM 不使用）

| 取り込み | 実装 | 記憶されるもの |
|---|---|---|
| 書き起こしの話者分離 | `service/transcript.go`。「面接官:」「Q.」「[00:12] 田中:」「【回答者】」を検出。既知ラベルは辞書、未知の名前は発話量ヒューリスティック（多く話す方が回答者）。推定は `guessed` として警告 | `IntakeProfile.SpeakerRoles` |
| CSV/TSV 列マッピング | `service/table_import.go`。区切り自動判別、BOM 除去。列名から本文 / タイトル / ID / 種別 / 予約メタデータへの対応を提案（日英語彙）。本文セルが会話形式なら話者分離も適用 | `IntakeProfile.ColumnMapping` |
| PII マスキング | `service/pii.go`。メール / URL / 電話 / 〒郵便番号 / カード番号 / 敬称付き氏名（「田中さん」→「[氏名]さん」、お客様・皆様などは除外）と辞書語。マスク後の本文に対して話者分離を行い、区間はマスク後テキストを指す | `IntakeProfile.MaskTerms` |

**プレビューが本体**である。貼り付けもファイルも、保存前に「回答者として分析される文字数 / 除外した区間 / マスク箇所 / 推定した役割」を見せ、人が直してから確定する。確定した対応はプロジェクトに記憶され、次回から既定になる。案件ごとの取り込みプロファイルはローカル DB にだけ存在し、OSS 版には汎用の取り込み器と語彙だけが入る。

UI からの保存は常に `detectSpeakers: true` で行い、区間はサーバー側でマスク後テキストから再計算する。クライアントが渡した区間はマスクで本文が変化した場合に拒否する（オフセットの不整合を構造的に防ぐ）。

### 24.5 方法論への接続

- Observation を LLM に渡す際に `documentId` / `situation` / `provenance` を付与する。痕跡検出は「月150件発行する営業事務なら自動化するはず」のように状況固有の予想を立て、同一人物の「言っていること」と「やっていること」を突き合わせる。
- 二次情報由来の Evidence は relevance を 0.7 倍に減衰し、支持する引用がすべて二次情報なら品質フラグ `secondhand_only` を付ける。
- 予約メタデータは将来の Segment Discovery（§19）の土台になる。

### 24.6 未実装（次の候補）

- LLM による構造化（崩れた貼り付けを話者・発言ごとに分割させ、各断片を `Ground` で原文照合してから採用）
- XLSX の直接読み込み（現状は CSV/TSV に書き出してもらう）
- マスク前原文の閲覧 UI（DB には `raw_content` として保持済み）
