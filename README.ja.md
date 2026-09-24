# Insight Lab

**Evidenceを持ち込んで、観測事実から監査可能な仮説・反証・不足Evidence・次の調査へ進むための、local-firstなOSSリサーチエンジンです。**

[English](README.md) · [ドキュメント](docs/README.md) · [因果推論セマンティクス](docs/causal-reasoning.md) · [Project Scope](docs/project-scope.md) · [Contributing](CONTRIBUTING.md)

## Insight Labとは

Insight Labは、**Bring Your Own Evidence（BYO Evidence）型のEvidence Reasoning Engine**です。

対象となる入力は、すでに何らかの形で存在している情報です。

- 顧客インタビュー、レビュー、問い合わせ、商談ログ、アンケートなどの一次情報
- 業務データ、BI出力、オープンデータ、外部adapter出力などをDataset Documentまたは対応adapter contractへ正規化した構造化データ
- 社内調査、外部調査会社レポート、コンサル資料、市場調査、AI分析などの既存Research Artifact

Insight Labは一般的な「質問すればWebを探して答えるAIリサーチチャット」ではありません。

中核となる流れは次です。

```text
Provided Evidence
  ↓
Observation / Claim
  ↓
Expectation + Provenance
  ↓
Mismatch / Surprise
  ↓
Primary + Competing Hypotheses
  ↓
Supporting / Counter / Neutral Evidence
  ↓
Research Gap / DataRequirement
  ↓
Validation / Identification
  ↓
Decision Readiness / Human Handoff
```

Evidenceが不足している場合は、もっともらしい結論を作るのではなく、**何が不足しているかを構造化して返す**ことを重視します。


## 何を入力でき、何を分析できるか

以下は **current mainで実際に受け付ける入力境界** です。「CSVなら何でも読める」「PDFやExcelをそのまま解析できる」という意味ではありません。外部データは、下記のDocument / Dataset境界へ正規化してから渡します。

| 入力 | 現在の受け付け方 | Insightが行う分析 | LLM |
| --- | --- | --- | --- |
| インタビュー、レビュー、問い合わせ、商談ログ、アンケート自由記述、求人文、SNS投稿 | UI/APIでDocument作成、またはDocument CSV | groundingされたObservation、Pattern/Mismatch、Primary/Competing Hypotheses、Supporting/Counter/Neutral Evidence、ResearchGap | 意味分析には必要 |
| 複数のテキストEvidence | 固定4列Document CSV | 各行をDocumentとして同じEvidence Reasoning pipelineへ投入 | 意味分析には必要 |
| 集計済みの数値系列 | APIで `source=dataset` のDocumentとして登録 | `record_count` のgrounded Observation、期間比較、delta、rate of change、baseline差、series内share | deterministic部分は不要 |
| 企業イベントregistry CSV | 専用Analysis CSV import | 月 × event type × 地域 × provider/versionに決定論的集計し、その後Dataset Analysis | 集計は不要、仮説生成等には必要 |
| 外部調査、社内分析、コンサル資料、AI分析の内容 | 内容をText Documentへ変換、またはResearchRun APIでClaimとして投入 | ClaimとObservationを分離し、根拠・反証・不足Evidenceをレビュー | 通常必要 |
| Acquisition Manifest | CSV import時の付随JSON | source / dataset ID / retrieval time / unit / population / schema / hash等のprovenance保持と互換性warning | 不要 |
| 追加Evidence | ResearchRunの新しいiterationとして投入 | `DataRequirement.gapId` とのlink、ValidationEvidence provenance、Insight Delta | 内容により必要 |

### Document入力

API/UIから1件ずつ投入するDocumentは、現在次の `source` を受け付けます。

```text
interview
review
support
sales
survey
job_posting
social_post
dataset
```

Documentの基本形は `source / title / content / metadata` です。Text系Documentでは `content` から引用可能なObservationをgroundingし、そこからMismatch・仮説・Evidence/Counter Evidenceへ進みます。

### Document CSV

汎用CSV importは **UTF-8の固定4列** です。Excel由来のUTF-8 BOMは除去されます。

```csv
id,source,title,content
1,interview,Interview 01,"導入は簡単だったが、毎月の集計が面倒"
2,support,Ticket 42,"CSV出力後に手作業で列を合わせている"
3,survey,Survey response,"レポート作成に毎週2時間かかる"
```

