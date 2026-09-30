# UI terminology (ja / en)

Issue: #141. This is the single glossary and translation review table for the
standalone Workspace UI (`web/`). It records which display words the browser
uses for each internal concept, why, and what the words must not imply.

Scope rules:

- It governs **display copy only** (`web/public/locales/{en,ja}.json`). API
  values, Go types, schema, the public contracts, CLI flags, URLs, route IDs,
  `localStorage` keys, option values and persisted IDs keep their exact
  technical names. Raw codes stay inspectable (data attribute, `<code>`,
  Advanced/詳細 disclosures, CSV column values).
- Quoted evidence, source text and model output are never rewritten.
- UI locale, the run's `outputLocale` (model-written text), source language and
  export language are independent. Labels must read well for ja UI + en-US
  analysis and en UI + ja-JP analysis.
- An unknown, incomplete or unverified state never reads as ready, verified or
  successful. No label says 「検証済み」 / "verified" unless the server records a
  verification.
- Changes are reviewed term by term, never by repository-wide search/replace.
- Label tables for server codes live in `web/src/domain/codes.ts` and are
  rendered through `I18nProvider.label`; unknown codes are shown verbatim.

Classification used in the tables:

- **Replace** — replace with common language in the primary UI.
- **Explain** — keep the formal term, with a concise explanation nearby.
- **Keep** — keep unchanged (brand, source, model name, file format, filename,
  raw code a user must type).

Status of the candidates proposed in the issue: **adopted**, **adjusted**
(with the reason), or **deferred**.

## Glossary

