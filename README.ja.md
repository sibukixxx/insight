# Insight Lab

**観測事実から、監査可能な仮説へ。相関を因果と決めつけない、Evidence-Groundedなリサーチ基盤です。**

[English](README.md) · [ドキュメント](docs/README.md) · [因果推論セマンティクス](docs/causal-reasoning.md) · [Contributing](CONTRIBUTING.md)

## Insight Labとは

LLMは、もっともらしい説明を作ることが得意です。しかし「もっともらしい」と「根拠によって支持されている」は同じではありません。

Insight Labは、分析過程を次のような追跡可能な構造として扱います。

```text
Data
  ↓
Observation（観測事実）
  ↓
Expectation（予想）
  ↓
Expectation Mismatch / Surprise（予想とのズレ）
  ↓
Primary + Competing Hypotheses（競合仮説）
  ↓
Supporting Evidence / Counter Evidence
  ↓
Missing Evidence / Falsification Criteria
  ↓
Validation / Identification Status
```

目的はAIに自信満々な結論を書かせることではありません。

**何を観測したのか、どこから推論したのか、何がその推論に反するのか、そして何がまだ分からないのかを、人間が監査できるようにすること**が目的です。

## 現在できること

- SQLiteを利用したlocal-firstなプロジェクト管理
- テキストおよびCSVの取り込み
- 原文にgroundingされたObservation抽出
- ExpectationとMismatchの検出
- アブダクションによる仮説生成
- Supporting / Counter / Neutral Evidenceの区別
- `HypothesisSetID`によるPrimary / Competing Hypothesisのグループ化
- 競合仮説ごとの独立したEvidence / Counter Evidence評価
- Exposure / Outcome / Confounder / Mediator / Collider / Unknownを表現するCandidate Causal Structure
- Causal / Validation / Identification Status
- アプリケーション側の決定的Quality Guardrail
- Evidence Trailを含むMarkdownレポート
- OpenAI-compatible API設定
- 実LLMを使った再現可能なDogfooding / Evaluation

## 因果関係についての重要な方針

Insight Labは**因果効果推定器ではありません**。

LLMが因果関係らしい文章を書いたこと、Evidenceが多いこと、Confidenceが高いことだけを理由に、因果関係を証明済みとして扱いません。

現在のpipelineでは、適切な外部研究デザインによる識別が行われていない因果仮説を `NOT_IDENTIFIED` として扱います。

Control Group、Pre/Post、Natural Experiment、Difference-in-Differences、RDD、IVなどを「次に検討すべき研究デザイン」として提案することはできますが、実際にその分析を実行したとは主張しません。

詳細は [Evidence-Grounded Causal Reasoning Semantics](docs/causal-reasoning.md) を参照してください。

## Quick Start

### 必要環境

- Go 1.25+
- モデルを利用した分析を行う場合はOpenAI-compatible API

### Fictional Demo

```bash
make build-demo
./bin/insight-lab-demo --demo
```

ブラウザで `http://127.0.0.1:8787` を開きます。

Settings画面、または `--base-url` / `--model` / `--api-key` からモデルを設定できます。

### 自分のデータを分析する

1. Projectを作成する
2. テキストを貼り付ける、または `id,source,title,content` 形式のCSVをimportする
3. Analysisを実行する
4. Observation、競合仮説、Evidence、Counter Evidence、Missing Evidence、Quality Warning、Identification Statusを確認する
5. Markdown Reportをexportする

```bash
curl -o report.md http://127.0.0.1:8787/api/projects/<projectID>/report.md
```

入力は設定したモデルが対応する言語を利用できます。生成される分析は入力言語に追従するよう指示されます。

## Build / Test / Evaluation

```bash
make build
make build-demo
make vet
make test
```

実モデルを利用した評価:

```bash
INSIGHT_LAB_API_KEY=sk-... \
INSIGHT_LAB_MODEL=<model> \
make eval-demo
```

評価結果は `docs/evaluation/` に保存されます。

## Confidenceについて

Insight Labのスコアは、grounding、coverage、source diversity、frequency、counter-evidenceなどから計算される**Evidence Qualityの内部指標**です。

**「この主張が正しい確率」でも「因果関係が正しい確率」でもありません。**

## OSSとしての境界

Insight Labは公開可能なResearch / Diagnostic Engineです。

OSS側では、Observation、Hypothesis、Evidence、Counter Evidence、Validation、Insight Candidateなどの再利用可能な分析基盤を扱います。

営業提案、価格判断、見積、顧客固有のRecommendation、商用Decision LogicなどはこのOSSの責務に含めません。

詳細は [Project Scope](docs/project-scope.md) を参照してください。

## 外部データとの接続

特定のデータ提供元をInsight Labのdomainへ埋め込みません。

```text
外部公開データ / Collector
        ↓
CSV（JSONL adapterは将来候補）
        ↓
Insight Lab
        ↓
Evidence-Grounded Analysis
```

この境界によって、企業データ、行政オープンデータ、調査データなどを同じ分析基盤へ接続できます。

## Documentation

全体像は [Documentation Index](docs/README.md) から確認できます。

特に重要なドキュメント:

- [Project Scope](docs/project-scope.md)
- [Current Project Status](docs/project-status.md)
- [Causal Reasoning Semantics](docs/causal-reasoning.md)
- [因果推論 Learning Notes](docs/learning-notes-causal-inference.md)
- [Detailed Design](docs/detailed-design.md)
- [Evaluation](docs/evaluation/README.md)
- [Contributing](CONTRIBUTING.md)
- [Security](SECURITY.md)

## Privacy

Project Dataはローカルに保存されます。ただし、モデルを利用する分析では必要なテキストが設定したAI Providerへ送信されます。機密情報・個人情報・規制対象データを扱う前に、利用するProviderのデータ取扱方針を確認してください。

API KeyなどのSecretをリポジトリへcommitしないでください。

## License

Copyright 2026 Yuichi Takada.

[Apache License 2.0](LICENSE) で公開しています。依存ライブラリにはそれぞれのライセンスが適用されます。
