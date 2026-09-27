// Second, plugin-independent enforcement of the onion dependency rules
// (docs/frontend-architecture.md). ESLint reports the same violations while
// editing; this test fails the suite even if the lint config drifts.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const src = resolve(dirname(fileURLToPath(import.meta.url)), "..");
type Layer = "domain" | "application" | "infrastructure" | "presentation" | "app" | "styles" | "root";

const allowed: Readonly<Record<Layer, readonly Layer[]>> = {
  domain: ["domain"],
  application: ["application", "domain"],
  infrastructure: ["infrastructure", "application", "domain"],
  presentation: ["presentation", "application", "domain", "styles"],
  app: ["app", "presentation", "infrastructure", "application", "domain", "styles"],
  styles: [],
  root: ["app", "styles"],
};

function files(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return name === "test" ? [] : files(path);
    return /\.(ts|tsx)$/.test(name) && !/\.test\.tsx?$/.test(name) ? [path] : [];
  });
}

function layerOf(path: string): Layer {
  const first = relative(src, path).split(sep)[0] ?? "";
  return (["domain", "application", "infrastructure", "presentation", "app", "styles"].includes(first) ? first : "root") as Layer;
}

function imports(code: string): string[] {
  return [...code.matchAll(/(?:import|export)\s+(?:type\s+)?(?:[^"';]*?\s+from\s+)?["']([^"']+)["']/g)].map((m) => m[1] ?? "");
}

const sources = files(src).map((path) => ({ path, layer: layerOf(path), code: readFileSync(path, "utf8") }));

describe("onion architecture", () => {
  it("finds the sources it checks", () => {
    expect(sources.filter((s) => s.layer === "presentation").length).toBeGreaterThan(10);
  });

  it("imports only from inner or same layers", () => {
    const violations: string[] = [];
    for (const { path, layer, code } of sources) {
      for (const spec of imports(code)) {
        if (!spec.startsWith(".")) continue;
        // The dictionary shape (domain/messages.ts) is a type-only view of the locale data.
        if (spec.endsWith("/public/locales/en.json")) continue;
        const target = layerOf(resolve(dirname(path), spec));
        if (!allowed[layer].includes(target)) violations.push(`${relative(src, path)} (${layer}) imports ${spec} (${target})`);
      }
    }
    expect(violations).toEqual([]);
  });

  it("keeps domain and application free of UI frameworks and CSS", () => {
    const violations = sources
      .filter((s) => s.layer === "domain" || s.layer === "application")
      .flatMap((s) => imports(s.code).filter((spec) => spec.startsWith("preact") || spec.endsWith(".css")).map((spec) => `${relative(src, s.path)} imports ${spec}`));
    expect(violations).toEqual([]);
  });

  it("does no network or storage I/O outside infrastructure", () => {
    const io = /\b(fetch|EventSource|XMLHttpRequest|WebSocket|localStorage|sessionStorage)\s*[.(]/;
    const violations = sources
      .filter((s) => s.layer !== "infrastructure" && s.layer !== "app")
      .filter((s) => io.test(s.code.replace(/\/\/.*$/gm, "").replace(/\/\*[\s\S]*?\*\//g, "")))
      .map((s) => relative(src, s.path));
    expect(violations).toEqual([]);
  });

  it("renders no raw HTML", () => {
    const violations = sources.filter((s) => /dangerouslySetInnerHTML|\.innerHTML\s*=/.test(s.code)).map((s) => relative(src, s.path));
    expect(violations).toEqual([]);
  });
});
