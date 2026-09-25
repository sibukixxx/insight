# UI 改善提案: Run 履歴・Release 比較・指標ダッシュボード

_作成: 2026-09-24 / 状態: 提案（未実装・未合意）_

## 0. 要約

- 現在の UI が「リリースを変えたら結果がどう変わったか」に答えられない最大の原因は、**プロジェクト画面が全解析実行の Insight / Pattern を合算して表示している**こと。解析を 2 回走らせると同じ Pattern が 2 つ並ぶ（§2.1 で実測済み）。
- 一方、比較に必要なデータ（モデル名・プロンプト指紋・ルール版・データセットハッシュ・Insight Delta・iteration 比較 API）は**バックエンドにはすでに永続化・公開されている**が、UI が一切表示していない。
- 提案の骨格は「**Run（1 回の解析実行）を UI の第一級オブジェクトにし、Release（エンジン構成）と Input（証拠集合）の 2 軸で履歴を並べ、差分を帰属可能な場合だけ帰属する**」こと。ML の実験管理（config × data → metrics）と同じ構造だが、判定は人間に残す。
- Phase 0（1 PR 規模）で合算バグの解消と provenance 表示だけ行えば、混乱の大半は消える。ダッシュボードと比較画面はその上に段階的に載せる。

## 1. 前提: この文書での「リリース」の定義（要確認）

コード内に "release" という概念は存在しない。本提案では次のように定義する。**この定義が意図と違えば、以降の設計はすべて変わるので最初に確認したい。**

| 軸 | 構成要素 | 現在の記録状況 |
|---|---|---|
| **Release（エンジン構成）** | エンジンのビルド版（git tag / commit） | **未記録**。`BuildInfo` は demoBuild / clientName のみ |
| | `ruleVersion` | 記録あり。ただし `dataset-preanalysis/v1` のみで、Confidence 式・品質ルール・グラウンディング正規化の変更は反映されない |
| | `promptFingerprint` | 記録あり。sha256 のため人間が読めず、「どのプロンプト版か」が分からない |
| | モデル名 / エンドポイント | モデル名のみ記録。Base URL（プロバイダ）は未記録 |
| | 実行モード（deterministic / model_backed） | 記録あり |
| **Input（証拠集合）** | Document 集合（件数・内容ハッシュ） | データセット文書は `datasetHashes` あり。テキスト文書の集合ハッシュは**未記録** |
| | Acquisition Manifest | 記録あり |
| **Run（1 回の実行）** | 上記のスナップショット + 指標 + 生成物 | `analyses.metrics` JSON に provenance と指標がある。API の DTO はこれを落としている |

つまり「Release を変える」＝ ビルド版・ルール・プロンプト・モデルのいずれかを変えること、「Input を変える」＝ 証拠を追加・削除すること、と読む。両者は直交する 2 軸であり、比較 UI はこの 2 軸のどちらが動いたかを常に明示する。

## 2. 現状診断

### 2.1 実測: 解析を 2 回走らせると結果が合算される

デモビルドを LLM 未設定で起動し、`source=dataset` の文書 3 件（3 期間）だけのプロジェクトで決定論的解析を 2 回実行した結果:

| 実行回 | analyses 件数 | `/patterns` 件数 | 内訳 |
|---|---|---|---|
| 1 回目 | 1 | 2 | 期間比較 2 件 |
| 2 回目 | 2 | 4 | **同じタイトルの期間比較が各 2 件ずつ** |

原因はコード上も明確で、`InsightRepository.ListByProject` と Pattern の一覧に `analysis_id` の絞り込みがなく、パイプラインも JobManager も過去結果を消さない（append-only 自体は正しい方針）。Research 側は `usecase/research.go` で `analysis.ID` によって絞り込んでいるので、修正パターンはリポジトリ内にすでに存在する。

同じ理由で `report.md` も一貫していない。Insight は全実行の合算、指標は最新実行のみ、という組み合わせで出力される。

LLM ありの経路（Insight 生成）は今回実測していないが、一覧クエリが同一なので同じ挙動になる。これはコードから導いた推定。

### 2.2 UI 構造の問題

