import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
// @ts-expect-error the generator is a plain ESM script without type declarations
import { generate } from "../scripts/generate-types.mjs";
import { loadFixtures } from "./conformance-runner.ts";

const schema = JSON.parse(readFileSync(new URL("../../../contracts/public-engine/v1/schema.json", import.meta.url), "utf8"));

test("generated contract types are up to date with the schema", () => {
  const current = readFileSync(new URL("../src/contract.gen.ts", import.meta.url), "utf8");
  assert.equal(current, generate(schema), "run `npm run generate`");
});

test("every fixture uses only operations the schema defines", () => {
  const operations = new Set([...Object.keys(schema["x-operations"]), "waitForAnalysis"]);
  for (const fixture of loadFixtures()) {
    for (const step of fixture.steps) assert.ok(operations.has(step.op), `${fixture.fixture}: unknown op ${step.op}`);
  }
});

test("every fixture error code is a contract error code", () => {
  const codes = new Set(Object.keys(schema["x-errorCodes"]));
  for (const fixture of loadFixtures()) {
    for (const step of fixture.steps) {
      if (step.expect.errorCode) assert.ok(codes.has(step.expect.errorCode), `${fixture.fixture}: ${step.expect.errorCode}`);
    }
  }
});
