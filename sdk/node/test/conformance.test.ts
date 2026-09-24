// Live conformance against real engines. The Go test
// internal/http/public_conformance_test.go starts a deterministic and a
// model-backed engine and runs this file with their URLs. Run standalone by
// setting INSIGHT_DETERMINISTIC_URL and INSIGHT_MODEL_BACKED_URL.
import { test } from "node:test";
import { InsightClient } from "../src/index.ts";
import { loadFixtures, runFixture } from "./conformance-runner.ts";

const urls: Record<string, string | undefined> = {
  deterministic: process.env.INSIGHT_DETERMINISTIC_URL,
  model_backed: process.env.INSIGHT_MODEL_BACKED_URL,
};
const required = process.env.INSIGHT_REQUIRE_CONFORMANCE === "1";

for (const fixture of loadFixtures()) {
  const baseUrl = urls[fixture.engine];
  test(`conformance ${fixture.fixture}`, { skip: !baseUrl && !required && `set the ${fixture.engine} engine URL to run` }, async () => {
    if (!baseUrl) throw new Error(`no URL for the ${fixture.engine} engine`);
    await runFixture(new InsightClient({ baseUrl, pollIntervalMs: 20, timeoutMs: 10_000 }), fixture);
  });
}
