// Runs contracts/public-engine/v1/fixtures through the TypeScript SDK with
// the same rules as the Go runner (sdk/go/conformance), so both SDKs are held
// to identical behavior against a live engine.
import { readFileSync, readdirSync } from "node:fs";
import { randomBytes } from "node:crypto";
import { isDeepStrictEqual } from "node:util";
import { InsightClient, InsightError } from "../src/index.ts";

export const fixtureDir = new URL("../../../contracts/public-engine/v1/fixtures/", import.meta.url);

export interface Fixture {
  fixture: string;
  engine: "deterministic" | "model_backed";
  steps: Step[];
}

interface Step {
  op: string;
  params?: Record<string, string>;
  request?: unknown;
  expect: { ok?: boolean; errorCode?: string; httpStatus?: number };
  save?: Record<string, string>;
  assert?: { path: string; equals?: unknown; exists?: boolean; minLength?: number }[];
}

export function loadFixtures(): Fixture[] {
  return readdirSync(fixtureDir)
    .filter((name) => name.endsWith(".json"))
    .sort()
    .map((name) => JSON.parse(readFileSync(new URL(name, fixtureDir), "utf8")) as Fixture);
}

export async function runFixture(client: InsightClient, fixture: Fixture): Promise<void> {
  const vars: Record<string, string> = { uniq: randomBytes(6).toString("hex") };
  for (const [index, step] of fixture.steps.entries()) {
    try {
      await runStep(client, step, vars);
    } catch (error) {
      throw new Error(`${fixture.fixture} step ${index + 1} (${step.op}): ${(error as Error).message}`, { cause: error });
    }
  }
}

async function runStep(client: InsightClient, step: Step, vars: Record<string, string>): Promise<void> {
  const params = Object.fromEntries(Object.entries(step.params ?? {}).map(([k, v]) => [k, substitute(v, vars)]));
  const request = step.request === undefined ? undefined : JSON.parse(substitute(JSON.stringify(step.request), vars));
  let result: unknown;
  let failure: unknown;
  try {
    result = await call(client, step.op, params, request);
  } catch (error) {
    failure = error;
  }
  if (step.expect.errorCode) {
    if (!(failure instanceof InsightError)) throw new Error(`expected ${step.expect.errorCode}, got ${failure ?? "a result"}`);
    if (failure.code !== step.expect.errorCode || (step.expect.httpStatus && failure.httpStatus !== step.expect.httpStatus)) {
      throw new Error(`expected ${step.expect.errorCode}/${step.expect.httpStatus}, got ${failure.code}/${failure.httpStatus}: ${failure.message}`);
    }
    return;
  }
  if (failure) throw failure;
  const doc = JSON.parse(JSON.stringify(result));
  for (const assertion of step.assert ?? []) {
    const { found, value } = lookup(doc, assertion.path);
    let problem = "";
    if (assertion.exists !== undefined) {
      if (found !== assertion.exists) problem = `exists = ${found}, want ${assertion.exists}`;
    } else if (assertion.minLength !== undefined) {
      if (!Array.isArray(value) || value.length < assertion.minLength) problem = `want at least ${assertion.minLength} items, got ${JSON.stringify(value)}`;
    } else if ("equals" in assertion) {
      const want = JSON.parse(substitute(JSON.stringify(assertion.equals), vars));
      if (!found || !isDeepStrictEqual(value, want)) problem = `= ${JSON.stringify(value)}, want ${JSON.stringify(want)}`;
    } else {
      problem = "assertion has no condition";
    }
    if (problem) {
      const reason = lookup(doc, "error");
      throw new Error(`${assertion.path}: ${problem}${reason.found ? ` (result error: ${reason.value})` : ""}`);
    }
  }
  for (const [name, path] of Object.entries(step.save ?? {})) {
    const { found, value } = lookup(doc, path);
    if (!found) throw new Error(`save ${name}: path ${path} not found`);
    vars[name] = String(value);
  }
}

function call(c: InsightClient, op: string, p: Record<string, string>, request: any): Promise<unknown> {
  const required = (name: string) => p[name] ?? "";
  switch (op) {
    case "getEngine":
      return c.getEngine();
    case "createSubject":
      return c.createSubject(request);
    case "addEvidence":
      return c.addEvidence(required("subjectId"), request);
    case "startAnalysis":
      return c.startAnalysis(required("subjectId"), request);
    case "getAnalysis":
      return c.getAnalysis(required("subjectId"), required("analysisId"));
    case "waitForAnalysis":
      return c.waitForAnalysis(required("subjectId"), required("analysisId"));
    case "getAnalysisResults":
      return c.getAnalysisResults(required("subjectId"), required("analysisId"));
    case "createResearchRun":
      return c.createResearchRun(required("subjectId"), request);
    case "appendIteration":
      return c.appendIteration(required("researchRunId"), request);
    case "getResearchRun":
      return c.getResearchRun(required("researchRunId"));
  }
  throw new Error(`unknown op ${op}`);
}

function substitute(text: string, vars: Record<string, string>): string {
  return text.replace(/\$\{([A-Za-z0-9_]+)\}/g, (match, name: string) => vars[name] ?? match);
}

function lookup(doc: unknown, path: string): { found: boolean; value: unknown } {
  let current: unknown = doc;
  for (const part of path.split(".")) {
    const match = /^([^[]+)((?:\[\d+\])*)$/.exec(part);
    if (!match || current === null || typeof current !== "object" || Array.isArray(current)) return { found: false, value: undefined };
    current = (current as Record<string, unknown>)[match[1] ?? ""];
    for (const index of (match[2] ?? "").matchAll(/\[(\d+)\]/g)) {
      if (!Array.isArray(current)) return { found: false, value: undefined };
      current = current[Number(index[1])];
    }
    if (current === undefined) return { found: false, value: undefined };
  }
  return { found: current !== undefined && current !== null, value: current };
}
