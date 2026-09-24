# Insight Lab TypeScript SDK

Thin TypeScript client for the [Public Engine Contract v1](../../docs/public-engine-contract.md). Types are generated from `contracts/public-engine/v1/schema.json`. The SDK holds no research logic and no domain policy.

```ts
import { InsightClient, InsightError } from "@sibukixxx/insight-sdk";

const client = new InsightClient({ baseUrl: "http://127.0.0.1:8787", timeoutMs: 30_000 });
const subject = await client.createSubject({ idempotencyKey: "my-subject", subject: { namespace: "my-app", id: "item-42" } });
// addEvidence -> startAnalysis -> waitForAnalysis -> getAnalysisResults
// createResearchRun -> appendIteration -> getResearchRun
```

Run the example against a local engine with `node example/minimal.ts http://127.0.0.1:8787`.

- Every method accepts `{ signal }` for aborts, combined with the client timeout.
- Failures throw `InsightError` with a contract `code` and `httpStatus`. `UNAVAILABLE` means the engine could not be reached.
- `contractVersion` and a random `idempotencyKey` are filled in when omitted. Reuse your own key on retry.

| Script | Purpose |
|---|---|
| `npm run generate` | Regenerate `src/contract.gen.ts` from the schema |
| `npm run check-generated` | Fail if the generated types are stale |
| `npm run typecheck` | Type-check with `tsc` |
| `npm test` | Unit, drift and (with engine URLs) conformance tests |
| `npm run build` | Emit `dist/` for publishing |

The package is not published to npm yet. It supports contract version 1 (`insightContractVersions` in `package.json`).
