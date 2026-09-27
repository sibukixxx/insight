import { createContext, Fragment, type ComponentChildren, type VNode } from "preact";
import { useCallback, useContext, useMemo, useState } from "preact/hooks";
import { labelKey, type LabelTable } from "../../domain/codes";
import { LOCALES, translate, type Dictionaries, type Locale } from "../../domain/locale";
import type { MessageKey, MessageParams } from "../../domain/messages";

export interface I18n {
  readonly locale: Locale;
  readonly setLocale: (next: Locale) => void;
  /** Plain text for key; never HTML. */
  readonly t: (key: MessageKey, params?: MessageParams) => string;
  /** key with some placeholders filled by elements (e.g. <code>). Text stays text. */
  readonly tRich: (key: MessageKey, params: Readonly<Record<string, string | number | VNode>>) => VNode;
  /** The label of a stable code; an unknown code is shown as-is. */
  readonly label: (table: LabelTable, code: string | undefined) => string;
  readonly dateTime: (value: string | undefined) => string;
  readonly percent: (ratio: number | undefined) => string;
  readonly number: (value: number | undefined, fractionDigits?: number) => string;
}

const I18nContext = createContext<I18n | undefined>(undefined);

export function useI18n(): I18n {
  const value = useContext(I18nContext);
  if (!value) throw new Error("useI18n outside I18nProvider");
  return value;
}

interface Props {
  readonly dictionaries: Dictionaries;
  readonly initialLocale: Locale;
  readonly onLocaleChange?: (next: Locale) => void;
  readonly children: ComponentChildren;
}

export function I18nProvider({ dictionaries, initialLocale, onLocaleChange, children }: Props) {
  const [locale, setLocaleState] = useState<Locale>(initialLocale);

  const setLocale = useCallback((next: Locale) => {
    setLocaleState(next);
    onLocaleChange?.(next);
  }, [onLocaleChange]);

  const value = useMemo<I18n>(() => {
    const tag = LOCALES[locale].tag;
    // t returns the current locale, then English, then a visibly broken
    // marker so a missing key is noticed instead of guessed.
    const t = (key: MessageKey, params?: MessageParams): string => {
      const result = translate(dictionaries, locale, key, params);
      if (result.ok) return result.text;
      console.error(`missing UI text: ${key}`);
      return `⟦${key}⟧`;
    };
    const tRich: I18n["tRich"] = (key, params) => {
      const template = t(key);
      const parts: (string | VNode)[] = [];
      let last = 0;
      template.replace(/\{(\w+)\}/g, (match, name: string, offset: number) => {
        parts.push(template.slice(last, offset));
        const value = params[name];
        parts.push(value === undefined ? match : typeof value === "object" ? value : String(value));
        last = offset + match.length;
        return match;
      });
      parts.push(template.slice(last));
      return <Fragment>{parts.map((p, i) => <Fragment key={i}>{p}</Fragment>)}</Fragment>;
    };
    return {
      locale,
      setLocale,
      t,
      tRich,
      label: (table, code) => {
        const key = labelKey(table, code);
        return key ? t(key) : String(code ?? "");
      },
      dateTime: (v) => {
        if (!v) return t("common.notRecorded");
        const d = new Date(v);
        if (Number.isNaN(d.getTime())) return v;
        return new Intl.DateTimeFormat(tag, { dateStyle: "medium", timeStyle: "short" }).format(d);
      },
      percent: (ratio) => new Intl.NumberFormat(tag, { style: "percent", maximumFractionDigits: 0 }).format(ratio ?? 0),
      number: (v, digits = 0) => new Intl.NumberFormat(tag, { minimumFractionDigits: digits, maximumFractionDigits: digits }).format(v ?? 0),
    };
  }, [dictionaries, locale, setLocale]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}
