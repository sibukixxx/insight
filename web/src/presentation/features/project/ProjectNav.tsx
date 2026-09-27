import type { MessageKey } from "../../../domain/messages";
import { useI18n } from "../../i18n/I18nProvider";
import { projectHash, type ProjectPage } from "../../router/routes";
import styles from "./ProjectNav.module.css";

type Tab = "overview" | ProjectPage;

const TABS: readonly (readonly [Tab, MessageKey, boolean])[] = [
  ["overview", "projectNav.overview", true],
  ["input", "projectNav.input", false],
  ["analysis", "projectNav.analysis", false],
  ["findings", "projectNav.findings", true],
  ["patterns", "projectNav.patterns", true],
  ["runs", "projectNav.runs", false],
  ["evaluation", "projectNav.evaluation", true],
  ["research", "project.researchPublications", false],
];

/** Project sections; run-scoped sections keep the selected run (?run=). */
export function ProjectNav({ projectId, current, runId }: { projectId: string; current: Tab; runId?: string | undefined }) {
  const { t } = useI18n();
  return (
    <nav class={styles.nav} aria-label={t("projectNav.label")}>
      <ul class={styles.list}>
        {TABS.map(([tab, key, runScoped]) => (
          <li key={tab}>
            <a class={styles.link} href={projectHash(projectId, tab === "overview" ? undefined : tab, runScoped ? runId : undefined)} aria-current={tab === current ? "page" : undefined}>
              {t(key)}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}
