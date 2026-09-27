# UI before / after — sample gallery and theme (#151, #131)

- **before**: origin/main `1cb1d47`. **after**: branch `feat/151-ja-demo-starter`.
- Captured with real binaries (delivery build + demo build) and the local scripted test model `cmd/insight-scripted-llm`. **No real LLM was used**: every hypothesis and insight title in these images ("Scripted insight" / 「スクリプトによるインサイト」) is the test double's fixed text, not an analysis result.
- `<variant>-<screen>.jpg`: variant = desktop-en / desktop-ja / mobile-ja (mobile-en: Home only), full page, JPEG-compressed. Screens: 01 Home (empty DB), 04 CSV preview, 07 analysis complete, 08 findings, 11 evidence (quote expanded), 09 report control (the report itself is a Markdown download; there is no in-app report view).
- `after/journey-demo-ja-*.jpg`: the Japanese E2E (`web/e2e/samples-ja.spec.ts`, `SHOTS_DIR=…`) — サンプル選択 → CSV プレビュー → 分析 → 発見 → 根拠 → レポート, plus the phone-width revisit.
- Regenerate: `SHOTS_DIR=/tmp/shots pnpm --dir web exec playwright test --project demo-ja --project demo-ja-mobile`.
