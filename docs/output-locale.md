# Output locale for model-generated research text (#125)

Status: decided and implemented in Public Engine Contract v1 as an additive, optional field.

## Question

Callers want new model-generated explanatory text in Japanese or English, independently of the UI language and of the existing axes (`AnalysisMode`, `ReasoningProfile`, `ResearchStage`, `ExecutionMode`, `ExecutionProfile`). Evidence and research state must stay authoritative and reproducible.

## Decision

| Option | Decision | Reason |
|---|---|---|
| **Generation-time locale on `startAnalysis`** | **Adopted** | The only place the model writes research text is the analysis pipeline. Research runs and iterations are projections of a completed analysis and never call a model, so they inherit the locale through the analysis provenance. |
| Locale on run continuation (`appendIteration`) | Rejected | Iterations do not generate text; a locale there would have nothing to act on. |
| Read-only translation projection of existing runs | Deferred | It would need source-bound derived presentation artifacts with their own provenance, version and hash, stored next to immutable research state. No consumer needs it yet. It must never replace or regenerate a persisted run. |
| Report/export language (`report.md` headings, downstream articles) | Out of scope for Core | Presentation and delivery formats belong to consumers (managed publishing has its own target-language profiles). |
| Adapter-side projection only (no contract change) | Rejected | An adapter cannot record the generation context in the execution snapshot or in run comparison, so a locale difference would be unauditable. |

## Semantics

- `StartAnalysisRequest.outputLocale` accepts exactly `ja-JP` or `en-US`. Other values (`ja`, `ja-jp`, `fr-FR`) are `INVALID_REQUEST`; they are rejected, never guessed.
- **Omitted means "not requested", not a locale.** Generated text follows the predominant language of the source material or research question, exactly as before #125. The engine never records a resolved locale it cannot know, and never adopts a UI or browser locale. `EngineInfo.supportedOutputLocales` lists the accepted values; there is intentionally no `defaultOutputLocale`.
- The locale is **execution configuration**. It is recorded in `AnalysisRun.outputLocale` and `provenance.execution.outputLocale`, changes the execution fingerprint and never the input fingerprint. Run comparison attributes a locale difference to `EXECUTION_CHANGE` with field `outputLocale`. It is never new Evidence.
- An omitted locale is omitted from the execution snapshot, so every execution fingerprint recorded before #125 is unchanged (pinned by `TestOutputLocaleUnsetKeepsLegacyExecutionFingerprints`).
- The prompt instruction only applies when a locale was requested. Source quotes stay exact, character-for-character excerpts in the source language; numbers, units, dates, identifiers, source names, citations and enum codes are copied as they appear; translating must not strengthen or weaken uncertainty, causal limitations, counter-evidence or what cannot be concluded.
- Locale is not a reasoning profile and not a delivery format. `GENERAL_RESEARCH` / `CUSTOMER_INSIGHT` are unchanged by it.

## UI locale is separate

The browser UI language (#124) changes labels only and never sends `outputLocale`. If a UI control for output language is added later, it must be a separate, explicit control on the analysis form, not derived from the display language. Changing the display language alone never modifies a research artifact.

## Verification

- `20-output-locale` conformance fixture (model-backed): supported locales, omitted / `ja-JP` / `en-US` runs, verbatim quote with Japanese generated text, comparison `input SAME` / `execution CHANGED` on `outputLocale`, and `INVALID_REQUEST` for `ja` and `fr-FR`.
- `internal/http/output_locale_fidelity_test.go`: the same evidence under `ja-JP` and `en-US` yields identical quotes and offsets, findings, hypothesis roles and causal / validation / identification statuses, missing evidence, falsification criteria, quality warnings, supporting and counter-evidence, research gaps, data requirements, what cannot be concluded and decision readiness; only generated text changes language.
- The scripted stand-in model (`internal/llm/scripted`) follows an explicit `ja-JP` instruction, so these checks run without a paid LLM. It measures contract behavior, not the translation quality of a real model.

## Known limitations

- A real model may still drift from the instruction (mixed-language output, translated quotes). Quotes are protected by grounding: a quote that is not an exact source span is discarded, as for any model output. Generated prose is not machine-checked for language.
- The legacy `CUSTOMER_INSIGHT` generic-need quality check matches a Japanese vocabulary only, so English output under `en-US` is not flagged by it. `GENERAL_RESEARCH` does not run that check.
- Report and artifact headings stay in their current language; only model-generated field values follow the locale.

## Owners and compatibility

- Core (this repository) owns the contract field, execution recording and prompt instruction. It is additive in v1; requests without the field and persisted runs behave exactly as before (migration `021_add_analysis_output_locale.sql` adds a nullable column).
- The standalone SDKs (`insight-sdk-go`, `insight-sdk-js`) must resync their contract snapshots from the exact engine revision and release a minor version. They stay thin and implement no translation logic.
- Consumers (for example TechVit Insight) pass `outputLocale` explicitly and keep their own UI locale separate.
