# OpenAPI descriptions (#157)

Two separate OpenAPI 3.1 documents. Keep them apart: they have different owners, sources of truth and stability promises.

| File | Surface | Stability | Source of truth |
| --- | --- | --- | --- |
| [`public-engine-v1.json`](public-engine-v1.json) | Public Engine Contract v1, `/api/public/v1` | Stable. This is the contract the Go/TypeScript SDKs target | **Generated** from [`contracts/public-engine/v1/schema.json`](../../contracts/public-engine/v1/schema.json). Do not edit by hand |
| [`reference-api.json`](reference-api.json) | Reference Web API, `/api/...` in [`internal/http/router.go`](../../internal/http/router.go) | **None.** Not public, not SDK-backed, can change at any time | Hand-maintained, partial, with an enforced `x-coverage` inventory |

Both files are JSON, which is also valid YAML. JSON keeps the generator and the tests on the Go standard library, so the module takes on no new dependency.

## Public Engine spec: generation and checks

```sh
make openapi         # regenerate docs/openapi/public-engine-v1.json (go run ./cmd/insight-openapi)
make openapi-check   # fail if the committed file is stale, then run the OpenAPI tests below
```

`internal/openapigen` converts the schema mechanically:

- Operations come from `x-operations`. The key is the `operationId`, and method, path, success status, request and response come from the entry. The generator adds no operation the schema does not list.
- `components.schemas` holds the schema's `$defs`. The only change is the rewrite of `#/$defs/X` to `#/components/schemas/X`. OpenAPI 3.1 uses JSON Schema 2020-12, so nothing else needs translation. The generator fails on any other `$ref` form and on `$id`/`$anchor`/`$dynamicRef` inside `$defs`.
- `x-contractVersion`, `x-contractSchema`, `x-errorCodes` and `x-limits` are copied through as-is. `info.version` is the contract version.
- Errors: every operation has a `default` response with `ErrorResponse`, which follows the `x-errorCodes` status table. Operations with a request body also get an explicit `400` because the transport's body decoder returns `INVALID_REQUEST` for invalid or oversized JSON. The generator does not list per-operation 404/409/422 codes because nothing verifies them per operation. That is why the linter reports `operation-4xx-response` warnings, and they stay.
- Root `security: []` records that the transport has no authentication.

## Tests (run by `make test`)

- `internal/openapigen` `TestPublicSpecIsCurrent` regenerates the spec and byte-compares it with the committed file.
- `internal/openapigen` `TestSpecsAreStructurallyValid` checks both files for: `openapi: 3.1.0`, a non-empty info title and version, internal `$ref`s only and every one resolving, unique `operationId`s, path templates matching declared required path parameters, and valid response codes.
- `internal/http` `TestPublicOpenAPIMatchesRouterAndContract` walks the real router from `NewRouter` with `chi.Walk` and the Public Engine mounted, then requires three sets to be equal: the routes served under `/api/public/v1`, the schema's `x-operations`, and the operations in `public-engine-v1.json`. Each `operationId` must also equal its `x-operations` key. Parameter spelling (`{subjectID}` vs `{subjectId}`) is normalised. If the schema lists an operation the router does not serve, this test fails. The spec never advertises such an operation.
- `internal/http` `TestReferenceOpenAPICoverageMatchesRouter` walks `NewRouter` without the Public Engine. `x-coverage.covered` plus `x-coverage.notYetCovered` must equal the served routes exactly, with no duplicates. `covered` must equal the operations described in `paths`. `/api/demo*` operations must carry `x-build: demo`.

## Reference API coverage

