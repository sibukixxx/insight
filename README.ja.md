# Insight Lab

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.25%2B-00ADD8.svg)](go.mod)

オープンソース・ローカルファーストの Research / Evidence Reasoning Engine です。

与えられたEvidenceを分析し、AnalysisとResearchの履歴を保存しながら、Observation、Hypothesis、ResearchGap、Timeline、Scenario Evaluationを扱います。決定論的処理だけでも利用でき、OpenAI互換モデルを接続した分析にも対応します。

[English](README.md) · [Documentation](docs/README.md) · [Architecture](docs/architecture.md) · [Public Engine Contract](docs/public-engine-contract.md)

## Quick start

必要環境:

- Go 1.25+
- SQLiteは組み込み。外部DBは不要

BuildしてReference Serverを起動します。

```sh
make build
./bin/insight-lab serve -no-browser
```

既定のendpoint:

```text
http://127.0.0.1:8787
```

APIのみ:

```sh
./bin/insight-lab serve -no-web
```

HTTPを起動せずengine capabilityを確認:

```sh
./bin/insight-lab engine
```

Headless CLIには `subject`、`evidence`、`analysis`、`research`、`status` があります。

Model-backed分析ではOpenAI互換endpointを指定します。

```sh
./bin/insight-lab serve \
  -base-url https://example.invalid/v1 \
  -model your-model \
  -api-key "$API_KEY"
```

Model未設定でも、決定論的なingestion、validation、temporal operation、対応済みanalytical processingは利用できます。Model生成のHypothesisにはModel設定が必要です。

## 何をするソフトウェアか

基本フロー:

```text
Evidence
  ↓
Observation / Claim
  ↓
Expectation / Mismatch
  ↓
Primary + competing hypotheses
  ↓
Supporting / counter / neutral evidence
  ↓
ResearchGap / DataRequirement
  ↓
ResearchIteration
  ↓
Re-evaluation / Timeline / Scenario
```

Research stateはCore自身が永続化します。再解析しても過去のRunは上書きしません。

## Input

| Input | 境界 |
| --- | --- |
| Text evidence | Documents / Document CSV |
| Structured observations | Dataset Documents |
| 外部の決定論的分析結果 | Analytical Artifact v1 |
| Large/raw files | `RAW_ARTIFACT` reference + 対応済みpreparation spec |

巨大なraw datasetそのものはSQLiteへ保存しません。外部で正規化・集計するか、対応済みraw-artifact preparationを通してEvidenceとしてCoreへ渡します。

汎用XLSX/PDF/Parquet ingestion、任意SQL接続、Web crawling、SaaS connectorはCoreの直接責務ではありません。

詳細: [BYO-Evidence boundary](docs/byo-evidence-boundary.md) / [Analytical Artifact contract](docs/analytical-artifact-contract.md)

## Research model

5つの軸は独立しています。

| Axis | Values |
| --- | --- |
| AnalysisMode | Discovery / Dataset Analysis / Research Review |
| ReasoningProfile | `GENERAL_RESEARCH` / `CUSTOMER_INSIGHT` |
| ResearchStage | `DISCOVERY` / `EXPLORATORY` / `VALIDATION` / `SYNTHESIS` |
| ExecutionMode | deterministic / model-backed |
| ExecutionProfile | `LIGHT` / `STANDARD` / `HEAVY` / `AUTO` |

既定ReasoningProfileは `GENERAL_RESEARCH` です。Profileをsource type、namespace、文章内容から推測しません。

詳細: [Architecture](docs/architecture.md) / [Causal reasoning semantics](docs/causal-reasoning.md)

## Persistence

既定の永続StoreはSQLiteです。

Coreが保存するResearch state:

- Subject / Analysis
- Observation / Evidence
- Insight / Hypothesis
- ResearchRun / append-only ResearchIteration
- Temporal Evidence
- ScenarioSet / ScenarioEvaluation
- Human research evaluation

Analysisにはinput snapshotとexecution snapshotも保存し、Evidence変更と実行設定変更を区別できます。

Longitudinal TimelineやObservation Deltaなど、保存済みstateから再構築できるread modelは原則として別の正本を持ちません。

DB pathを指定する場合:

```sh
./bin/insight-lab serve -db ./insight.db
```

`-db` を省略するとOSのapplication data directoryを使用します。

## Public API / SDK

言語非依存のPublic API:

```text
/api/public/v1
```

正典schema / conformance fixture:

```text
contracts/public-engine/v1
```

Standalone SDK:

- [insight-sdk-go](https://github.com/sibukixxx/insight-sdk-go)
- [insight-sdk-js](https://github.com/sibukixxx/insight-sdk-js)

SDKはthin clientです。CoreはSDKへ依存しません。

## Execution profile

- `LIGHT` — 小規模・ローカル処理
- `STANDARD` — bounded streaming / concurrent preparation
- `HEAVY` — Heavy Runtime adapterへ委譲
- `AUTO` — 決定論的にprofileを選択し、結果を記録

Raw file referenceやlocal Heavy Runtimeを有効にする例:

```sh
./bin/insight-lab serve -input-root ./data -heavy-dir ./heavy
```

## 運用責任

Insight Labはself-hosted softwareです。利用者が以下を管理します。

- deployment / access control
- DBのbackup / restore
- Model credential / provider設定
- 外部・raw data storage
- retention / privacy policy
- monitoring / availability

Managed productはこれらをCoreの外側で提供できますが、OSS CoreのResearch semanticsには含めません。

## Non-goals

Insight Coreは以下を目的にしません。

- 自律Web research agent
- 汎用data warehouse
- causal effect estimator
- most-likely futureを選ぶforecast engine
- managed multi-tenant SaaS control plane

Unknown、insufficient evidence、not identifiedは正常なResearch結果です。

## Build / Test

```sh
make build
make test
make vet
```

Golden evaluation:

```sh
make test-golden
```

## Documentation

- [Documentation index](docs/README.md)
- [Architecture](docs/architecture.md)
- [Public Engine Contract v1](docs/public-engine-contract.md)
- [Research loop](docs/research-loop.md)
- [Temporal evidence](docs/temporal-evidence.md)
- [Longitudinal research](docs/longitudinal-research.md)
- [Scenario analysis](docs/scenario-analysis.md)
- [Project scope](docs/project-scope.md)
- [Project status](docs/project-status.md)

## License

Apache License 2.0. [LICENSE](LICENSE)
