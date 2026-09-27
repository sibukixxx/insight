import type { Readiness } from "../../../domain/readiness";
import { useI18n } from "../../i18n/I18nProvider";
import { buildHash } from "../../router/routes";
import styles from "./Analysis.module.css";

const ICON = { ok: "✓", info: "i", warning: "!", blocked: "×" } as const;
const LEVEL_LABELS = { ok: "readiness.level.ok", info: "readiness.level.info", warning: "readiness.level.warning", blocked: "readiness.level.blocked" } as const;

/** Pre-run checks; the server remains authoritative when a run starts. */
export function ReadinessList({ readiness, projectId }: { readiness: Readiness; projectId: string }) {
  const { t } = useI18n();
  return (
    <ul class={styles.checks} aria-label={t("readiness.title")}>
      {readiness.checks.map((c) => (
        <li key={c.id} class={`${styles.check} ${styles[c.level]}`} data-check={c.id} data-level={c.level}>
          <span class={styles.icon} aria-hidden="true">{ICON[c.level]}</span>
          <span class="visually-hidden">{t(LEVEL_LABELS[c.level])}</span>
          <span class={styles.checkBody}>{t(c.message, c.params)}</span>
          {c.fix === "input" && <a class={styles.fix} href={buildHash({ name: "input", projectId })}>{t("readiness.fixInput")}</a>}
          {c.fix === "settings" && <a class={styles.fix} href="#/settings">{t("readiness.fixSettings")}</a>}
        </li>
      ))}
    </ul>
  );
}
