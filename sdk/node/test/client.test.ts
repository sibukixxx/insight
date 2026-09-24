import { test } from "node:test";
import assert from "node:assert/strict";
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import type { AddressInfo } from "node:net";
import { InsightClient, InsightError } from "../src/index.ts";

async function withServer(handler: (req: IncomingMessage, res: ServerResponse, body: string) => void, run: (baseUrl: string) => Promise<void>): Promise<void> {
  const server = createServer((req, res) => {
    let body = "";
    req.on("data", (chunk) => (body += chunk));
    req.on("end", () => handler(req, res, body));
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  try {
    await run(`http://127.0.0.1:${(server.address() as AddressInfo).port}`);
  } finally {
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
  }
}

function reply(res: ServerResponse, status: number, body: string): void {
  res.writeHead(status, { "content-type": "application/json" });
  res.end(body);
}

test("createSubject fills the contract version and an idempotency key", async () => {
  let sent: any;
  await withServer((req, res, body) => {
    assert.equal(req.url, "/api/public/v1/subjects");
    sent = JSON.parse(body);
    reply(res, 201, '{"contractVersion":"1","subjectId":"s1","subject":{"namespace":"n","id":"i"},"title":"t","createdAt":"2026-01-01T00:00:00Z"}');
  }, async (baseUrl) => {
    const subject = await new InsightClient({ baseUrl }).createSubject({ subject: { namespace: "n", id: "i" } });
    assert.equal(subject.subjectId, "s1");
  });
  assert.equal(sent.contractVersion, "1");
  assert.equal(sent.idempotencyKey.length, 32);
});

test("contract errors become typed InsightErrors", async () => {
  await withServer((_req, res) => reply(res, 409, '{"contractVersion":"1","error":{"code":"IDEMPOTENCY_CONFLICT","message":"key reused"}}'), async (baseUrl) => {
    await assert.rejects(new InsightClient({ baseUrl }).startAnalysis("s1", { idempotencyKey: "k" }), (error: unknown) => {
      assert.ok(error instanceof InsightError);
      assert.deepEqual({ code: error.code, message: error.message, httpStatus: error.httpStatus }, { code: "IDEMPOTENCY_CONFLICT", message: "key reused", httpStatus: 409 });
      return true;
    });
  });
});

test("a response in an unsupported contract version fails explicitly", async () => {
  await withServer((_req, res) => reply(res, 200, '{"contractVersion":"2"}'), async (baseUrl) => {
    await assert.rejects(new InsightClient({ baseUrl }).getEngine(), { name: "InsightError", code: "UNSUPPORTED_CONTRACT_VERSION" });
  });
});

test("an unreachable engine is UNAVAILABLE", async () => {
  let url = "";
  await withServer((_req, res) => reply(res, 200, "{}"), async (baseUrl) => {
    url = baseUrl;
  });
  await assert.rejects(new InsightClient({ baseUrl: url }).getEngine(), { name: "InsightError", code: "UNAVAILABLE" });
});

test("a non-contract response is UNAVAILABLE with its HTTP status", async () => {
  await withServer((_req, res) => {
    res.writeHead(502);
    res.end("<html>bad gateway</html>");
  }, async (baseUrl) => {
    await assert.rejects(new InsightClient({ baseUrl }).getEngine(), { name: "InsightError", code: "UNAVAILABLE", httpStatus: 502 });
  });
});

test("the client timeout ends a hanging request as UNAVAILABLE", async () => {
  await withServer(() => {
    /* never answers */
  }, async (baseUrl) => {
    await assert.rejects(new InsightClient({ baseUrl, timeoutMs: 50 }).getEngine(), { name: "InsightError", code: "UNAVAILABLE" });
  });
});

test("waitForAnalysis stops when the caller aborts", async () => {
  await withServer((_req, res) => reply(res, 200, '{"contractVersion":"1","subjectId":"s1","analysisId":"a1","status":"running","createdAt":"2026-01-01T00:00:00Z"}'), async (baseUrl) => {
    const client = new InsightClient({ baseUrl, pollIntervalMs: 5 });
    await assert.rejects(client.waitForAnalysis("s1", "a1", { signal: AbortSignal.timeout(50) }), { name: "TimeoutError" });
  });
});

test("waitForAnalysis resolves a failed run instead of throwing", async () => {
  await withServer((_req, res) => reply(res, 200, '{"contractVersion":"1","subjectId":"s1","analysisId":"a1","status":"failed","error":"no documents","createdAt":"2026-01-01T00:00:00Z"}'), async (baseUrl) => {
    const run = await new InsightClient({ baseUrl }).waitForAnalysis("s1", "a1");
    assert.deepEqual({ status: run.status, error: run.error }, { status: "failed", error: "no documents" });
  });
});
