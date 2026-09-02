# Contributing to Insight Lab

Issue や Pull Request を歓迎します。始める前にこのドキュメントに目を通してください。

## 開発環境のセットアップ

必要なもの: Go 1.25 以上（`go.mod` 参照）。CGO 不要（`modernc.org/sqlite` を使用しているため、クロスコンパイルも `CGO_ENABLED=0` で完結します）。

```bash
git clone https://github.com/sibukixxx/insight.git
cd insight
make build-demo
./bin/insight-lab-demo --demo
```

## テストと検証

PR を出す前に以下を実行し、通ることを確認してください（デモ/納品ビルドの両 build tag を通します）。

```bash
make vet
make test
```

## テストの書き方

このプロジェクトは t-wada 氏の TDD スタイルに従います。

- 実装コードより先にテストを書く（Red → Green → Refactor）
- 1 つのテストで検証するのは 1 つの振る舞いだけにする
- 外部依存（LLM API、ファイルシステムなど）のみをテストダブルに置き換え、それ以外は本物を使う
- テスト名は「何を・どういう条件で・どうなるか」が分かるように書く（例: `TestExtractInsightsShouldReturnEmptyWhenNoObservations`）

## コミットメッセージ

[Conventional Commits](https://www.conventionalcommits.org/) 形式に従ってください。

```
<type>: <subject>

<body（任意）>
```

`type` は `feat` / `fix` / `docs` / `refactor` / `test` / `chore` / `perf` などの英語表記を使い、`subject` は日本語・英語どちらでも構いませんが、リポジトリ内で書式を揃えてください。

## Pull Request の出し方

1. Issue がある場合は先にリンクしてください（ない場合は PR 内に背景を書いてください）
2. 変更は 1 つの関心事に絞ってください。複数の関心事が混ざる場合は PR を分割してください
3. `make vet` と `make test` がローカルで通ることを確認してください
4. UI に影響する変更は、スクリーンショットまたは動作確認手順を PR に記載してください
5. 破壊的変更（既存 API・DB スキーマ・CLI フラグの互換性を崩す変更）を含む場合は、その旨を PR の説明に明記してください

## DB スキーマを変更する場合

スキーマ変更は必ず migration として追加し、一度マージされた migration ファイルは編集・削除せず forward-only で扱ってください。詳細は `docs/detailed-design.md` の DDL / マイグレーション方針を参照してください。

## バグ報告・機能要望

GitHub Issues を使用してください。Issue テンプレートに沿って、再現手順・期待する動作・実際の動作を記載していただけるとスムーズです。

## セキュリティ上の問題

脆弱性を発見した場合は Issue を立てず、[SECURITY.md](SECURITY.md) の手順に従って報告してください。

## 行動規範

このプロジェクトへの参加者は [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) に同意したものとみなされます。

## ライセンス

コントリビュートしたコードは、本プロジェクトの [LICENSE](LICENSE)（Apache License 2.0）の下でライセンスされることに同意したものとみなされます。
