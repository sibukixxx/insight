# UI/UX follow-up review

Screenshot baseline: main `e963631` (includes #129 and #130).
Before finishing, main `e9f0a20` (#147–#149) was merged into the work branch. This change completes form
feedback and the CSV-to-analysis flow without replacing the frontend or
changing Go APIs, import contracts, research semantics, or delivery/demo tags.
Indigo/Cyan tokens and the #130 readiness rules are retained.

## Screenshots

Real delivery binary, same project state: empty Input and text-only Analysis
with no model. English desktop 1280 × 900; Japanese mobile 390 × 844. Full-page
captures, with Noto Sans CJK installed in the test environment. Browser-native
file-picker labels follow the browser installation language.

| Screen | Before | After |
| --- | --- | --- |
| Input, desktop/en | [Before](before-input-desktop-en.png) | [After](after-input-desktop-en.png) |
| Input, mobile/ja | [Before](before-input-mobile-ja.png) | [After](after-input-mobile-ja.png) |
| Analysis, desktop/en | [Before](before-analysis-desktop-en.png) | [After](after-analysis-desktop-en.png) |
| Analysis, mobile/ja | [Before](before-analysis-mobile-ja.png) | [After](after-analysis-mobile-ja.png) |

## Local verification

Environment: Go 1.26.1, Node 24.19.0, pnpm 11.25.0, Linux, Chrome for Testing
131.0.6778.204 via `PLAYWRIGHT_CHROMIUM_EXECUTABLE`. The managed Playwright
browser download was unavailable; the installed Chromium override is optional
and CI retains its normal browser selection. No paid model was used.

| Check | Result |
| --- | --- |
| `pnpm --dir web typecheck` | Passed |
| `pnpm --dir web lint` | Passed |
| `pnpm --dir web test` | 97 passed, 11 files; includes Onion boundary and 20 light/dark text contrast checks |
| `make web-check` | Passed after commit; generated dist matches source |
| `make test` | Passed, normal and demo tags |
| `make vet` | Passed, normal and demo tags |
| `make web-e2e` | 23 passed; real delivery/demo binaries |
| `make build` / `make build-demo` | Passed |

The E2E suite replaces the contradictory blocked-start test and retains a
separate real-server failure/retry test. An active-run response is deliberately
held by a route fixture to verify duplicate-start prevention. Settings UNKNOWN
is also injected; the CSV, deterministic analysis and report paths use real
APIs. English/Japanese desktop/mobile projects verify error focus, summary
links, accessible descriptions, keyboard disclosure, downloads, import counts
and row errors, completion, page overflow, and reduced motion.

Controls and relevant action/disclosure/link targets use at least 44px height.
Screenshots were visually checked for wrapping and missing labels. Validation
summaries expose `role=alert`; error text is linked by `aria-describedby` and
invalid controls expose `aria-invalid`. These are automated accessibility
checks, not a claim of manual NVDA/VoiceOver testing. Contrast tests cover the
listed token pairs, not every possible rendered pixel. Mobile coverage is
browser emulation rather than physical-device testing. Project navigation
retains its existing locally scrollable tab strip.

The generated `internal/web/dist` is rebuilt from source and committed; the
post-commit `make web-check` verifies reproducibility against git. No Go or
public-contract sources are changed.

## GitHub Actions

[Frontend checks run 36330665901](https://github.com/sibukixxx/insight/actions/runs/36330665901)
failed before any steps ran. GitHub reports: “The job was not started because
your account is locked due to a billing issue.” This is an account/billing
restriction, separate from the successful local checks above. No successful
GitHub Actions run is claimed.
