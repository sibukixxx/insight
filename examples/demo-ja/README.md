# 日本語デモのサンプル (#151)

ブラウザの Home 「何を調べてみますか？」から選べる3つのサンプルの元データです。**デモビルド（`make build-demo`）にだけ埋め込まれ、納品ビルドには含まれません**（`internal/sampledata/scenarios_delivery_test.go` がバイナリを検査します）。

| サンプル | データ | 入力 | 置き場所 |
| --- | --- | --- | --- |
| 01 架空店舗の記録から、変化と足りない情報を調べる | 架空データ（9行） | 文書CSV `id,source,title,content` | `internal/sampledata/scenarios/ja-shop-records/` |
| 02 日本の公的統計から、人口の変化を調べる | 実データ：国勢調査 総人口（茨城県 水戸市・常陸太田市・つくば市、2015/2020）（8行） | 文書CSV | `internal/sampledata/scenarios/ja-official-population/` |
| 03 人口と事業所数を合わせて、比べられること・比べられないことを調べる | 混合：国勢調査 2020 ＋ 経済センサス 事業所数（民営）2016/2021 ＋ 架空メモ3件（12行） | 文書CSV | `internal/sampledata/scenarios/ja-population-establishments/` |

各ディレクトリの `scenario.json` に、データ種別、入力CSVの sha256、出典（作成機関・系列コード・地域・期間・単位・取得日時・ライセンス・元ファイル sha256）、変換方法、限界を記録しています。公的統計の取得と変換は [`official/README.md`](official/README.md) を参照してください（`node examples/demo-ja/official/build.mjs --check` でオフライン再現を確認できます）。

## ブラウザでの手順（README や CLI は不要）

1. デモビルドを起動し、Home の「何を調べてみますか？」でサンプルの「この例で試す」を押す（専用プロジェクトが作られ、2回目以降は同じプロジェクトを開く）。
2. 入力画面の「この CSV をプレビュー」で、自分のファイルと同じ「プレビュー → インポート」を行う。
3. 分析を実行し、結果・根拠（原文の引用）・限界を確認する。

3つとも**文書CSV**のため、分析の実行には AI モデルの接続が必要です。モデル未接続でもプレビュー・取り込み・出典の確認はできます。モデルなしで分析まで試せるのは、分析用CSV（法人の登録・変更記録）の決定論的集計だけです。

## 通常の文書CSVとして扱えないもの

`03-numeric-raw-aggregation.csv`（`year,region,amount`）は**文書CSVにも分析用CSVにも投入できません**。対応済み RAW_ARTIFACT `csv-aggregate/v1` の明示的な `-input-root` 設定、または別製品 TechVit Insight の managed CSV producer 向けの例です。空欄は欠損で 0 ではありません。ギャラリーには載せていません。

Draft 時点にあった架空の複合調査 CSV（`02-ja-composite-documents.csv`）と取得前カタログ（`04-official-source-catalog.json`）は、実データを使うサンプル 02/03 に置き換えたため削除しました。
