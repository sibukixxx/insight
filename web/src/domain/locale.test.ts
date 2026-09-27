import { describe, expect, it } from "vitest";
import { resolveInitialLocale, translate } from "./locale";

describe("resolveInitialLocale", () => {
  it("prefers a saved supported locale", () => {
    expect(resolveInitialLocale("ja", ["en-US"])).toBe("ja");
  });

  it("follows the first supported browser language when nothing valid is saved", () => {
    expect(resolveInitialLocale("fr", ["de-DE", "ja-JP", "en"])).toBe("ja");
  });

  it("falls back to English", () => {
    expect(resolveInitialLocale(undefined, ["fr-FR"])).toBe("en");
  });
});

describe("translate", () => {
  const dicts = { en: { a: "Hello {name}", b: "Only English" }, ja: { a: "こんにちは {name}" } };

  it("fills placeholders in the current locale", () => {
    expect(translate(dicts, "ja", "a", { name: "<b>" })).toEqual({ ok: true, text: "こんにちは <b>" });
  });

  it("falls back to English, then reports the key as missing", () => {
    expect(translate(dicts, "ja", "b")).toEqual({ ok: true, text: "Only English" });
    expect(translate(dicts, "ja", "c")).toEqual({ ok: false, key: "c" });
  });

  it("leaves an unknown placeholder visible", () => {
    expect(translate(dicts, "en", "a")).toEqual({ ok: true, text: "Hello {name}" });
  });
});
