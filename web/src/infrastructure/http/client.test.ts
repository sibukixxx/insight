import { describe, expect, it } from "vitest";
import { AppError } from "../../application/errors";
import { httpAnalysis, httpEvidence, httpSamples } from "./adapters";
import { createHttpClient, type Fetch } from "./client";
import { decodeRun } from "./dto";

const respond = (status: number, body: unknown): Fetch => async () => new Response(body === undefined ? "" : JSON.stringify(body), { status });

describe("http client", () => {
  it("keeps the server's error message verbatim", async () => {
    const http = createHttpClient(respond(409, { error: "this build does not include demo data; start a demo build instead" }));
    await expect(http.getJson("/api/x", (v) => v)).rejects.toMatchObject({ kind: "http", status: 409, message: "this build does not include demo data; start a demo build instead" });
  });

  it("reports an unreachable server as a network error", async () => {
    const http = createHttpClient(async () => { throw new TypeError("Failed to fetch"); });
    await expect(http.getJson("/api/x", (v) => v)).rejects.toMatchObject({ kind: "network" });
  });

  it("rejects a response that does not match the DTO shape", async () => {
    const http = createHttpClient(respond(200, { id: 1 }));
    const error = await http.getJson("/api/analysis/x", decodeRun).catch((e: unknown) => e);
    expect(error).toBeInstanceOf(AppError);
    expect(error).toMatchObject({ kind: "invalid-response" });
  });
});

describe("sample scenarios", () => {
  it("downloads the scenario CSV byte for byte and keeps the server's refusal verbatim", async () => {
    const csv = "id,source,title,content\nr001,record,架空,架空データ。\n";
    const ok = httpSamples(createHttpClient(async (url) => new Response(url.endsWith("/input.csv") ? csv : "[]", { status: 200 })));
    const upload = await ok.input("ja-shop-records");
    expect(upload.name).toBe("insight-sample-ja-shop-records.csv");
    expect(await upload.blob.text()).toBe(csv);
    const refused = httpSamples(createHttpClient(respond(409, { error: "this build does not include demo data; start a demo build instead" })));
    await expect(refused.input("ja-shop-records")).rejects.toMatchObject({ kind: "http", status: 409, message: "this build does not include demo data; start a demo build instead" });
  });

  it("decodes provenance and keeps absent optional fields absent", async () => {
    const samples = httpSamples(createHttpClient(respond(200, [{
      id: "ja-official-population", projectId: "demo-scenario-ja-official-population", projectName: "Sample 02", dataKind: "official",
      importKind: "documents", inputFile: "input.csv", inputSha256: "fea2", rows: 8,
      sources: [{ kind: "official", publisher: "総務省統計局", unit: "人", regions: [{ code: "08201", name: "水戸市" }], periods: ["2015", "2020"] }],
      transform: { script: "examples/demo-ja/official/build.mjs", version: "1", description: "d" }, limitations: ["l"],
    }])));
    const [scenario] = await samples.list();
    expect(scenario?.sources[0]).toMatchObject({ kind: "official", unit: "人", regions: [{ code: "08201", name: "水戸市" }] });
    expect(scenario?.sources[0]?.license).toBeUndefined();
    expect(scenario?.sources[0]?.rows).toEqual([]);
  });
});

describe("decoders", () => {
  it("keep an omitted optional field absent rather than zero or empty", () => {
    const run = decodeRun({ id: "r1", projectId: "p1", status: "completed", progress: 100, createdAt: "t", metrics: { provenance: { mode: "deterministic", ruleVersion: "v" }, evidenceCoverage: 0 } });
    expect(run).not.toHaveProperty("executionFingerprint");
    expect(run).not.toHaveProperty("finishedAt");
    expect(run.metrics?.evidenceCoverage).toBe(0);
    expect(run.metrics).not.toHaveProperty("counterEvidenceCoverage");
    expect(run.metrics?.provenance).not.toHaveProperty("model");
  });
});

describe("adapters", () => {
  it("translate ?run= into the server's ?analysisId= and only send a chosen output locale", async () => {
    const calls: { url: string; body: unknown }[] = [];
    const fetchSpy: Fetch = async (url, init) => {
      calls.push({ url, body: init?.body ? JSON.parse(String(init.body)) : undefined });
      return new Response(JSON.stringify({ id: "r1", projectId: "p 1", status: "queued", progress: 0, createdAt: "t" }), { status: 202 });
    };
    const analysis = httpAnalysis(createHttpClient(fetchSpy));
    await analysis.start("p 1", { researchQuestion: "q", reasoningProfile: "GENERAL_RESEARCH", outputLocale: "" });
    await analysis.start("p 1", { researchQuestion: "", reasoningProfile: "GENERAL_RESEARCH", outputLocale: "ja-JP" });
    expect(calls[0]).toEqual({ url: "/api/projects/p%201/analysis", body: { researchQuestion: "q", reasoningProfile: "GENERAL_RESEARCH" } });
    expect(calls[1]?.body).toEqual({ researchQuestion: "", reasoningProfile: "GENERAL_RESEARCH", outputLocale: "ja-JP" });
  });

  it("send uploads as multipart to the kind's existing import endpoint", async () => {
    const urls: string[] = [];
    const fetchSpy: Fetch = async (url, init) => {
      urls.push(url);
      expect(init?.body).toBeInstanceOf(FormData);
      return new Response(JSON.stringify({ imported: 1, skipped: 0, errors: [], recordsRead: 3 }), { status: 200 });
    };
    const evidence = httpEvidence(createHttpClient(fetchSpy));
    const result = await evidence.import("p1", "analysis", { name: "a.csv", blob: new Blob(["x"]) });
    await evidence.import("p1", "documents", { name: "d.csv", blob: new Blob(["x"]) });
    expect(urls).toEqual(["/api/projects/p1/documents/import/analysis", "/api/projects/p1/documents/import"]);
    expect(result.recordsRead).toBe(3);
  });
});