- **1 画面 1 スクロール**。プロジェクト → 解析 → Insight → テキスト貼付 → CSV → 解析 CSV → Documents が縦に並び、入力フォームが結果と証拠の間に挟まる。
- **解析パネルは最新 1 件のみ**。過去の実行、いつ・どの構成で走ったかを見る場所がない。
- **provenance が画面に出ない**。モデル名・プロンプト指紋・ルール版・データセットハッシュは `report.md` と `artifact.json` にしか現れない。設定画面のモデルを変えて再実行しても、UI 上は何も変わったように見えない。
- **評価画面は最新 1 件の静止画**。履歴も前回比もない。
- **Insight に「どの Run の産物か」がない**。カードにも詳細にも実行 ID・実行時刻・モデルが表示されない。

### 2.3 バックエンドにあるが UI に出ていないもの

| 機能 | バックエンド | UI |
|---|---|---|
| Run provenance（mode / model / promptFingerprint / ruleVersion / datasetHashes / 互換性警告） | `analyses.metrics.provenance` に永続化 | なし |
| Insight の競合仮説・validation / identification / causal status・missing evidence・反証基準・mechanism・generalization | Insight DTO で返している | 詳細画面に**未表示** |
| Research iteration 比較 | `GET /research-runs/{id}/compare/{from}/{to}` | なし |
| Insight Delta（入力差分・結果差分・仮説変化） | iteration に永続化 | なし |
| Human evaluation（人手の novelty 評価） | `PUT .../iterations/{id}/evaluation` | なし |
| Human handoff | `GET .../handoff` | なし |
| Research gaps / DataRequirements / 停止判断 / readiness | iteration に永続化 | 最新 iteration の promotion のみ |
| 実 LLM 評価の履歴 | `make eval-demo` が `docs/evaluation/<日付>-<モデル>/` に出力する規約 | アプリ外。現在ディレクトリは空 |

要するに「UI がドメインに 2〜3 世代遅れている」状態で、機能追加より**露出**の問題が大きい。

## 3. 提案の骨格

### 3.1 用語

- **Run**: 1 回の解析実行。ID・時刻・Release スナップショット・Input スナップショット・指標・生成物（Observation / Pattern / Insight）を持つ。既存の `analyses` 行そのもの。
- **Release**: Run が使ったエンジン構成。`engineVersion + ruleVersion + promptVersion(+fingerprint) + model + provider host + executionMode`。この組のハッシュを **release fingerprint** と呼ぶ。
- **Input set**: Run が読んだ証拠集合。Document ID とコンテンツハッシュの集合、データセットハッシュ、manifest。この組のハッシュを **input fingerprint** と呼ぶ。
- **Baseline**: プロジェクト内でユーザーが 1 つ選ぶ基準 Run。比較・ダッシュボードの既定の比較先。
- **Label / Note**: Run 実行時に任意で付ける短い名前とメモ（例: 「prompt v8 反証強化」「gpt-5 → claude 切替」）。ML の run name に相当。

### 3.2 差分の帰属ルール（このプロジェクトの流儀を自分自身に適用する）

比較画面は、2 つの Run について必ず次の 3 分類のいずれかを最初に表示する。

| 分類 | 条件 | 表示 |
|---|---|---|
| **Release 効果の候補** | input fingerprint 同一、release fingerprint 異なる | 「差分は Release の変更に帰属しうる」 |
| **Evidence 効果の候補** | release fingerprint 同一、input fingerprint 異なる | 「差分は証拠の追加・削除に帰属しうる」 |
| **帰属不能** | 両方異なる | 「両方が変わったため、差分をどちらかに帰属できない」 |

さらに model_backed の Run では、**同一 Release・同一 Input でも結果はぶれる**。したがって:

- 「良くなった / 悪くなった」の自動判定はしない。数値差と件数差を出すだけ。
- 同一構成での**反復実行（N 回）**を推奨し、反復がある場合は指標のばらつき（最小〜最大）を並べて表示する。ばらつきの範囲内の差は「ノイズと区別できない」と明記する。
- これは既存ルール「証拠を追加した後に解釈が変わっても、追加した証拠が原因だとは扱わない」の対称形。

### 3.3 スコープとの整合

`docs/project-scope.md` は「generic BI/dashboard platform」を範囲外としている。本提案のダッシュボードは外部データの BI ではなく、**エンジン自身の出力に対する provenance と回帰履歴**であり、同文書の「auditable reports and evaluation tooling」に属する。判断基準「別ドメイン・別データセットの研究者にも意味があるか」に対しては Yes（どのドメインでもプロンプト・モデルを変えれば結果の追跡が必要になる）。

## 4. 画面案

