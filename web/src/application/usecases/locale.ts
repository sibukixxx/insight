import { resolveInitialLocale, type Dictionaries, type Locale } from "../../domain/locale";
import type { Ports } from "../ports";

export interface LocaleBoot {
  readonly dictionaries: Dictionaries;
  readonly locale: Locale;
}

export function localeUseCases({ locale }: Pick<Ports, "locale">) {
  return {
    /** Loads every dictionary; fails when they cannot be loaded (the UI shows that, not keys). */
    bootLocale: async (): Promise<LocaleBoot> => ({
      dictionaries: await locale.loadDictionaries(),
      locale: resolveInitialLocale(locale.savedLocale(), locale.browserLanguages()),
    }),
    rememberLocale: (next: Locale): void => locale.saveLocale(next),
  };
}