- 先頭4列は `id,source,title,content` である必要があります。
- `source` は上記8種類のいずれかです。
- `content` は必須です。
- `id` はtraceability用で、Insight内部IDとしては使いません。
- 追加列があっても現在の汎用importでは分析列として扱いません。

したがって、**任意の表形式CSVをアップロードしただけで全列を自動統計分析する機能ではありません。**

### Dataset Document

任意の数値データをInsightのdeterministic pre-analysisへ渡す場合は、外部で集計・正規化して `source=dataset` のDocumentへします。

現在のpre-analysisで重要なmetadataは次です。

```json
{
  "source": "dataset",
  "title": "2026-01 Example City inquiries",
  "content": "Dataset observation: 2026-01 Example City inquiries = 120.",
  "metadata": {
    "record_count": "120",
    "period": "2026-01",
    "event_type": "inquiry",
    "location": "Example City",
    "source_provider": "internal-export",
    "source_version": "v1"
  }
}
```

`record_count` が数値ならgrounded Observationを決定論的に作れます。さらに同じseriesで `period` が2期間以上あれば、次をLLMなしで計算します。

- 前期間からの差分
- rate of change
- baselineからの差分
- series totalに対するshare

Acquisition Manifestに `unit` や `populationScope` がある場合、異なる母集団・単位を無条件に同じseriesとして差し引かないようにします。

### Corporate-event Analysis CSV

現在、raw tabular data向けに実装済みの専用adapterはcorporate-event Analysis CSVです。最低限、次の列を要求します。

```text
corporate_number
event_type
prefecture_name
city_name
assignment_date
update_date
change_date
close_date
source_provider
source_version
source_fetched_at
```

`ASSIGNED / UPDATED / CHANGED / CLOSED` ごとの対応日付を使い、月 × event type × 地域 × provider/versionで件数を集計します。

この件数は**行政レコード件数**であり、「創業数」「開業数」「政策効果」へ自動変換しません。

### Research Review入力

すでに解釈済みの資料は、ファイル形式そのものではなく **Claimとしてどう扱うか** が重要です。

たとえば外部レポートに、

> 「施策Aによって問い合わせが増加した」

と書いてあっても、Insightはそれを一次Observationへ昇格しません。Text Documentとして内容を渡すか、ResearchRun APIの `claims` と `inputReferences` へ構造化し、Underlying Evidence・Assumption・Counter Evidence・ResearchGapを確認します。

### 現在直接は食べないもの

current mainでは、以下をそのままファイル投入するstable pathはありません。

- 任意schemaのCSVをそのままtabular analyticsすること
- JSON / JSONLの汎用file import
- XLSX / Excel workbook
- PDF / DOCX
- Parquet
- SQL databaseへの直接接続
- Web URLのcrawl / scraping
- Google Drive / CRM / SaaS connector
- 画像・音声・動画そのもの

これらはInsight外で **Text / Document CSV / Dataset Document / domain adapter output** へ変換してから投入します。


## OSS境界 — Bring Your Own Evidence

Insight Lab内部で扱うもの:

- genericなDocument / Dataset ingestion
- 可能な範囲でのdeterministic dataset pre-analysis
- groundingされたObservation / Claim
- Expectation / Mismatch
- Primary / Competing Hypotheses
- Supporting / Counter / Neutral Evidence
- ResearchGap / provider-neutralな `DataRequirement`
- append-onlyなResearchIteration
- Validation / Identification / Decision Readiness
- Markdown Report
- versioned JSON Research Artifact

Insight Lab自身が行わないもの:

- autonomous Web search
- e-Statや各種registryへの認証付き自動取得
- Drive / CRM / SaaS等のmanaged connector
- 顧客固有Credential管理
- 外部データの継続監視
- 自律Evidence Acquisition Agent
- 見積、価格、提案書、顧客固有Recommendation

不足Evidenceがあれば、Insight Labは `DataRequirement` を返します。

```text
Insight Lab
   ↓ DataRequirement
外部 / Private Orchestration Layer
   ↓ Evidence取得
Insight Lab
   ↓ new ResearchIteration
再分析
```

つまり、**Evidence取得責任だけを外へ出し、研究ループそのものはInsightへ戻ります。**

