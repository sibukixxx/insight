# Discovery Benchmark v1 (#120)

「Insight を何件生成したか」では、要約やハルシネーションを発見と取り違える。Discovery Benchmark は、Core が **仕込まれた非自明なズレを拾えるか**、**何もないときに何も言わないか**、**もっともらしい偽の関連を関連止まりに留められるか** を固定ケースで測る。

## 何を測るか

| 区分 | 件数 | 意味 |
|---|---:|---|
| `POSITIVE_DISCOVERY` | 3 | 仕込んだズレ（単月の外れ値・水準シフト・参照系列からの乖離）を Core が候補として出す |
| `VALID_NO_DISCOVERY` | 3 | ノイズ・季節性・参照系列と同じ動きしかないとき、発見を出さない |
| `FALSE_ASSOCIATION_GUARD` | 3 | 共通トレンドによる高相関、母集団定義の変更、同一ソースの再投入を、発見や独立 Evidence として扱わない |

ケースはすべて合成データでドメイン中立。private な顧客・コマース情報は含めない。

- ケース: `testdata/golden/discovery/cases/DB-*.json`
- 計測結果（ベースライン）: `testdata/golden/discovery/baseline.json`
- Core を実際に動かす Go テスト: `internal/goldenset/discovery_benchmark_test.go`（`-tags=golden`）
- ケース集合の整合性チェック: `testdata/golden/harness/discovery_benchmark.py`

## 二層の評価

1. **決定的な不変条件（CI で判定）** — Go テストが各ケースの入力を Temporal Analytics Pack（`ApplyTemporalOperation` → `ToCandidatesForAnalysis`）と dataset pre-analysis（`RunDatasetPreAnalysis`）に通し、Core が実際に出した値・フラグ・limitation・注記をケースの assertion と照合する。結果はベースラインと完全一致しなければ失敗する。
2. **人手評価（AI に自己採点させない）** — novelty / grounding / surpriseValidity / hypothesisDiversity / falsifiability / missingEvidenceSpecificity / reproducibility / usefulnessForContinuedResearch の 8 項目を各ケースの `humanRubric` に置く。人が `humanReview` を記録するまで全項目 `PENDING` のままで、Shared Eval Contract には `NOT_REVIEWED` として渡る。ハーネスがこれらを PASS にすることはない。

## 入力と実行の分離

ベースラインは各ケースに `inputFingerprint` と `executionFingerprint` を別々に記録する。

- `inputFingerprint`: 研究質問・外部 claim・入力系列/文書・operation spec のハッシュ
- `executionFingerprint`: `service.BuildExecutionSnapshot`（決定的モード、ビルド情報なし）の実行設定と、`datasetPreanalysis` / `analyticalArtifact` / `grounding` / `temporalOperation` のルールバージョン

結果が変わったとき、入力が変わったのか、ルール（計算器）が変わったのかをこの 2 つで切り分ける。派生 Artifact のハッシュは生の浮動小数点に依存しアーキテクチャ差が出うるため記録せず、assertion が観測値を安定した精度で残す。

## 現状のベースライン（2026-09-26）

9 ケース中 7 件 PASS、2 件 KNOWN_FAILURE。

### 成功例

- **DB-P01 単月スパイク**: 安定した月次系列の 2024-07 だけが robust z = 35.7 で `ANOMALY_CANDIDATE`。他の月は候補にならない。
- **DB-P02 水準シフト**: 2 年平均では目立たない 2024-01 からの水準変化を、change-point の shift score 0.987 で候補化。「確認された構造変化ではない」という limitation が付く。
- **DB-P03 参照系列からの乖離**: break 後に A だけ上がり、difference of changes = 19.5。「参照系列の比較可能性は検証していない」と明記される。
- **DB-N01 / DB-N03**: ノイズだけの系列では候補ゼロ。欠損月は `MISSING_OR_GAP` のまま 0 にならない。参照系列と同じだけ動いた場合は差が 0 になる。
- **DB-F01 共通トレンド**: 無関係な 2 系列の相関は lag 0 で 0.998 と高いが、出力には "Association only" と「なぜ動いたかは説明しない」の limitation が必ず付く。外部 claim は RESEARCH_REVIEW の入力として検証され、Evidence には昇格しない。
- **DB-F02 定義変更**: 2023 年だけ母集団定義が違うため、2022→2023 の差分は計算されず（比較は 2021→2022 の 1 件のみ）、`population_mismatch` と `schema_version_mismatch` の警告が残る。

### 失敗例（KNOWN_FAILURE）

- **DB-N02 季節ピーク**: `anomaly_candidate` は系列全体の robust z-score で季節調整をしないため、毎年繰り返す 12 月のピーク（2023-12, 2024-11, 2024-12）を外れ値候補にしてしまう。YoY は +2.0% で一定で、季節性だと分かる。季節性のある系列で anomaly 候補だけを見て「発見」とするのは誤り。
- **DB-F03 同一ソースの再投入**: dataset hash・manifest・期間比較は再投入を 1 ソースに畳む（比較は重複期間のため保留され、注記が残る）。一方で dataset pre-analysis は文書ごとに Observation を作るため、同じ 2023 年ファイルから 2 つ目の `record_count` Observation ができる。「重複を独立 Evidence として上積みしない」は Observation 層ではまだ満たしていない。

KNOWN_FAILURE の assertion が後で通るようになると Go テストは失敗する。直したら `knownFailure` を外してケースの `version` を上げ、ベースラインを再生成する。

## 限界

- CI の決定的経路はモードによって計算を変えない。DISCOVERY / DATASET_ANALYSIS / RESEARCH_REVIEW の違いは実行フィンガープリントと入力（質問・claim）の違いとして記録されるだけで、モード間の発見能力の差は測っていない。モデルを使う段階（仮説生成・Insight 化）の比較は `make eval-demo` と人手評価で行う。
- 各ケースは operation と閾値を明示している。どの operation を選ぶべきかの判断は測っていない。
- 合成データは小さく、仕込んだズレは明瞭。実データの曖昧さ（欠損の偏り、定義の段階的変更）は反映していない。実 Open Data の dogfood（sibukixxx/techvit-insight#54）で見つかった失敗を、private な情報を除いたうえでケースに加えていく。
- RESEARCH_REVIEW ケースは claim の入力検証までで、主張の分解・検査は #119 の範囲。

## 実行

```bash
make test-golden                                                     # Python ハーネス + Go golden（ベンチマーク含む）
go test -tags=golden ./internal/goldenset/ -run DiscoveryBenchmark    # ベンチマークだけ
python3 testdata/golden/harness/discovery_benchmark.py                # ケース集合のチェック表
python3 testdata/golden/harness/discovery_benchmark.py --case DB-F01 --report report.md   # 禁止表現スキャン
GOLDEN_UPDATE=1 go test -tags=golden ./internal/goldenset/ -run DiscoveryBenchmark        # 意図した変更後にベースライン再生成
```

## 編集ルール

- `expected` を変えたらケースの `version` を上げる。ベースラインの `version` がずれると Python チェックが失敗する。
- 期待を満たさない結果を assertion の緩和で隠さない。直せないなら `knownFailure` に理由付きで記録する。
- `humanRubric` と `humanReview` は人が埋める。ツールで埋めない。
