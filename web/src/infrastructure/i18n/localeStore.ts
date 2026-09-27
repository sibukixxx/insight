// Dictionaries are served next to the bundle (/locales/{lang}.json, see
// internal/web/i18n_test.go); the chosen locale is kept in localStorage
// under the key the pre-TypeScript UI used, so a saved choice survives.

import type { LocalePort } from "../../application/ports";
import { LOCALE_CODES, type Dictionaries, type Locale } from "../../domain/locale";
import type { Dictionary } from "../../domain/messages";
import { AppError } from "../../application/errors";
import type { Fetch } from "../http/client";

export const LOCALE_STORAGE_KEY = "insight-lab.locale";

type StorageLike = Pick<Storage, "getItem" | "setItem">;

function isDictionary(v: unknown): v is Dictionary {
  return typeof v === "object" && v !== null && !Array.isArray(v) && Object.values(v).every((x) => typeof x === "string");
}

export function browserLocaleStore(
  fetchImpl: Fetch = (input, init) => fetch(input, init),
  storage: () => StorageLike | undefined = () => {
    try {
      return window.localStorage;
    } catch {
      return undefined;
    }
  },
  languages: () => readonly string[] = () => navigator.languages ?? [navigator.language ?? ""],
): LocalePort {
  return {
    async loadDictionaries(): Promise<Dictionaries> {
      const entries = await Promise.all(LOCALE_CODES.map(async (code: Locale) => {
        const res = await fetchImpl(`/locales/${code}.json`);
        if (!res.ok) throw new AppError("http", `/locales/${code}.json: ${res.status}`, res.status);
        const body: unknown = await res.json();
        if (!isDictionary(body)) throw new AppError("invalid-response", `/locales/${code}.json is not a dictionary`);
        return [code, body] as const;
      }));
      return Object.fromEntries(entries);
    },
    savedLocale() {
      try {
        return storage()?.getItem(LOCALE_STORAGE_KEY) ?? undefined;
      } catch {
        return undefined;
      }
    },
    saveLocale(locale) {
      try {
        storage()?.setItem(LOCALE_STORAGE_KEY, locale);
      } catch {
        // storage unavailable: the choice lasts for this page only
      }
    },
    browserLanguages: () => languages(),
  };
}