詳細は [BYO-Evidence boundary](docs/byo-evidence-boundary.md) を参照してください。

## Analysis Mode

現在の基本設計では、入力ファイル形式ではなく**情報の意味論・成熟度**によって分析方法を切り替えます。

### 1. Discovery

未解釈の一次情報向けです。

対象例:

- interview
- review
- support
- sales
- survey free text
- customer feedback

```text
Raw Evidence
→ Observation
→ Pattern / Latent Need
→ Hypothesis
→ Evidence / Counter Evidence
→ Research Gap
```

### 2. Dataset Analysis

構造化データ向けです。

対象例:

- normalized Dataset Documents
- supported CSV adapter output
- operational metrics normalized into Dataset Documents
- e-Stat / 自治体Open Dataを外部adapterで正規化したもの
- corporate-event Analysis CSV

```text
Structured Dataset
→ Schema / Unit / Period / Population Check
→ Deterministic Aggregation / Delta / Baseline
→ Candidate Observation
→ Mismatch
→ Competing Hypotheses
→ Evidence / Counter Evidence
→ Research Gap
```

数値のsource of truthは可能な限りdeterministicに処理します。LLMに権威ある数値を再計算させません。

### 3. Research Review

すでに誰かが解釈した資料向けです。

対象例:

- 顧客社内の分析
- 外部調査会社レポート
- コンサル資料
- BI narrative
- 人間の分析メモ
- ChatGPT / Claude等のAI分析

重要なルール:

> 外部資料に書かれたClaimを、そのままObservationや一次Evidenceへ格上げしない。

Claim / Evidence / Assumption / Method / Counter Evidence / Missing Evidenceへ分解してから評価します。

**現状:** `DISCOVERY` / `DATASET_ANALYSIS` / `RESEARCH_REVIEW` のfirst-classなsemantic Analysis Modeはmainに実装済みです。Research Reviewでは外部資料のClaimをClaimとして保持し、Observationや一次Evidenceへ暗黙に格上げしません。各Modeは同じEvidence Reasoning Coreへ収束し、source固有の取得処理は引き続きInsight外です。

**Analysis ModeとExecution Modeは別概念です。** Analysis Modeは入力をどう読むかを表し、`service.ExecutionMode` の `deterministic / model_backed` はLLMが実行に参加したかだけを表します。両者は独立して記録されます。Research Artifact v1の既存JSON key `analysisMode` は互換性のためExecution Modeとして維持し、semantic modeは別fieldとしてexportします。

## Analysis ModeとResearch Stageは別物

Analysis Mode:

> この入力をどう読むか

Research Stage:

> 今の知見が研究工程のどこにいるか

Research Stageは別軸です。

```text
DISCOVERY
→ EXPLORATORY
→ VALIDATION
→ SYNTHESIS
```

データを見た後に生成した仮説はExploratoryです。同じデータを使って「事前仮説を検証した」ように扱ってはいけません。

