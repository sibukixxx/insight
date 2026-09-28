// Developer-only Swagger UI for docs/openapi (issue #157). Never shipped:
// it lives outside web/ and internal/web/dist, and only Node built-ins plus
// the pinned swagger-ui-dist package are used.
//
// It serves the Swagger UI page, the two OpenAPI documents (read fresh on
// every request) and reverse-proxies /api/* to a running `insight-lab
// serve`. The proxy keeps Try-it-out same-origin: insight-lab sends no CORS
// headers and answers no OPTIONS preflight. The browser's Origin header is
// forwarded unchanged, so insight-lab's loopback-origin check still applies
// to every proxied request.
//
//   OPENAPI_DOCS_PORT   listen port on 127.0.0.1 (default 8840)
//   INSIGHT_URL         insight-lab base URL (default http://127.0.0.1:8787)
import { createReadStream, existsSync, readFileSync } from "node:fs";
import http from "node:http";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, "..", "..");
const port = Number(process.env.OPENAPI_DOCS_PORT || 8840);
const target = new URL(process.env.INSIGHT_URL || "http://127.0.0.1:8787");
if (target.protocol !== "http:") {
  console.error("INSIGHT_URL must be an http:// URL of a local insight-lab server");
  process.exit(1);
}

const specs = {
  "/openapi/public-engine-v1.json": path.join(repoRoot, "docs/openapi/public-engine-v1.json"),
  "/openapi/reference-api.json": path.join(repoRoot, "docs/openapi/reference-api.json"),
};

const uiDir = path.join(here, "node_modules", "swagger-ui-dist");
if (!existsSync(uiDir)) {
  console.error("swagger-ui-dist is missing: run `pnpm --dir tools/openapi-docs install --frozen-lockfile` (make openapi-ui does this)");
  process.exit(1);
}
// Only the files index.html loads are served from the package.
const uiFiles = {
  "/swagger-ui/swagger-ui.css": "text/css; charset=utf-8",
  "/swagger-ui/swagger-ui-bundle.js": "text/javascript; charset=utf-8",
  "/swagger-ui/swagger-ui-standalone-preset.js": "text/javascript; charset=utf-8",
  "/swagger-ui/favicon-32x32.png": "image/png",
};

function sendFile(res, file, type) {
  res.writeHead(200, { "Content-Type": type, "Cache-Control": "no-store" });
  createReadStream(file).pipe(res);
}

function proxy(req, res) {
  const headers = { ...req.headers, host: target.host };
  const upstream = http.request(
    { hostname: target.hostname, port: target.port || 80, method: req.method, path: req.url, headers },
    (upRes) => {
      res.writeHead(upRes.statusCode || 502, upRes.headers);
      upRes.pipe(res);
    },
  );
  upstream.on("error", (err) => {
    if (!res.headersSent) {
      res.writeHead(502, { "Content-Type": "application/json; charset=utf-8" });
    }
    res.end(JSON.stringify({ error: `openapi-docs proxy: cannot reach ${target.origin} (${err.code || err.message}); is insight-lab serve running?` }));
  });
  // A client that goes away (e.g. closing an event stream) stops the
  // upstream request too. IncomingMessage "close" fires once the request
  // body is read, so watch the response instead.
  res.on("close", () => {
    if (!res.writableFinished) upstream.destroy();
  });
  req.pipe(upstream);
}

const server = http.createServer((req, res) => {
  const { pathname } = new URL(req.url, "http://localhost");
  if (pathname === "/api" || pathname.startsWith("/api/")) {
    proxy(req, res);
    return;
  }
  if (req.method !== "GET" && req.method !== "HEAD") {
    res.writeHead(405).end();
    return;
  }
  if (pathname === "/" || pathname === "/index.html") {
    sendFile(res, path.join(here, "index.html"), "text/html; charset=utf-8");
    return;
  }
  if (specs[pathname]) {
    res.writeHead(200, { "Content-Type": "application/json; charset=utf-8", "Cache-Control": "no-store" });
    res.end(readFileSync(specs[pathname]));
    return;
  }
  if (uiFiles[pathname]) {
    sendFile(res, path.join(uiDir, path.basename(pathname)), uiFiles[pathname]);
    return;
  }
  res.writeHead(404, { "Content-Type": "text/plain; charset=utf-8" }).end("not found");
});

server.listen(port, "127.0.0.1", () => {
  console.log(`Swagger UI (dev only): http://127.0.0.1:${port}/`);
  console.log(`Proxying /api/* to ${target.origin} (start it with: insight-lab serve -no-browser)`);
});
