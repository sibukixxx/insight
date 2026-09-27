import type { ComponentChildren } from "preact";
import { useI18n } from "../i18n/I18nProvider";
import { useBuild } from "../services/context";
import { Badge } from "./Badge";
import { LocaleSelect } from "./LocaleSelect";
import styles from "./AppShell.module.css";

export function AppShell({ children, onSettings = false }: { children: ComponentChildren; onSettings?: boolean }) {
  const { t } = useI18n();
  const build = useBuild();
  return (
    <>
      <a class={styles.skip} href="#main" onClick={(e) => { e.preventDefault(); document.getElementById("main")?.focus(); }}>{t("nav.skipToContent")}</a>
      <header class={styles.header}>
        <div class={styles.headerInner}>
          <div class={styles.brand}>
            <a href="#/">Insight Lab</a>
            <span class={styles.tagline}>{t("app.tagline")}</span>
          </div>
          <div class={styles.tools}>
            {build.demoBuild ? <Badge tone="accent">{t("build.demo")}</Badge> : <Badge tone="warning">{t("build.delivery")}</Badge>}
            <span class={styles.localeSelect}><LocaleSelect id="locale-select-header" compact /></span>
            <a class={styles.settingsLink} href="#/settings" aria-current={onSettings ? "page" : undefined}>{"⚙"} {t("nav.settings")}</a>
          </div>
        </div>
      </header>
      <main id="main" class={styles.main} tabIndex={-1}>
        {build.clientName && <div class={styles.banner}>{t("build.confidential", { client: build.clientName })}</div>}
        {children}
      </main>
      <footer class={styles.footer}>{t("app.privacy")}</footer>
    </>
  );
}
