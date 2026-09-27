import { describe, expect, it } from "vitest";
import { buildHash, parseHash, type Route } from "./routes";

describe("parseHash keeps every pre-TypeScript hash", () => {
  it.each([
    ["", { name: "home" }],
    ["#/", { name: "home" }],
    ["#/settings", { name: "settings" }],
    ["#/projects/p%201", { name: "workspace", projectId: "p 1" }],
    ["#/projects/p1?run=r1", { name: "workspace", projectId: "p1", runId: "r1" }],
    ["#/projects/p1/patterns?run=r1", { name: "patterns", projectId: "p1", runId: "r1" }],
    ["#/projects/p1/evaluation", { name: "evaluation", projectId: "p1" }],
    ["#/projects/p1/research", { name: "research", projectId: "p1" }],
    ["#/research-runs/rr1", { name: "promotion", researchRunId: "rr1" }],
    ["#/insights/i1", { name: "insight", insightId: "i1" }],
    ["#/unknown/path", { name: "home" }],
    ["#/projects/%E0%A4%A", { name: "home" }],
  ] as const)("%s", (hash, route) => {
    expect(parseHash(hash)).toEqual(route);
  });

  it("parses the run comparison pair", () => {
    expect(parseHash("#/projects/p1/runs?a=r1&b=r2")).toEqual({ name: "runs", projectId: "p1", from: "r1", to: "r2" });
  });
});

describe("buildHash", () => {
  const routes: Route[] = [
    { name: "home" }, { name: "settings" }, { name: "workspace", projectId: "p/1", runId: "r&1" }, { name: "input", projectId: "p1" },
    { name: "analysis", projectId: "p1" }, { name: "findings", projectId: "p1", runId: "r1" }, { name: "runs", projectId: "p1", from: "a", to: "b" },
    { name: "promotion", researchRunId: "rr1" }, { name: "insight", insightId: "i1" },
  ];
  it.each(routes)("round-trips %o", (route) => {
    expect(parseHash(buildHash(route))).toEqual(route);
  });
});
