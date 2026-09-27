// The locale dictionary shape. en.json is the reference dictionary; every
// other locale must define the same keys with the same placeholders
// (internal/web/i18n_test.go).
import type en from "../../public/locales/en.json";

export type MessageKey = keyof typeof en;
export type MessageParams = Readonly<Record<string, string | number>>;
export type Dictionary = Readonly<Record<string, string>>;
