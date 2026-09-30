# Frontend architecture (Reference Web)

Issue: #128. The browser UI is a TypeScript (strict) + Preact application in
`web/`, bundled by Vite into `internal/web/dist/` and embedded into the Go
binary with the existing `//go:embed all:dist` (`internal/web/embed.go`).
The product is still a single Go binary; there is no Node runtime, SSR or
separate web server.

## Build and distribution

| Command | What it does | Needs Node |
|---|---|---|
| `make build` / `make build-demo` | Go binary embedding the **committed** `internal/web/dist` | no |
| `make web-build` | `pnpm install --frozen-lockfile` + `vite build` into `internal/web/dist` | yes |
| `make build-all` | `web-build` + `build` | yes |
| `make web-check` | typecheck, ESLint, Vitest, rebuild, then fail if `internal/web/dist` differs from git or has untracked files | yes |
| `make web-e2e` | Playwright against real binaries (see E2E) | yes |

- `internal/web/dist` is generated. Never edit it by hand; change `web/` and
  run `make web-build`. The build is deterministic (content-hashed asset names,
  CSS Module class names `[file]__[class]` without path hashes), so CI can
  compare it byte for byte (`.github/workflows/frontend-checks.yml`).
- Locale dictionaries live in `web/public/locales/{en,ja}.json` and are copied
  to `dist/locales/`; the Go handler still serves `/locales/{lang}.json`.
- `pnpm dev` runs Vite with `/api` proxied to `insight-lab serve` on `:8787`.
- Nothing under `web/public` may contain sample data: it would ship in delivery
  builds. The sample project is loaded only from `POST /api/demo` (demo builds),
  and the sample scenarios only from `/api/demo/scenarios` (below).
  E2E fixtures live in `web/e2e/fixtures/` and are not bundled.

## Onion layers

```text
web/src/
  domain/          read models, code→label tables, pure display rules
  application/     ports (interfaces) and use cases
  infrastructure/  HTTP/SSE adapters, DTO decoding, locale persistence
  presentation/    pages, features, components, router, i18n context
  app/             App.tsx and composition-root.ts (the only wiring point)
  styles/          tokens.css, reset.css, global.css
  main.tsx         boot: dictionaries → build info → render
```

Allowed dependencies (enforced twice: ESLint `no-restricted-imports` /
`no-restricted-globals` in `web/eslint.config.js`, and the import walker in
`web/src/test/architecture.test.ts`):

| Layer | May import |
|---|---|
| domain | domain |
| application | application, domain |
| infrastructure | infrastructure, application (ports/errors), domain |
| presentation | presentation, application, domain |
| app | everything |

Additional rules checked by the same tests:

- domain/application import no Preact and no CSS, and touch no DOM globals.
- Only infrastructure (and the composition root) may use `fetch`,
  `EventSource`, `localStorage` etc. Presentation reaches the server only
  through use cases.
- No `dangerouslySetInnerHTML` / `innerHTML`. All user and server text is
  rendered as text nodes; the old `th()`/`trustedHtml` escape hatch is
  replaced by `tRich()`, which inserts elements (e.g. `<code>`) into a
  translated string without parsing HTML.

### What stays in Go

The UI does not re-implement research semantics. Import validation, the
Dataset Profile (Data Triage profiler), run selection on the server,
provenance, comparison attribution, quality flags and metrics all come from
the API. The UI-side rules are limited to display:

- `domain/runs.ts` — which run a view shows (`?run=` or latest completed,
  mirroring the server rule) and how missing provenance is shown
  ("not recorded" is never rendered as empty, zero or "same").
- `domain/readiness.ts` — pre-run checks that mirror conditions the server
  enforces or fails on (no evidence; active run; no model and no dataset →
  `pipeline.go` failure). A text-only project without a model is blocked before start; a dataset with no model retains deterministic analysis. Unknown settings are left to the server. The server
  remains authoritative and its error is always shown.
- `domain/evidence.ts` — splitting a quote out of its document and counting the
  links the server returned for the evidence map.

### Boundary validation

`infrastructure/http/decode.ts` + `dto.ts` validate every response. An
`omitempty` field that is absent decodes to `undefined` (never `""`, `0` or
`false`); a JSON `null` metric stays `null`. A malformed response becomes
`AppError("invalid-response")`; server errors keep the server's message
verbatim (`AppError("http")`); transport failures become `AppError("network")`
with a localized explanation.

## API used by the UI

