# 検証計画: インサイト流派の系譜から見た Insight Lab の改善点

作成: 2026-09-03
出典: 松本健太郎 note 質問箱「インサイトの流派を体系的・系譜学的に知りたいです」（2026-09-03）
関連: docs/detailed-design.md §7 / §15 / §16 / §23（§23 は同著者の 9/2 COMEMO 記事を参照済み）

## 出典が言っていること（原文はこの 4 文のみ）

1. 九州単品通販流と P&G 流の 2 つが「始まりの呼吸」
2. 他のすべての流派はここから派生した
3. 九州単品通販流は紙 / TV / D2C の呼吸に分かれる。系譜を知るなら紙の呼吸から
4. P&G 流は「インサイトが大切」と言うが、見つけ方の言語化が上手い人は書籍を出していない

方法の中身は書かれていない。Insight Lab の 予想 → ズレ → アブダクション は同著者の方法なので、この投稿自体が直接の変更を要求するわけではない。
以下の「各流派の検証の作法」（通販流 = 反応で検証、P&G 流 = コンセプトテストや読み返しで検証）は一般知識であり、投稿の記述ではない。

## 系譜のレンズで見えた構造的な差

インサイトの定義は「人を動かす無自覚な欲求」だが、現状のパイプラインは「Evidence に支持されているか」「定義に反していないか」までしか検証しない。「人を動かすか」は検証されていない。その前提で、検証可能な仮説に落としたものが下表。

| # | 仮説 | 根拠（コード） | 検証方法 | 合否基準 |
|---|---|---|---|---|
| H1 | 一覧の並び順が、方法論が重視する痕跡ベースの Insight を構造的に埋もれさせている | `internal/repository/sqlite/insight.go:47` は `ORDER BY confidence DESC`。Confidence の 45% が広さ（coverage 0.25 + frequency 0.20）、20% が source 多様性。20 件のプロジェクトで N=1 痕跡 Insight の上限 ≈ 0.44、10 件に繰り返された顕在ニーズの言い換えは ≈ 0.75 | ベースライン eval の insights.json から、順位・confidence・trace-backed・flags を並べる | 上位 3 件に `stated_need_echo` 付きがあり、かつ警告なし trace-backed Insight が 4 位以下にあれば「確認」 |
| H2 | 「surprisingFact に新しい事実を加えない」ルールがプロンプト頼みで、アプリ側で検証されていない | `AssessQuality` は空チェックのみ。quality.go の「model proposes, app checks」に反する | insights.json の各 Insight について、surprisingFact と引用 Pattern の observations（quote / behavior）の bigram 類似・包含を計算し、根拠なし率を出す（既存の normalizeForCompare / bigramSimilarity を流用可） | 根拠なし率が 20% を超えるなら `surprising_fact_ungrounded` フラグを TDD で追加 |
| H3 | Quality Flags が人間の「粗悪品」判定と一致するか未計測 | §23.3 は設計のみ。docs/evaluation は README しかない | ベースラインの各 Insight をリサーチャーが 3 値で判定（認識 / 既知 / 抽象）し、フラグとの一致率を出す | 一致率 70% 未満、または人間が「認識」とした Insight に警告が付く例が複数あれば閾値・語彙を見直す |
| H4 | ドメイン非依存の主張が 通販流（紙の呼吸）型のテキストで未検証 | デモデータは B2B SaaS の interview 14 / review 3 / support 2 / sales 1 | 架空の健康食品通販「お客様のはがき」20 通を作り、痕跡を意図的に埋め込む（excess_payment / persistence / absence / contradiction 各 2〜3 件）。§16 の Golden Dataset をここで初めて実装する | 埋め込んだ痕跡の再現率 70% 以上。短文（80〜200 字）で grounding 破棄率が上がらない |
| H5 | 「人を動かすか」の検証ステップが存在しない | Insight に検証用の一文（読み返し文 / 紙の呼吸の見出し相当）も、検証結果を記録する場所もない | H3 の人手判定を先に行い、必要性を確認してから設計する | H3 で「フラグは通るが読み返すとピンとこない」例が出れば、読み返し文の生成と結果記録を機能化する |

補足: `confidence.go` の `sourceDiversitySlots = 5` に対して SourceType は 7 種。clamp01 で 1.0 に丸まるので不具合ではないが、設計上の意図は確認したい。

## フェーズ

### Phase 0: ベースライン取得（ユーザー実行、コード変更なし）

```
INSIGHT_LAB_API_KEY=... INSIGHT_LAB_MODEL=gpt-5 make eval-demo
```

- 20 件のドキュメントに対して数十回の LLM 呼び出しが発生する
- `docs/evaluation/<日付>-<モデル>/` に metrics.json / patterns.json / insights.json / summary.md が出る
- できれば 2 モデルで取り、モデル差と構造差を分離する

### Phase 1: オフライン分析（LLM 不要、1〜2 時間）

- `scripts/eval-summary.py` に順位表（rank / confidence / trace-backed / flags）を追加 → H1
- surprisingFact の根拠率を出すスクリプトを追加 → H2
- 両方とも Phase 0 の JSON だけで動く

### Phase 2: 人手キャリブレーション（ユーザー、1 時間程度）

- Phase 0 の Insight 一覧を 認識 / 既知 / 抽象 で判定 → H3
- 判定結果は docs/evaluation の同ディレクトリに human-labels.json として残す

### Phase 3: 判断ゲート

Phase 1〜2 の数値で、次のどれを実装するか決める。実装は各々 TDD で。

- H1 確認 → 並び順を「警告なし・痕跡あり」優先にするか、痕跡の強さを別軸として表示する
- H2 確認 → `surprising_fact_ungrounded` フラグ追加（quality_test.go から）
- H3 不一致 → 閾値 0.5 / 語彙リスト / minEchoRunes の調整
- H5 必要 → 読み返し文フィールドと検証結果の記録

### Phase 4: 通販流フィクスチャと Golden テスト（2〜3 時間）

- `internal/sampledata/testdata/` に 2 つ目のデータセット
- `go test -tags=golden` で手動トリガ（§16 の設計どおり、コスト管理のため CI には入れない）
- 埋め込み痕跡の一覧を期待値として持ち、再現率を出す → H4

## やらないこと

- 出典が方法を書いていない以上、P&G 流の形式（欲求 + 障壁のテンション文）を今の段階で取り込まない
- 数値が出る前にプロンプトや重みを変えない