### 4.1 プロジェクト画面（タブ化）

```
┌──────────────────────────────────────────────────────────────────────┐
│ Insight Lab   ← Projects                          [Demo build] ⚙     │
├──────────────────────────────────────────────────────────────────────┤
│ Demo: Evidence-grounded policy research                              │
│ 12 documents · 5 runs · baseline: run #3 · latest: run #5 (完了)     │
│                                                                      │
│ [Overview] [Runs 5] [Insights] [Evidence 12] [Research 2] [Metrics]  │
├──────────────────────────────────────────────────────────────────────┤
│ Overview                                                             │
│ ┌ Latest run #5 ─────────────────────────────────────────────────┐   │
│ │ 2026-09-24 14:41 · model_backed · gpt-5 · prompts v7 (a1b2c3)  │   │
│ │ rules dataset-preanalysis/v1 · engine v0.9.2 · input 12 docs   │   │
│ │ ⚠ Release changed since baseline: model gpt-4.1 → gpt-5        │   │
│ │ Insights 4 · trace-backed 75% · flagged 25% · unsupported 3%    │   │
│ │ [Run analysis ▾ label/note] [Compare with baseline] [Report]   │   │
│ └────────────────────────────────────────────────────────────────┘   │
│ Insights (run #5)      ← 選択中 Run に限定。合算しない                │
│ …                                                                    │
└──────────────────────────────────────────────────────────────────────┘
```

- 入力（テキスト貼付・CSV・解析 CSV）は **Evidence タブ**に移す。Overview から「＋ Add evidence」で同じフォームをドロワー表示。
- Insights / Patterns は常に **選択中の Run** に限定。Run セレクタをタブ直下に置き、既定は最新完了 Run。
- Run 実行ボタンにラベルとメモの入力を付ける。

### 4.2 Runs タブ（履歴テーブル）

```
 #  日時         状態  engine  rules   prompts  model    input     insights trace% flag% unsup%  Δ vs prev
 5  09-24 14:41  完了  v0.9.2  dp/v1   v7       gpt-5    12 docs   4        75     25    3       model
 4  09-23 18:02  完了  v0.9.2  dp/v1   v7       gpt-4.1  12 docs   5        60     40    5       —(repeat)
 3  09-23 17:40  完了  v0.9.2  dp/v1   v7       gpt-4.1  12 docs   5        60     20    4       prompts ★baseline
 2  09-20 11:07  完了  v0.9.1  dp/v1   v6       gpt-4.1  12 docs   3        33     67    9       +2 docs
 1  09-19 09:12  失敗  v0.9.1  dp/v1   v6       gpt-4.1  10 docs   —        —      —     —
 [☑ 3] [☑ 5]  → [Compare selected]   [Set baseline]   [Export CSV]
```

- 「Δ vs prev」列は前 Run と比べて **何が変わったか**（release のどのフィールドか、input が何件変わったか）を 1 語で示す。何も変わっていなければ `repeat` と表示し、反復実行であることを可視化する。
- 行クリックで Run 詳細へ。

### 4.3 Run 詳細

- 上段: Release カード（engine / rules / prompts / model / provider / mode）と Input カード（doc 件数・input fingerprint・データセットハッシュ・manifest・互換性警告）。
- 中段: 指標タイル（現在の評価画面と同じ 7 種）＋ baseline との差。
- 下段: この Run の Insight 一覧、Pattern 一覧、この Run 時点の `report.md`。
- 既存の `#/projects/:id/evaluation` と `#/projects/:id/patterns` はこの画面に吸収する。

### 4.4 Run 比較（A vs B）

```
 Run #3 (baseline)  vs  Run #5
 ┌ 帰属 ────────────────────────────────────────────────────────┐
 │ Input: 同一 (fingerprint 9f3e…)                               │
 │ Release: 異なる  model gpt-4.1 → gpt-5                        │
 │ → 差分は Release 変更に帰属しうる。ただし #3 と #4 は同一構成で │
 │   flagged 20% / 40% と 20pt ぶれている。反復 1 回では判断不可。 │
 └──────────────────────────────────────────────────────────────┘
 ┌ 指標 ───────────┬──────┬──────┬────────┬────────────────────┐
 │                 │ #3   │ #5   │ Δ      │ 同構成の反復範囲(#3,#4) │
 │ Insights        │ 5    │ 4    │ -1     │ 5–5                │
 │ Trace-backed    │ 60%  │ 75%  │ +15pt  │ 60–60              │
 │ Quality flagged │ 20%  │ 25%  │ +5pt   │ 20–40  ← 範囲内     │
 │ Unsupported     │ 4%   │ 3%   │ -1pt   │ 4–5                │
 └─────────────────┴──────┴──────┴────────┴────────────────────┘
 ┌ Insight の対応 ──────────────────────────────────────────────┐
 │ 一致 3  (evidence 重なり ≥ 0.5)   変化: confidence / flags   │
 │ #5 のみ 1  「…」                                              │
 │ #3 のみ 2  「…」「…」                                          │
 │ 一致行を開くと、両 Run の abduction 4 段と evidence を左右並列 │
 └──────────────────────────────────────────────────────────────┘
```