`x-coverage` in `reference-api.json` is the route inventory of `router.go`. The test above keeps it in sync, so other documents (e.g. the #156 acceptance matrix) should link to these entries and `operationId`s instead of keeping their own list.

Described so far (8 of 60 routes):

| Route | operationId | Notes |
| --- | --- | --- |
| `GET /api/health` | `refGetHealth` | build kind, engine identity, ingest/runtime capabilities |
| `GET /api/projects` | `refListProjects` | |
| `POST /api/projects` | `refCreateProject` | 400 for invalid JSON / empty name |
| `GET /api/import-formats/{kind}/template.csv` | `refGetImportTemplate` | `text/csv` attachment; 404 for unknown kind |
| `GET /api/projects/{projectID}/report.md` | `refExportProjectReport` | `text/markdown` attachment; optional `analysisId` query |
| `GET /api/analysis/{analysisID}/events` | `refStreamAnalysisEvents` | `text/event-stream`: `status`, `progress`, `completed`, `error` |
| `POST /api/demo` | `refCreateDemoProject` | `x-build: demo`; a delivery build answers 409 |
| `GET /api/demo/scenarios` | `refListDemoScenarios` | `x-build: demo`; a delivery build answers `[]` |

Everything else is listed in `x-coverage.notYetCovered`: documents and CSV import/preview, ingests (which answer 501 only when the server has no ingest manager; `insight-lab serve` always configures one), analysis get/cancel/retry/compare, insights, patterns, evaluation, research runs, settings and the remaining demo routes. A route missing from `paths` is a documentation gap, not a missing server feature. Extend the spec one domain group at a time, and move each entry from `notYetCovered` to `covered` in the same change.

Reference errors are `{"error": "<message>"}`, not the Public Engine `ErrorResponse`.

## Local Swagger UI (developer only)

```sh
# terminal 1: a real local server (delivery or demo build), default http://127.0.0.1:8787
go run ./cmd/insight-lab serve -no-browser            # or: bin/insight-lab serve -no-browser
# terminal 2: Swagger UI
make openapi-ui                                       # http://127.0.0.1:8840/
```

- Use the top-bar selector to switch between "Public Engine v1 (stable, SDK contract)" and "Reference API (non-public, non-stable, partial)". Extensions (`x-build`, `x-coverage`, `x-contractOperation`) are shown.
- Environment: `INSIGHT_URL` (default `http://127.0.0.1:8787`) points at the server and `OPENAPI_DOCS_PORT` (default `8840`) sets the page port, for example `INSIGHT_URL=http://127.0.0.1:8851 OPENAPI_DOCS_PORT=8852 make openapi-ui`.
- `tools/openapi-docs/` is a standalone dev package: Node built-ins plus the pinned `swagger-ui-dist@5.33.0`, installed by pnpm from its own lockfile. The telemetry install script of its `@scarf/scarf` dependency is denied in `pnpm-workspace.yaml`. It is not under `web/`, is not embedded in `internal/web/dist`, and is not compiled into any binary. The page binds to 127.0.0.1 only.

### Why the page proxies `/api/*` (origin constraints)

`internal/http/middleware/origin.go` (`RestrictOrigin`) answers `403 forbidden origin` to any request whose `Origin` header is not loopback (`localhost`, `127.0.0.1`, `::1`). The server sends no CORS headers and registers no `OPTIONS` routes. A Swagger UI page on a different port is cross-origin, so the browser would block the responses and reject JSON POSTs at preflight. The dev server therefore reverse-proxies `/api/*` to `INSIGHT_URL`, which keeps Try-it-out same-origin. It forwards the browser's `Origin` header unchanged, so the loopback check still protects the server: a request made from a non-loopback page still gets 403.

There is no authentication. `/api/settings` reads and writes the LLM connection, including the API key. Never expose the insight-lab port or the Swagger UI port beyond localhost.

### Try it

- Public: `getEngine` (no input), then `createSubject` with
  `{"contractVersion":"1","idempotencyKey":"swagger-1","subject":{"namespace":"demo","id":"s1"}}` returns 201 `Subject`. Sending invalid JSON returns 400 `INVALID_REQUEST` in the `ErrorResponse` shape.
- Reference: `refGetHealth`, then `refCreateProject` with `{"name":"My project"}` returns 201. `refGetImportTemplate` with `kind=documents` returns the `text/csv` header row. On a delivery build, `refCreateDemoProject` returns 409.
- Swagger UI cannot render a live `text/event-stream`. Read `refStreamAnalysisEvents` with `EventSource` or a streaming HTTP client.

## Linting

```sh
make openapi-lint   # pnpm dlx --package=@redocly/cli@2.54.3 redocly lint ... (fetches the linter once)
```

Last run (#157 first slice): both documents valid with 0 errors. The 15 warnings are all `operation-4xx-response`: GET operations that document only the generic `default` error, plus `/api/health`, `GET /api/projects` and `GET /api/demo/scenarios`, which have no 4xx response. The linter is not a project dependency and `make test` does not run it. `make test` runs the Go structural test instead.
