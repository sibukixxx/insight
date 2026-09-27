import type { Dictionaries } from "../../domain/locale";

export interface LocalePort {
  loadDictionaries(): Promise<Dictionaries>;
  savedLocale(): string | undefined;
  saveLocale(locale: string): void;
  browserLanguages(): readonly string[];
}