現在のmainには `ResearchIteration.Stage` に加え、first-classな `Expectation` entity、provenance、freeze-for-validation、`EXPLORATORY → VALIDATION` guard、iteration間carry-forward、Research Artifactへのexportまで実装されています。このExpectation lifecycleは [#31](https://github.com/sibukixxx/insight/issues/31) で完了済みです。

## 現在mainでできること

- SQLiteによるlocal-first project
- text / CSV ingestion
- source-backed grounding
- LLMなしでも動くdeterministic dataset pre-analysis
- acquisition manifest / dataset hash provenance
- unit / population / period compatibility warning
- OpenAI-compatible modelを使った意味解釈
- Primary / Competing Hypotheses
- Supporting / Counter / Neutral Evidence
- Causal / Validation / Identification Status
- append-onlyな `ResearchRun` / `ResearchIteration`
- semantic Analysis Mode（`DISCOVERY` / `DATASET_ANALYSIS` / `RESEARCH_REVIEW`）とResearch Claim
- first-class `Expectation` provenance / freeze-for-validation / iteration間lineage
- independent validation Evidence provenance
- ResearchGapの優先順位付け
- Next Data Requirement
- `DataRequirement.gapId` と追加Evidenceの明示link
- Insight Semantics v2（Connection / Mechanism Candidate / Generalization・boundary condition）
- iteration input snapshotとInsight Delta
- Decision Readiness
- Stop Reason
- Human Override / Human Handoff
- Markdown Research Report
- versioned JSON Research Artifact
  - `GET /api/research-runs/{runID}/artifact.json`
  - Research Stage / semantic mode / Expectations / Claims / validation evidence / gap linkage / Insight Delta / Promotionをexport
- human review済みのapproved artifact snapshot
- deterministic quality guardrail
- Shared Eval / Golden evaluation基盤
  - association-only / population mismatch / 実Open Data再現 / new-evidence再分析 / inconclusive / competing-hypothesis / promotion / human-review cases

Public Report向けPromotion workflowはdomain/service/usecase/HTTP/reportまで実装済みです。`PUBLICATION_READY`にはHuman Reviewが必須で、自動公開は行いません。`approved-artifact.json` は後続状態から再生成せず、承認時に保存したreview済みsnapshotを返します。

## 結果は1つの解析Runに束縛される

解析Runはすべて保持され、再実行しても過去のRunは削除も上書きもされません。結果の表示は常に1つのRunに束縛され、Run同士が混ざりません。

- プロジェクト画面、痕跡・パターン、評価、`report.md` は既定で**最新の完了Run**を表示します。queued / running / failed のRunには結果がないため、暗黙に選ばれることはありません。
- 別の完了Runはプロジェクト画面のRunセレクタで選べます。APIでは `GET /api/projects/{id}/insights`、`/patterns`、`/evaluation`、`/report.md` に `?analysisId=<id>` を付けます。別プロジェクトのRunを指定すると404になります。
- `report.md` は対象Run（analysis ID、status、開始・終了時刻、mode、model、prompt fingerprint、rule version）を明記します。Runが記録していない値は空や既定値ではなく `not recorded` と表示します。
- Research report は最新iterationを生んだRunに束縛され、`artifact.json` と同じRunを指します。後から解析を実行しても変わりません。iterationのInsightが複数Runにまたがる場合は409で拒否します。

この変更以前は、再解析するとInsightとPatternが全Run分まとめて表示され、`report.md` は全RunのInsightと最新Runのmetricsを組み合わせていました。

## 因果関係について

Insight Labは**因果効果推定器ではありません**。

LLMが因果らしい文章を書いたこと、Evidenceが多いこと、Confidenceが高いことだけでは因果関係を証明済みにしません。

観察データのみで識別できない因果仮説は `NOT_IDENTIFIED` のまま扱います。

Control Group、Pre/Post、Natural Experiment、Difference-in-Differences、RDD、IV等を「次に検討すべき研究デザイン」として提示することはできますが、その分析を実際に実行したとは主張しません。

詳細は [Causal reasoning semantics](docs/causal-reasoning.md) を参照してください。

## Research Loop

Insight Labはone-shot reportで終了する設計ではありません。

```text
Evidence
→ Analysis
→ ResearchGap / DataRequirement
→ 外部取得
→ Additional Evidence
→ New ResearchIteration
→ Re-analysis
→ Stop / Continue
```

停止後も未解決Gapは消しません。

追加Evidenceは、どの `DataRequirement.gapId` に対応するものかを明示的にlinkできます。この対応関係はappend-onlyなiteration historyとResearch Artifactへ保持されるため、後から「どの不足Evidenceを埋めるために取得したものか」を追跡できます。

停止理由には、仮説の十分な識別、重要な不確実性の残存、取得可能なsourceなし、Evidence競合、人間による停止などがあります。

詳細は [Research Loop dogfooding](docs/research-loop.md) を参照してください。

## Quick Start

### 必要環境

- Go 1.25+
- model-backed analysisを使う場合のみOpenAI-compatible API

### Fictional Demo

```bash
make build-demo
./bin/insight-lab-demo --demo
```

ブラウザで `http://127.0.0.1:8787` を開きます。

### 一次情報を分析する

1. Projectを作成
2. Textを貼る、または `id,source,title,content` CSVをimport
3. Analysis実行
4. Observation / Hypothesis / Evidence / Counter Evidence / Missing Evidence / Identification Statusを確認
5. 必要ならResearch Runを作成し、追加Evidenceでiterationを継続

### 構造化Datasetを分析する

外部データはInsight外で取得し、正規化したDatasetとprovenanceを持ち込みます。

```text
External Source
  ↓
Adapter / Human / Private Acquisition
  ↓
Normalized Dataset + Acquisition Manifest
  ↓
Insight Lab
```

DatasetによってはLLMなしでdeterministic pre-analysisまで完走できます。

Hypothesis生成やnarrativeなどのmodel-backed処理を行う場合のみProvider設定が必要です。

実例は [corporate-event CSV dogfooding](docs/dogfooding-corporate-events.md) を参照してください。

### Research Artifactをexportする

```bash
curl -o artifact.json \
  http://127.0.0.1:8787/api/research-runs/<runID>/artifact.json
```

Downstream systemは `report.md` をparseせず、このversioned artifactを機械連携契約として利用します。

## 現在のロードマップ

現在は以前の大規模feature拡張より、次の方向を優先します。

1. **実データdogfooding / Public Evidence Report** — 実Open DataをResearch Loopへ通し、少なくとも1つのResearchGapを追加Evidenceで追跡し、validation provenance / Insight Delta / Promotion reviewまで実行します。Evidenceが公開基準を満たす場合はapproved artifactから公開レポートを作ります。[#60](https://github.com/sibukixxx/insight/issues/60)
2. **Stable Public Engine Contract** — 将来のGo SDK / Node.js SDKが `internal/*` に依存せず使える、狭くversionedな公開境界を定義します。[#59](https://github.com/sibukixxx/insight/issues/59)
3. **Failure-driven expansion** — dogfoodingや実consumerでgenericなcontract / semantics / correctness / performance gapが観測された場合だけcore機能を追加します。

現在の小規模interactive UIはこのフェーズには十分です。大量・巨大ファイル向けWorkspaceはactive roadmapには置かず、実workloadで必要性が確認された場合に、streaming / bounded-memory ingestion等のengine concernとUI / orchestration concernへ分割して再設計します。

Go SDK / Node.js SDK自体は**まだ存在しません**。まずInsight本体をresearch semanticsとmachine-readable contractのsource of truthとして整理し、その公開境界を固定してから別repositoryとしてSDKを作ります。
## Build / Test / Evaluation

```bash
make build
make build-demo
make vet
make test
```

Golden tests:

```bash
go test -tags=golden ./...
```

実モデル評価:

```bash
INSIGHT_LAB_API_KEY=sk-... \
INSIGHT_LAB_MODEL=<model> \
make eval-demo
```

## Confidenceについて

Insight LabのConfidence / Scoreはgrounding、coverage、source diversity、frequency、counter-evidenceなどから計算されるEvidence Quality系の内部指標です。

**「Claimが正しい確率」でも「因果関係が正しい確率」でもありません。**

Semantic classifier等のprovider confidenceも、そのbounded classification decisionへのconfidenceとしてのみ扱います。

## Project Scope

Insight Labの公開OSS境界は、Evidence-Grounded Research Artifact、Research Gap、Validation / Identification、Decision-Ready Handoff付近までです。

以下はOSS coreに含めません。

- commercial assessment
- proposal generation
- pricing / estimate
- customer-specific architecture recommendation
- customer-specific business decision

これらはdownstream applicationの責務です。

詳細は [Project Scope](docs/project-scope.md) を参照してください。

## Documentation

全体像は [Documentation Index](docs/README.md) から確認できます。

重要なドキュメント:

- [Project Scope](docs/project-scope.md)
- [BYO-Evidence boundary](docs/byo-evidence-boundary.md)
- [Current Project Status](docs/project-status.md)
- [Research Loop](docs/research-loop.md)
- [Causal Reasoning Semantics](docs/causal-reasoning.md)
- [Detailed Design](docs/detailed-design.md)（historical v1）
- [Evaluation](docs/evaluation/README.md)
- [Contributing](CONTRIBUTING.md)
- [Security](SECURITY.md)

## Privacy

Project Dataはローカルに保存されます。model-backed analysisでは、必要なテキストが設定したAI Providerへ送信されます。機密情報・個人情報・規制対象データを扱う前にProviderのデータ取扱方針を確認してください。

API KeyなどのSecretをリポジトリへcommitしないでください。

## License

Copyright 2026 Insight Lab contributors.

[Apache License 2.0](LICENSE) で公開しています。依存ライブラリにはそれぞれのライセンスが適用されます。
