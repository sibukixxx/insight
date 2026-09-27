import { useEffect } from "preact/hooks";
import type { UseCases } from "../application";
import type { Dictionaries, Locale } from "../domain/locale";
import type { BuildInfo } from "../domain/models";
import { AppShell } from "../presentation/components/AppShell";
import { I18nProvider, useI18n } from "../presentation/i18n/I18nProvider";
import { Router } from "../presentation/router/Router";
import { useRoute } from "../presentation/router/useRoute";
import { ServicesProvider } from "../presentation/services/context";

interface Props {
  readonly useCases: UseCases;
  readonly build: BuildInfo;
  readonly dictionaries: Dictionaries;
  readonly locale: Locale;
}

export function App({ useCases, build, dictionaries, locale }: Props) {
  return (
    <ServicesProvider useCases={useCases} build={build}>
      <I18nProvider dictionaries={dictionaries} initialLocale={locale} onLocaleChange={useCases.locale.rememberLocale}>
        <Shell />
      </I18nProvider>
    </ServicesProvider>
  );
}

function Shell() {
  const route = useRoute();
  const { locale, t } = useI18n();
  useEffect(() => {
    document.documentElement.lang = locale;
    document.title = t("app.title");
  }, [locale, t]);
  useEffect(() => { window.scrollTo(0, 0); }, [route]);
  return (
    <AppShell onSettings={route.name === "settings"}>
      <Router route={route} />
    </AppShell>
  );
}
