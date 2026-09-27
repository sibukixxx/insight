import { LOCALE_CODES, LOCALES, isLocale } from "../../domain/locale";
import { useI18n } from "../i18n/I18nProvider";
import { formStyles } from "./Field";

/** Switches the display language in place: the route and typed values stay. */
export function LocaleSelect({ id, compact = false }: { id: string; compact?: boolean }) {
  const { locale, setLocale, t } = useI18n();
  return (
    <select
      id={id}
      class={compact ? undefined : formStyles.control}
      data-locale-select
      aria-label={t("locale.label")}
      value={locale}
      onChange={(e) => {
        const next = e.currentTarget.value;
        if (isLocale(next)) setLocale(next);
      }}
    >
      {LOCALE_CODES.map((code) => <option key={code} value={code} lang={code}>{LOCALES[code].name}</option>)}
    </select>
  );
}