All existing routes are used unchanged (`internal/http/router.go`), including
`?analysisId=` run scoping, `report.md`, SSE `GET /api/analysis/{id}/events`,
research/promotion routes and `GET …/analyses/compare?a=&b=`. Three internal
routes were **added** for the guided intake (no Public Engine Contract change):

- `GET /api/import-formats` — the accepted formats, columns and source types,
  derived from the Go importers (the UI shows nothing else as supported).
- `GET /api/import-formats/{kind}/template.csv` — header-only template.
- `POST /api/projects/{id}/documents/import/preview?kind=documents|analysis` —
  runs the real importer against a non-persisting repository and returns row
  errors, the first documents that would be created, and the Data Triage
  dataset profile of the same bytes.

Three more internal routes serve the sample-scenario gallery (#151). Their
data lives in `internal/sampledata/scenarios/` and is compiled in only by the
`demo` build tag; a delivery build lists no scenarios and answers 409 to the
other two. They are Reference API routes, not Public Engine Contract methods.

- `GET /api/demo/scenarios` — scenarios in display order with their stable
  `projectId`, data kind (`synthetic` / `official` / `mixed`), input format,
  row count, input sha256 and per-source provenance (publisher, series,
  regions, periods, unit, retrieval time, licence, raw snapshot sha256).
- `GET /api/demo/scenarios/{id}/input.csv` — the input CSV byte for byte,
  checked against the manifest checksum; used for the download link and for
  "Preview this CSV", which feeds it into the ordinary import preview.
- `POST /api/demo/scenarios/{id}/project` — creates the scenario's empty
  project once (`demo-scenario-<id>`) and returns it again afterwards. It
  imports nothing: the user brings the CSV in through preview → import.

Descriptions of what a sample is for are dictionary copy (`samples.*`), shown
before any analysis and never presented as results. Every screen of a sample
project repeats its data kind (`SampleStrip`).

## Theme

`styles/tokens.css` has three groups. Neutral surfaces and Deep Ink text
carry most of the UI. Brand colour is spent on emphasis: Deep Ink
(`--color-ink`) for the header and first-use hero, Primary Indigo
(`--color-primary`, one primary action per screen) and Data Cyan
(`--color-data`, selection bars and data-path accents only — 2.4:1 on
white, so never a text colour). Epistemic and status meaning uses its own
tokens (`--color-info*`, `--color-hypothesis*`, success/warning/danger) so a
brand change never recolours "hypothesis", "info" or a run state.
`src/test/contrast.test.ts` checks the text pairs for WCAG AA and that cyan
is not used as text.

SSE handling: the server only pushes events that happen after subscribing, so
`watchRun` (application) reads the run snapshot whenever the stream opens or
drops, and reports the terminal event exactly once. A server `error` event
(has `data`) is a run failure; a connection error (no `data`) is a disconnect.

## Screens and routes

Hash routing is kept; every pre-TypeScript hash keeps its meaning and unknown
hashes show Home, as before.

| Hash | Screen |
|---|---|
| `#/` | Home / dashboard: hero, "What would you like to investigate?" sample gallery (demo builds), English policy demo, own CSV, projects |
| `#/projects/:id[?run=]` | Project workspace: workflow stepper, single next action, latest run status, run selector, next steps, top findings |
| `#/projects/:id/input` | Input / evidence: supported formats + templates, CSV upload → preview → import, paste text, documents |
| `#/projects/:id/analysis` | Analysis: input checks, live progress, failure + retry with the same settings, question/profile/output locale |
| `#/projects/:id/findings[?run=]` | Results / findings of one run + next actions + report |
| `#/projects/:id/patterns[?run=]` | Traces and patterns (existing) |
| `#/projects/:id/runs[?a=&b=]` | Run history + non-causal comparison |
| `#/projects/:id/evaluation[?run=]` | Evaluation metrics (existing) |
| `#/projects/:id/research`, `#/research-runs/:id` | Research publications / promotion review (existing) |
| `#/insights/:id` | Insight: evidence map (observation → pattern → hypothesis → evidence), reasoning trail, quotes revealed in source |
| `#/settings` | Display language, LLM settings, connection test |

Technical detail (provenance chips, all runs, dataset profile) sits in
collapsed "Advanced" disclosures. The reference UI does not expose the newer job cancellation API yet.

## CSS

- `styles/tokens.css` — color, spacing, type, radius tokens. Components use
  tokens only; a dark token set exists behind `html[data-theme="dark"]`
  (opt-in; text/semantic token pairs are covered by WCAG AA contrast tests).
- `styles/reset.css`, `styles/global.css` — element defaults only.
- Every component/feature/page imports its own `*.module.css`. JavaScript sets
  only dynamic custom properties (e.g. `--meter-value` for progress bars).
- Responsive rules live next to the component (breakpoints ~600/720/800px);
  focus is always visible; motion respects `prefers-reduced-motion`.

## i18n

`presentation/i18n/I18nProvider.tsx` provides `t` (plain text), `tRich`,
`label` (stable code → label, unknown codes shown verbatim) and `Intl`
formatters. Switching the language re-renders in place: the route and typed
form values stay. The choice is stored under the pre-existing
`localStorage` key `insight-lab.locale`. Model output language is the run's
`outputLocale` (#125), chosen on the Analysis screen.
Display terms for internal concepts (ja/en, raw code, helper copy and
semantic cautions) are kept in [ui-terminology.md](ui-terminology.md).

Guards: `MessageKey` is typed from `en.json`; `internal/web/i18n_test.go`
checks en/ja key and placeholder parity, that `web/src` uses only defined keys
and every key is used, that user-facing JSX attributes are not hard-coded, and
that `dist/locales` matches `web/public/locales`. ESLint
`react/jsx-no-literals` rejects hard-coded JSX text.

## Tests

- Vitest (`web/src/**/*.test.ts[x]`): architecture rules, domain rules, router
  (legacy hashes, round trips), decoders/HTTP client, SSE adapter, `watchRun`,
  and component flows against in-memory fake ports (XSS rendering, locale
  switch, CSV preview/import, unsupported file, readiness, retry, run fallback).
- Go: `internal/web` (i18n guards, embedded index references embedded assets),
  `internal/service` and `internal/http` tests for the new import endpoints.
- Playwright (`web/e2e`, `make web-e2e`): starts real binaries with empty
  databases via `e2e/serve.sh` — a delivery build without a model (template
  download, unsupported file, CSV preview/import, XSS, deterministic run,
  report, deep links/back-forward, locale switch, failing run + retry path,
  settings, API error) and demo builds with `cmd/insight-scripted-llm`
  (sample → analysis → evidence → report on desktop and a Pixel 7 viewport;
  run comparison). No paid LLM is called. Ports 8811–8815.

## Form feedback and readiness follow-up

`Field` uses explicit `requirement` (`required`, `required-later`, `optional`)
and a render prop: `children(controlProps)`. The caller spreads these props on
the actual input/select/textarea. Field owns the label association, native
`required`, `aria-required`, `aria-invalid`, and description IDs for the
requirement, hint, example and error. It does not clone or rewrite children.
`required-later` stays natively optional for the current save operation.

`useFormValidation` and `ValidationSummary` keep presentation feedback local:
errors appear inline and in a live summary, links focus the control without
changing the hash route, and invalid submit focuses the first error after
rendering. Checks mirror Go: project name and document source/content are
trimmed and required (`internal/usecase/application.go`); a research question
is required for a new research action (`internal/usecase/research.go`), but
optional for starting an analysis. Promotion requires a contribution value.
Settings can be saved empty; model and base URL are only needed for model use.
Server validation remains authoritative. No research/input contracts changed.

CSV choices, extensions, columns and templates come from `import-formats`.
Documents is selected by default when offered. Other server formats are in a
disclosure. Selection calls the existing nonpersistent preview; only explicit
confirmation calls import. Counts, rejected rows, filename and next action
remain visible. The large asynchronous ingest API is outside this UI change.

Readiness preserves the #130 guards: no evidence, an active run, or known
missing model with no dataset prevents start. A dataset keeps deterministic
analysis available without a model. Unknown settings are a separate warning,
not proof that a run must fail. Start and retry handlers also guard against
blocked/active state. The server may still reject an unprocessable dataset;
real failure/retry coverage is independent of blocked-start coverage.

Advanced settings show effective choices outside the disclosure: open
question, GENERAL_RESEARCH, and source/question language detection by default.
An empty output locale delegates to the engine's language inference, not the
saved settings language. Explicit selections remain visible when collapsed.

The delivery E2E suite covers blocked start, settings navigation, deterministic
execution, a mocked active-run snapshot preventing duplicate submission, and
a real API failure with retry. Additional English/Japanese desktop/mobile
projects cover validation focus, descriptions, keyboard disclosure, template,
preview, import, analysis, report, no horizontal overflow and reduced motion.
`PLAYWRIGHT_CHROMIUM_EXECUTABLE` optionally selects an installed compatible
Chromium; otherwise Playwright uses its managed browser. See
[review evidence](ux-review/README.md) for environment and verification limits.