| Canonical concept | Raw code / key | ja display | en display | Helper copy | Semantic caution | Class |
|---|---|---|---|---|---|---|
| Analysis run | `AnalysisRun`, `run.*`, `runs.*` | 実行（見出しは「{日時} に完了した実行」） | run ("Run finished {date}") | The run ID is shown under 詳細 as 「実行 ID: …」, on `data-run-id`, and in the run selector options | "Completed" means processing finished, not that results are valid. Raw IDs never head a card | Replace |
| Selected run | `run.selectorLabel`, `project.insightsForRun` | 結果を表示する実行 / 表示中の実行のインサイト | Showing results of run / Insights from the run shown | — | Only completed runs can be selected; others are listed disabled | Replace |
| Research run | `ResearchRun`, `research.*` | 研究実行 | research run | 完了した分析から作り、人によるレビューに進める | A research run is not a publication and not proof | Replace |
| Research iteration | `iterationSequence` | 反復 {n} 回目 | Iteration {n} | — | Review approval and publication are separate actions | Replace |
| Evidence | documents, evidence spans | 根拠（資料） | evidence | — | Do not confuse with hypotheses, model reasoning or citations. 「エビデンス」 is no longer used in ja copy | Replace |
| Document | `EvidenceDocument` | 文書 | document | — | One imported row or pasted text; keeps its source text | Replace (was mixed ドキュメント/文書) |
| Generic source types | `document` `report` `paper` `web` `record` `other` `dataset` | 文書 / レポート / 論文 / Web 由来のテキスト / 記録 / その他 / データセット | Document / Report / Paper / Web-derived text / Record / Other / Dataset | Grouped as 一般的な資料 / General sources in the source select | `dataset` = structured or pre-aggregated data; the only kind analysed without a model | Keep |
| Customer and market research source types (valid since v1) | `interview` `review` `support` `sales` `survey` `job_posting` `social_post` | インタビュー / 顧客レビュー / サポート対応 / 商談 / アンケート / 求人・案件の募集文 / SNS 投稿 | Interview / Customer review / Support conversation / Sales call / Survey / Job or project posting / Social media post | Grouped as 顧客・市場調査向け / Customer and market research (`<optgroup>`; option values unchanged). CSV hint names the raw values | Previously suffixed 「（旧形式）」/"(legacy)", which exposed internal versioning. They remain fully valid inputs. `review` is a customer review, not the human publication review | Replace |
| Dataset profile (Data Triage profiler) | `triage.Profile`, `input.preview.profile*` | 列ごとの値の概要（データセットプロファイル） / 列の概要（Data Triage 用のプロファイル） | column summary (dataset profile) / Column summary (Data Triage profile) | Inside the 詳細 / Advanced disclosure of the import preview | **Adjusted** from the issue's 「分析に使う項目の選別」: the preview shows only the per-column profile (type, filled, empty, distinct, sample values). No selection happens here | Replace, formal name kept in parentheses |
| Data Triage selection plan | `triage.Plan`, buckets `INCLUDE` `DEFER` `NEEDS_REVIEW` `EXCLUDE` | 分析に使う項目の選別（候補） | Choose relevant data (candidate) | Not surfaced in the browser UI yet | `EXCLUDE` never deletes source data; an AI proposal is unapproved until a human accepts it. **Adopted** as wording for when the plan is shown | Replace (future) |
| Research artifact | `GET /api/research-runs/{id}/artifact.json` | 現在の調査結果の記録（JSON） | Current research record (JSON) | Versioned JSON export of the latest iteration | Hashes and provenance do not make it verified or true. **Adopted** 「調査結果の記録」 | Replace |
| Approved research artifact | `approved-artifact.json`, `approvedArtifactReference` | 承認済みの調査結果の記録（JSON） | Approved research record (JSON) | Fixed copy taken at human approval; its reference stays visible as `<code>` | Approved for publication by a reviewer, not externally validated | Replace |
| Analytical artifact | `contracts/analytical-artifact/v1` | 出典付き集計結果（候補） | Aggregated results with provenance (candidate) | Not surfaced in the browser UI | **Deferred**: no screen shows it yet. Must not be called verified because it carries hashes | Replace (future) |
| Reasoning profile | `ReasoningProfile`: `GENERAL_RESEARCH`, `CUSTOMER_INSIGHT` | 分析の視点（一般リサーチ / 顧客インサイト） | Research approach (General research / Customer insight) | 顧客インサイトは顧客調査向けのチェックを追加する | Independent of execution mode and execution profile. **Adopted** | Replace |
| Execution profile | `ExecutionProfile`: `LIGHT` `STANDARD` `HEAVY` `AUTO` | 処理規模・実行設定（候補） | Processing mode (candidate) | Not surfaced in the browser UI | **Deferred**; codes never change | — |
| Execution mode | `deterministic`, `model_backed` | 決定的（モデル不使用） / モデル使用 | deterministic (no model) / model-backed | ja copy now always says 「モデルを使わない決定的な方法」 instead of mixing 決定論的/決定的 | Deterministic runs analyse dataset documents only | Replace (explained) |
| Run provenance | `RunProvenance`, `provenance.*`, `advanced.*` | 実行条件の記録（来歴） | run provenance | Engine, model, prompt fingerprint, rule version, input and question; always inside 詳細 / Advanced | "Not recorded" is shown as 未記録, never as empty, zero or "same". **Adjusted** from 「出典・処理履歴」 because this record is about how the run was produced, not about the sources. en keeps "provenance" | Replace (ja) / Explain (en) |
| Sample provenance (#153) | `samples.provenance.*` | 出典・取得・変換の記録 | Source, retrieval and transformation record | — | Describes where sample data came from; not a result | Keep (#153) |
| Pre-run readiness | `readiness.status`: `ready` `blocked` `running` `unknown` | 入力チェック / 実行可能 / 実行不可 / 実行中 / 状態は不明 | Input checks / Ready to start / Blocking / Running / state is unknown | `data-readiness` keeps the raw status | `unknown` never reads as ready; the raw 「（UNKNOWN）」 was removed from copy but the text still says the state is unknown | Replace |
| Decision readiness | `decisionReadinessHonestlyStated` | 意思決定に使える段階か | decision readiness | Promotion checklist item | Distinct from pre-run input checks. The issue's 「判断への利用準備状況」 applies here | Explain |
| Promotion / publication review | `PromotionState`, `promotion.*` | 公開に向けた確認 / 公開状態 / 公開判定のチェックリスト | Publication review / Publication state / Publication checklist | — | Research validation and the publication decision stay separate. en "Promotion checklist" was replaced | Replace |
| Promotion states | `DRAFT` `RESEARCH_COMPLETE` `HUMAN_REVIEW_REQUIRED` `PUBLICATION_READY` `PUBLISHED` `REJECTED_FOR_PUBLICATION` | 下書き / 調査完了 / 人手レビューが必要 / 公開可能 / 公開済み / 公開見送り | Draft / Research complete / Human review required / Ready for publication / Published / Rejected for publication | — | 公開可能 is a review outcome, not validation | Keep |
| Customer-research insight fields | `statedNeed`, `jtbd`, `productOpportunity`, `monetizationAngle` | 表明されたニーズ / JTBD・片付けたい用事（顧客調査向けの項目）、プロダクト機会 / 収益化の観点（顧客調査向けの参考項目） | Stated need / Job to be done (JTBD) (customer-research field), Product opportunity / Monetization angle (customer-research reference field) | Hidden when empty | Were labelled 「（旧形式）」 and shown as "-" when empty, which looked like a value. Other empty insight fields read 未記録 / not recorded | Replace |
| Customer-research quality checks | `stated_need_echo`, `generic_term` | 表明されたニーズの言い換え / 抽象的なニーズ表現（顧客調査向け） | Restates the stated need / Generic need language (customer research) | 顧客調査向けのチェック。表明されたニーズがある場合にのみ適用 | Hints for the researcher, not verdicts | Replace |
| Supported inputs | `GET /api/import-formats`, `input.formats.hint` | 取り込めるのは、下に示す CSV 形式（それぞれテンプレートあり）と、貼り付けたテキストです。… | You can import the CSV formats listed below (each has a template) or paste text. … | — | Replaces the placeholder-like 「対応している入力: このサーバーが受け付ける形式です」. Only server-reported formats are offered | Replace |
| Run comparison attribution | `SAME_CONFIGURATION` `EXECUTION_CHANGE` `INPUT_CHANGE` `CONFOUNDED` `ATTRIBUTION_UNAVAILABLE` | 差分の分類（記録上の分類であり、原因ではありません） | Attribution (bookkeeping, not cause) | — | Never causal, never a ranking | Replace (ja) |
| Axis state | `SAME` `CHANGED` `UNKNOWN` | 同一 / 変化あり / 不明（未記録） | Same / Changed / Unknown (not recorded) | — | `UNKNOWN` is not "same" | Keep |
| Research gap | `ResearchGap` | まだ分かっていないこと（研究上のギャップ） | research gaps | — | A gap is missing knowledge, not a failure | Explain |
| Insight | `Insight` | インサイト | insight | 各発見は根拠の状態を伴う仮説 | Never a validated conclusion without its evidence state and limitations | Explain |
| Finding | findings page | 発見 | finding | — | A hypothesis with its evidence state | Explain |
| Observation / hypothesis / counter-evidence / confidence | — | 観察 / 仮説・説明仮説 / 反証 / 確信度 | observation / hypothesis / counter-evidence / confidence | — | Confidence is the run's score, not a probability of truth | Explain |
| Trace / pattern | `deviation`, `repetition` | ズレ（期待・基準との食い違い） / 繰り返しのパターン | trace (expectation mismatch) / recurring pattern | — | Explanations of a mismatch remain hypotheses. 「ズレ」 matches the insight page wording (「ズレ・意外な事実」) | Replace (ja; en keeps "trace") |
| Output locale | `outputLocale`: `""` `ja-JP` `en-US` | モデルが書くテキストの言語 / 自動（資料・質問の言語） | Language of model-written text / Automatic | — | Never translates evidence or quotes; separate from the UI language | Keep (#131) |
| Model connection | settings | AI モデルの接続設定 / 接続先 URL（OpenAI 互換 API のベース URL） / モデル / API キー | AI model connection / Base URL (OpenAI-compatible endpoint) / Model / API key | — | Was 「LLM 設定」 / "LLM settings" | Replace |
| Brand, formats, identifiers | — | Insight Lab, CSV, PDF, Excel, Markdown, JSON, URL, API, AI, SNS, Web, `en-US`, `ja-JP`, model names, filenames, CSV column names and source values | same | — | Kept verbatim | Keep |

## Labels adopted by the #151 gallery (PR #153)

Dictionary keys `samples.*`, `home.hero.*` and `input.sample.*`; ja/en were
added together. Kept as is.

| Concept | ja | en |
|---|---|---|
| Entry to samples | サンプルで試す | Try a sample |
| Own-data entry | 自分の CSV を取り込む | Import your own CSV |
| Gallery question | 何を調べてみますか？ | What would you like to investigate? |
| Data kind | 架空データ / 実データ（公的統計） / 混合（公的統計＋架空メモ） | Synthetic data / Real data (official statistics) / Mixed (official + fictional memos) |
| Card action | この例で試す | Try this example |
| Provenance panel | 出典・取得・変換の記録 | Source, retrieval and transformation record |

## Audit of the dictionaries (first slice)

All 522 keys of `web/public/locales/ja.json` and `en.json` were read side by
side. Latin-script words left in ja copy after this slice are brand names,
file formats, identifiers and abbreviations in the Keep row (CSV, AI, URL,
API, SNS, Web, JTBD alongside its Japanese gloss, Data Triage inside a
parenthesis of the Advanced profile). No ja string contains "Run" or
「旧形式」 any more.

Four defects observed after #153 were fixed:

1. 「Run」 in ja copy (「表示中の Run」, 「インサイト — Run ana_…」, 「Run ana_… ·
   完了」) and raw run IDs in headings.
2. 「インタビュー（旧形式）」 and the other "(legacy)" source labels.
3. 「表明されたニーズ（旧形式）」「JTBD（旧形式）」 rendered with "-".
4. 「対応している入力: このサーバーが受け付ける形式です」.

## Settled in the second slice (#141, with #165)

- Navigation and page titles: 「痕跡とパターン」 → 「ズレとパターン」 (ja only;
  the badge 「痕跡」 → 「ズレ」). English keeps "Traces and patterns".
- ja uses 「取り込む／取り込み」 for every import action; 「インポート」 is gone
  (buttons 「取り込む」, 「CSV を取り込む」, 「分析データを取り込む」). The API, CSV and
  route names keep `import`.
- The publication state heading shows only the label. The raw code is in the
  heading's `data-state` attribute and `title`.
- Home follows the real project state (no stored first-visit flag): with no
  project it shows the introduction and the sample gallery; with projects it
  shows the project list first, drops the lead sentences and keeps samples
  behind an opt-in 「サンプルを見る」 / "Show samples" button
  (`aria-expanded`, `aria-controls`). Sample cards show title, data kind,
  question, caution and the actions; input, what it teaches and sources are in
  a 「詳細・出典」 / "Details and sources" disclosure (native `<details>`, keyboard
  reachable). Duplicated introductions removed on a returning Home: the hero
  lead, the gallery lead and, per card, three of four fact rows.

## Pending (later slices)

- Evaluation metric names (「カバー率」, "Trace-backed Insights").
- README.ja.md and the user guide: use the display terms above with the
  original term on first mention.
- Cross-check generic concepts with TechVit (techvit-insight #70); private
  business terms stay private.
