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
  builds. The sample project is loaded only from `POST /api/demo` (demo builds).
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

SSE handling: the server only pushes events that happen after subscribing, so
`watchRun` (application) reads the run snapshot whenever the stream opens or
drops, and reports the terminal event exactly once. A server `error` event
(has `data`) is a run failure; a connection error (no `data`) is a disconnect.

## Screens and routes

Hash routing is kept; every pre-TypeScript hash keeps its meaning and unknown
hashes show Home, as before.

| Hash | Screen |
|---|---|
| `#/` | Home / dashboard: first-run guide, sample (demo builds), new project, projects |
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
collapsed "Advanced" disclosures. There is no cancel action because the API has
no cancel endpoint.

## CSS

- `styles/tokens.css` — color, spacing, type, radius tokens. Components use
  tokens only; a dark token set exists behind `html[data-theme="dark"]`
  (opt-in until reviewed for contrast).
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
