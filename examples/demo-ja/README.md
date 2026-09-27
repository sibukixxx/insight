# 日本語デモの最小サンプル (#151)

このフォルダは **インポート仕様に適合する例** と公式統計の取得先を明確に区別します。01〜03の値・文章はすべて架空データです。**公的統計の実数は同梱していません。** 04は公式取得先の参照であり、`REFERENCE_ONLY_NOT_FETCHED` と明記しています。

| File | Current supported use |
| --- | --- |
| `01-ja-shop-records-documents.csv` | 現行ブラウザの「文書CSV」→ プレビュー → インポート → 調査。固定列 `id,source,title,content` (UTF-8) |
| `02-ja-composite-documents.csv` | 同じ文書CSV。複数の架空資料の調査例。2025年人口と2021年事業所数を同時点の比率にしない |
| `03-numeric-raw-aggregation.csv` | **通常の文書CSVには投入不可**。対応済みRAW_ARTIFACT `csv-aggregate/v1` の明示的な `-input-root` 設定、または別製品TechVit Insightのmanaged CSV producer向け。空欄は欠損で0ではない |
| `04-official-source-catalog.json` | e-Statの公的統計取得候補。現時点ではリンクのみで実数未取得 |

## 初回の3操作

1. Insightのプロジェクト画面で `01-ja-shop-records-documents.csv` を文書CSVとしてプレビューし、取り込む。
2. 問い「売上が増加したという観測を、何が説明できて何がまだ分からないか？」を入力する。
3. モデルが設定されていれば分析を実行し、原文への根拠、対立する説明、データ不足を確認する。モデル未設定時はモデル生成の説明を使えると表示しない。

複合的な調査例は `02` を別プロジェクトに取り込み、資料ごとの時点・定義・未取得情報の違いを確認する。**Cross-Dataset Discovery #118 を実装済みと装わない**。

## 次の段階（このPRの範囲外）
- #151で実データのe-Stat CSV取得・出典/ハッシュ/ライセンス固定と、デモビルド専用の選択ギャラリーを追加。
- #131が使いやすいブラウザ取り込み・進捗・分析・成果確認の実経路を担当。
- private TechVit Insight #54が同じ日本の公的データを managed Selection Plan → sealed Analytical Artifact → note/顧客レポートに利用。
- サンプルを `web/public` に配置しない。公開用/納品用バイナリへ架空デモが混入しないことを実装時にテストする。