### 4.5 Metrics タブ（時系列ダッシュボード）

- x 軸 = Run 通番、y 軸 = 各指標。1 指標 1 スパークライン（7 本）。
- Release が変わった Run に縦の**マーカー**を置き、ホバーで「何が変わったか」を表示。Input が変わった Run は別色のマーカー。
- 同一構成の反復は同じ x にドットを重ね、ばらつきをそのまま見せる。
- Research iteration に人手評価（novelty / overallUsefulness）があればそれも重ねる。指標は工程品質（グラウンディング率・反証探索率）であって真実性ではないので、人手評価と並べて初めて読める、という注記を常設する。
- プロジェクト横断ビュー（後述 Phase 4）: release fingerprint ごとに、全プロジェクトの指標分布を並べる。`make eval-demo` の固定ベンチマークをここに取り込むと「リリース × ベンチマーク」の回帰表になる。

### 4.6 Research タブ（iteration タイムライン）

現在は最新 iteration の promotion しか出ない。既存データで次を描ける。

```
 Q: Did the intervention cause the increase?
 ●─ it.1  EXPLORATORY   readiness EVIDENCE_INSUFFICIENT   gaps 3   run #2
 │        [handoff] [evaluation]
 ●─ it.2  VALIDATION    readiness VALIDATION_REQUIRED     gaps 2 (1 addressed)   run #3
 │        added evidence: comparison-period.csv → gap G-2
 │        Δ vs it.1: insights +1 / -0 · hypothesis H1 STRENGTHENED
 │        [compare it.1 → it.2] [handoff] [evaluation ✓ novelty PARTIALLY_NEW]
 ●─ it.3  …
```

- `compare` は既存エンドポイントをそのまま呼ぶ。
- 各 iteration に「その iteration の Insight を生んだ Run」へのリンクを付ける。Run 比較（エンジン軸）と iteration 比較（証拠軸）が相互に行き来できる。

### 4.7 Insight 詳細への追加

- 先頭に「Run #5 · gpt-5 · prompts v7 · 2026-09-24」のパンくず。
- 「この Insight の履歴」セクション: 対応付けられた過去 Run の同一 Insight（confidence 推移・flags の変化・validation status の変化）。
- ドメインにあるのに未表示の項目を出す: 競合仮説（PRIMARY / COMPETING）、causal / validation / identification status、missing evidence、反証基準、next validation、mechanism の各ステップと未裏付けブリッジ、generalization の境界条件。

### 4.8 Settings

- 「次の Run に記録される Release」を 1 カードで表示: engine version、rules、prompts version、model、provider host。Settings を変えた瞬間に「次回から Release が変わる」ことが分かる。

## 5. 必要な変更（最小構成）

### 5.1 ビルド・provenance

- `-ldflags "-X main.version=$(git describe --tags --always --dirty)"` でエンジン版を埋め込み、`/api/health` と `RunProvenance.engineVersion` に載せる。これがないと同一プロンプトの別バイナリを区別できない。
- `RunProvenance` に追加（既存 JSON への additive 追加なので migration 不要）: `engineVersion`, `promptVersion`（人間可読ラベル。`prompts.go` に定数を置き、fingerprint と併記）, `providerHost`, `inputFingerprint`, `documentCount`, `releaseFingerprint`。
- `ruleVersion` を単一文字列から、Confidence 式・品質ルール・グラウンディング正規化・事前解析の各版を持つ構造に拡張する（後方互換のため既存キーは残す）。
- Release スナップショットは **enqueue 時**に確定して `analyses` に保存する。現在は worker 起動時に Settings を読むため、キュー待ち中に設定を変えると意図と違う構成で走りうる。

