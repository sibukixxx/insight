// UI display language only (#124). Research content, evidence, quotes,
// server messages, IDs and enum codes are never translated; the language of
// model-generated text is a separate run setting (#125).

import type { Dictionary, MessageParams } from "./messages";

export const LOCALES = {
  en: { tag: "en-US", name: "English" },
  ja: { tag: "ja-JP", name: "日本語" },
} as const;

export type Locale = keyof typeof LOCALES;
export const LOCALE_CODES = Object.keys(LOCALES) as Locale[];

export function isLocale(value: unknown): value is Locale {
  return typeof value === "string" && Object.prototype.hasOwnProperty.call(LOCALES, value);
}

/** A saved choice wins; otherwise the first supported browser language; else English. */
export function resolveInitialLocale(saved: string | undefined, browserLanguages: readonly string[]): Locale {
  if (isLocale(saved)) return saved;
  for (const lang of browserLanguages) {
    const base = String(lang).toLowerCase().split("-")[0];
    if (isLocale(base)) return base;
  }
  return "en";
}

export type Dictionaries = Readonly<Partial<Record<Locale, Dictionary>>>;

/** Result of a lookup: the text, or the key when no dictionary defines it. */
export type Translation = { readonly ok: true; readonly text: string } | { readonly ok: false; readonly key: string };

/**
 * Looks key up in the current locale, then English. Placeholders {name}
 * are replaced by params; an unknown placeholder stays visible.
 */
export function translate(dicts: Dictionaries, locale: Locale, key: string, params?: MessageParams): Translation {
  const text = dicts[locale]?.[key] ?? dicts.en?.[key];
  if (text === undefined) return { ok: false, key };
  return {
    ok: true,
    text: text.replace(/\{(\w+)\}/g, (m, name: string) => (params && name in params ? String(params[name]) : m)),
  };
}
