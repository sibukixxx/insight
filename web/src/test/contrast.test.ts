import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
const css = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), "../styles/tokens.css"), "utf8");
const tokens = (s: string) => Object.fromEntries([...s.matchAll(/(--[\w-]+):\s*([^;]+);/g)].map((m) => [m[1], m[2]]));
const light = tokens(css.split(':root[data-theme="dark"]')[0] ?? "");
const dark = { ...light, ...tokens(css.split(':root[data-theme="dark"]')[1] ?? "") };
function color(set: Record<string, string>, key: string): string {
  const value = set[key];
  if (!value) throw new Error(`missing ${key}`);
  const ref = /^var\((--[\w-]+)\)$/.exec(value);
  return ref?.[1] ? color(set, ref[1]) : value;
}
function luminance(hex: string) {
  const channels = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255).map((x) => x <= 0.04045 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4);
  return (channels[0] ?? 0) * 0.2126 + (channels[1] ?? 0) * 0.7152 + (channels[2] ?? 0) * 0.0722;
}
for (const [theme, set] of Object.entries({ light, dark })) {
  describe(`${theme} WCAG contrast`, () => {
    it.each([
      ["text", "surface"], ["text-muted", "surface"], ["text-muted", "bg"],
      ["on-accent", "primary"], ["on-accent", "primary-hover"],
      ["primary", "primary-soft"], ["secondary", "secondary-soft"],
      ["success", "success-weak"], ["danger", "danger-weak"], ["warning", "warning-weak"],
      ["text", "bg"], ["primary", "surface"], ["primary", "bg"], ["on-ink", "ink"], ["on-ink-muted", "ink"], ["on-ink", "ink-raised"],
      ["info", "info-weak"], ["hypothesis", "hypothesis-weak"], ["text", "data-weak"],
    ])("%s text on %s meets AA", (foreground, background) => {
      const a = luminance(color(set, `--color-${foreground}`));
      const b = luminance(color(set, `--color-${background}`));
      expect((Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05)).toBeGreaterThanOrEqual(4.5);
    });
  });
}

// Data Cyan is decoration only: it must never be mistaken for a text color on
// a light surface, and it must stay visible as a bar or border next to text.
describe("Data Cyan usage", () => {
  const ratio = (fg: string, bg: string) => {
    const a = luminance(color(light, fg)); const b = luminance(color(light, bg));
    return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
  };
  it("is too light for text on white, so it is not used as a text token", () => {
    expect(ratio("--color-data", "--color-surface")).toBeLessThan(4.5);
    expect(css).not.toMatch(/--color-text[\w-]*:\s*var\(--color-data\)/);
  });
  it("still reaches 3:1 against Deep Ink as a non-text accent", () => {
    expect(ratio("--color-data", "--color-ink")).toBeGreaterThanOrEqual(3);
  });
});
