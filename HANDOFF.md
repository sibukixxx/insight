# 引き継ぎドキュメント

このプロジェクトを初めて引き継ぐ人向けの入り口です。**最初にこのファイルだけ読んでください。** 詳細は下記の順に読み進めれば、設計判断の背景からコードの構造、実際に動かす手順まで一通り把握できます。

## これは何か

Insight Lab / Hidden Needs Finder。顧客インタビューやレビュー、案件募集文などのテキストから、本人も言語化できていない「隠れたニーズ」を、AIの出力を鵜呑みにせず原文照合しながら見つけるツール。Go単一バイナリ + ブラウザUI + SQLite + OpenAI互換LLM。詳しくは [docs/blueprint.md](docs/blueprint.md) へ。

## 読む順番

1. **[docs/blueprint.md](docs/blueprint.md)** — なぜ作っているか、誰のためか、今どこまで動くか、何が未検証か（10分）
2. **[docs/architecture.md](docs/architecture.md)** — コードの地図。ディレクトリ構成、分析パイプライン全体図、データモデル、「〜したい時はここ」チートシート、絶対に壊してはいけない不変条件（15分）
3. **[docs/operations.md](docs/operations.md)** — ビルド・起動・デプロイ・トラブルシューティングのRunbook（必要な時に参照）
4. **[docs/detailed-design.md](docs/detailed-design.md)** — 設計判断とその変更履歴の詳細ログ（実装中に何が・なぜ変わったかを知りたい時に）
5. **[docs/design-review.md](docs/design-review.md)** / **[docs/implementation-plan.md](docs/implementation-plan.md)** — 初期設計の検証記録とフェーズ別進捗チェックリスト

## 今の状態を一言で

Phase 1〜3が完了し、**実際に動く**。フェイクLLMサーバーを使った実HTTP経由のパイプライン実行、Playwrightによるブラウザ操作まで含めて検証済み。ただし**実際のOpenAI APIキーでの動作確認はまだ行っていない**（構造はOpenAI Chat Completions API準拠だが実キーでのテストは未実施）。実務での利用実績もゼロ。詳細は blueprint.md の「現在地」「まだ検証されていないこと」を参照。

## 今すぐ気をつけること（Gotchas）

- **`.github/workflows/*` は現状リポジトリに反映されていない可能性がある。** セットアップ時のGitHub AppトークンにOAuthの`workflows`スコープがなく、pushが拒否された。`.gitignore`で除外中。対処法は [docs/operations.md §5](docs/operations.md) を参照し、引き継ぎ後の最初の作業として解消すること。
- **`internal/repository/sqlite/migrations/001_init.sql` を直接編集する運用は、このプロジェクトがどこかに配布・リリースされた後は絶対にやめること。** 現状は実ユーザーがいない前提で直接編集してきたが、以降は新規マイグレーションファイルを追加する方式に切り替える（[docs/operations.md §8](docs/operations.md) 参照）。
- **APIキーはメモリ内のみで永続化しない設計。** `internal/service/settings.go` にDB保存やファイル保存のコードを足さないこと。
- **Evidence・Observationの引用は必ず `internal/service/grounding.go` の `Ground()` を通す。** これを飛ばすと「AIが存在しない発言を引用する」という、このプロダクトが最も避けたい失敗が起きる。

## 最初の一歩（ローカルで動かす）

```bash
git clone <このリポジトリ>
cd insight
make build-demo
./bin/insight-lab-demo --demo
```

ブラウザが自動で開く。「デモを試す」→「解析を実行」で、LLM未設定のままだと `LLMが設定されていません` のエラーになるのが正常（設定画面からBase URL/Model/API Keyを入れると動く）。動作確認だけしたい場合は、実LLMの代わりにOpenAI互換のフェイクサーバーを自分で立てて `--base-url` に向ける方法もある（このセッションでの検証で実際に使った手法。プロンプトの `response_format.json_schema.name` を見て各ステップに応じたJSONを返すだけの小さなHTTPサーバーで十分）。

## 質問が出たら

- 「なぜこの設計？」→ [docs/detailed-design.md](docs/detailed-design.md)（`[変更]`マーカーの箇所に理由が書いてある）
- 「これは動くのか？」→ [docs/blueprint.md](docs/blueprint.md) の「まだ検証されていないこと」を先に確認
- 「どこを直せばいいか分からない」→ [docs/architecture.md](docs/architecture.md) のチートシート
