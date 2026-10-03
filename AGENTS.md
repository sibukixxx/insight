# Insight Lab

Evidence-first analysis engine. Keep observations, expectations, mismatches, competing hypotheses, supporting/counter evidence, limitations, and insights distinguishable and traceable.


<!-- techvit-development-cycle:start -->
## 開発サイクルと実装の粒度

開発中の未公開機能は、最も単純で明確な正本へ直接書き換える（KISS）。将来の利用者や要件を推測して互換レイヤー、移行コード、抽象化を先回りして追加しない。

1. **資料を先に更新する。** 変更に関係するREADME・設計・契約・手順を読み、必要な箇所を先に更新して矛盾と古い情報を解消する。変更規模に見合う既存資料を使い、小さな修正のために長大な設計書やADRを増やさない。現状と今回の変更後を区別し、未実装を実装済みと書かない。
2. **必要なテストを先に書く。** 振る舞いを変える場合は、利用者から見える結果や重要な境界を確認する最小の失敗テストを先に用意し、TDDで進める。既存テストを使えるなら再利用する。細かな内部実装をなぞるテスト、同じ保証の重複、推測上の極端な例外を網羅するテストは増やさない。資料だけの変更や低リスクな見た目の調整には、形式的な新規テストを要求しない。
3. **最小実装する。** 最小の縦断的ユースケースを完成させる。呼び出し側も一緒に更新できる未公開APIはまとめて変更し、旧・新の二重実装や使われないfallbackを残さない。開発用の使い捨てデータだけなら新しい正本・fixtureへ更新し、移行処理を作らない。
4. **最後に整理して確認する。** この変更で不要になったコード、分岐、設定、依存、テスト、手順を削除・更新する。参照元と資料の矛盾を確認し、必要なテスト・lint・buildを実行する。未実施の確認は明記する。無関係な大掃除は同時に行わない。

互換・移行が必要なのは、実際の外部利用者、公開済み契約、本番・継続利用の保存データ、オンチェーン状態などを守る場合、またはユーザーが明示した場合。対象と理由を確認して最小限にする。存在を確認できないという理由だけで保存データを削除・初期化してよいと判断しない。

この方針は互換・移行を無条件に求める旧来の記述を上記の必要性に限定する。既存のセキュリティ、認可、秘密情報、個人情報、会計・証跡、不可逆操作、保存データ保護の境界や必須チェックを弱めるものではない。事故時の影響が大きい境界は、発生頻度が低くても必要な検証を維持する。履歴・監査証跡や既に適用した移行を、不要コードと同じ扱いで書き換えない。
<!-- techvit-development-cycle:end -->

## Commands
- `make build` — delivery build without demo data
- `make build-demo` — demo-tag build
- `make test` — normal + demo-tag tests
- `make test-golden` — golden evaluation harness
- `make vet`
- `make eval-demo` — real-LLM evaluation; requires configured API credentials
- `make web-build` / `make build-all` — rebuild `internal/web/dist` from `web/` (Node 22+, pnpm) / plus the Go binary
- `make web-check` — frontend typecheck, lint, unit tests, and stale-`dist` detection
- `make web-e2e` — Playwright E2E against real delivery/demo binaries with the local scripted model (no paid LLM)

## Shared rules
- Never present an observation, anecdote, or model output as a validated insight without its evidence state and limitations.
- Delivery builds must not embed demo/sample data; preserve the build-tag separation in `internal/sampledata/`.
- Keep evidence and counter-evidence history reproducible; do not overwrite prior evidence to make a hypothesis look cleaner.
- Do not run real-LLM evaluation merely as a completion check when credentials/cost are not part of the task.

## Change-dependent checks
- Core/domain changes: `make test && make vet`.
- Insight scoring/evaluation changes: also run `make test-golden`.
- Demo-only changes: verify both normal and `demo` build paths remain separated.
- Browser UI changes: edit `web/` only (never hand-edit `internal/web/dist`), keep the onion layer rules in `docs/frontend-architecture.md`, add en/ja dictionary keys together, then `make web-check` and commit the rebuilt `dist`; run `make web-e2e` for flow changes. No sample data under `web/public` (it would ship in delivery builds).
- Public contract changes (`contracts/public-engine/v1`, `contracts/analytical-artifact/v1`): `make test` (drift and conformance tests), update `docs/public-engine-contract.md` in the same PR, then resync the snapshots in `insight-sdk-go` and `insight-sdk-js` (`contract/PROVENANCE.md`) and release a new SDK minor version. The SDK repositories never lead; this repository is authoritative.

## Done
- Changed analysis behavior is covered by tests or golden fixtures as applicable.
- Delivery build remains free of demo data.
- Unverified claims and checks that could not run are explicitly reported.