### 5.2 DB（forward-only migration `007_run_labels.sql`）

```sql
ALTER TABLE analyses ADD COLUMN label TEXT;
ALTER TABLE analyses ADD COLUMN note TEXT;
ALTER TABLE analyses ADD COLUMN is_baseline INTEGER NOT NULL DEFAULT 0;
```

001〜006 は編集しない。

### 5.3 API

| 変更 | 内容 |
|---|---|
| `GET /projects/{id}/analyses` | DTO に `metrics` と `provenance`、`label` / `note` / `isBaseline` を含める |
| `GET /projects/{id}/insights`, `/patterns` | `?analysisId=` を受け付け、**既定を最新完了 Run に変更**（現在の合算は不具合として扱う。挙動変更なので README に明記） |
| `GET /projects/{id}/report.md` | 同上。`?analysisId=` で過去 Run のレポートも出せるようにする |
| `POST /projects/{id}/analysis` | body に `label`, `note` |
| `PUT /analysis/{id}/baseline` | baseline 指定（プロジェクト内で 1 件） |
| `GET /analysis/{a}/compare/{b}` | 帰属分類・Release 差分・Input 差分・指標差分・Insight 対応表 |
| `GET /projects/{id}/metrics-history` | Run ごとの指標 + provenance の配列（ダッシュボード用） |
| Pattern / Insight DTO | `analysisId` を含める |

### 5.4 Run 間の Insight 対応付け（決定論的・モデル不使用）

- Research iteration 間ではすでに `hypothesisComparisonKey`（タイトルの正規化）で対応付けている。Run 間でもまずこれを再利用する。
- タイトルはモデルが毎回言い換えるので、**証拠スパンの重なり**を主キーにする。Document は Run をまたいで同一 ID で残るため、Insight の evidence `(documentId, startOffset, endOffset)` 集合の Jaccard 係数が ≥ 0.5 なら同一候補、次点でタイトル一致。閾値と一致理由（「evidence 重なり 0.7」「タイトル一致」）を UI に表示し、対応付け自体も検証可能にする。
- 対応付けはあくまで候補であり、人手で「別物」に切り替えられるようにする（append-only のメモとして保存）。

## 6. フェーズ計画

| Phase | 内容 | 規模 |
|---|---|---|
| **0** | Insight / Pattern / report を `analysisId` で絞り込み（既定は最新完了）。解析パネルに provenance チップ（mode / model / prompt 指紋先頭 7 桁 / rules）。`/analyses` に metrics・provenance を含め、プロジェクト画面に Run 履歴リストを追加 | 1 PR。新画面なし |
| **1** | engineVersion（ldflags）、promptVersion ラベル、Release / Input fingerprint、Run の label / note / baseline（migration 007）。Runs タブと Run 詳細。既存の evaluation / patterns 画面を吸収。プロジェクト画面のタブ化と入力フォームの Evidence タブ移動 | 2〜3 PR |
| **2** | Run 比較画面（帰属分類・指標差分・Insight 対応表・反復範囲表示）。Insight 詳細の履歴セクションと未表示ドメイン項目の露出 | 2 PR |
| **3** | Metrics タブ（時系列 + release マーカー）。Research タブの iteration タイムラインと既存 compare / handoff / evaluation の UI 化 | 2 PR |
| **4** | プロジェクト横断の Release ボード。`make eval-demo` の結果をベンチマーク Run としてアプリ内に取り込み、golden set は正しさのゲート、Run 履歴は回帰の履歴、と役割を分ける | 設計次第 |

Phase 0 は動作変更（合算 → 最新 Run のみ）を含むので、`make test && make vet` に加えて、2 回実行後に一覧が最新 Run のみを返すテストを追加する。

## 7. 未検証・要確認事項

- **「リリース」の定義（§1）**が本提案の前提。ビルド版のことか、プロンプト / モデル構成のことか、あるいは分析対象データの版のことか、確認したい。
- LLM ありの Insight 合算は実測していない（コードからの推定、§2.1）。
- Insight 対応付けの閾値 0.5 は仮置き。実データで調整が必要。
- 反復実行のコスト（API 課金）は運用判断。反復なしでも比較画面は出すが、帰属の但し書きは消さない。
- タブ化に伴う既存 URL（`#/projects/:id/evaluation`, `#/projects/:id/patterns`）はリダイレクトで残す前提。
