import type { AnalysisRun } from "../../../domain/models";
import { useI18n } from "../../i18n/I18nProvider";
import { runLabel } from "./runLabel";
import styles from "./Runs.module.css";

export function RunList({ runs, selected, hrefFor }: { runs: readonly AnalysisRun[]; selected: AnalysisRun | undefined; hrefFor: (runId: string) => string }) {
  const i18n = useI18n();
  if (runs.length === 0) return null;
  return (
    <details class={styles.list}>
      <summary>{i18n.t("run.allRuns", { count: runs.length })}</summary>
      <ul class={styles.items}>
        {runs.map((run) => {
          const current = selected?.id === run.id;
          const text = runLabel(run, i18n);
          return (
            <li key={run.id} class={[styles.item, current && styles.current, run.status === "failed" && styles.failed].filter(Boolean).join(" ")}>
              {run.status === "completed" && !current ? <a href={hrefFor(run.id)}>{text}</a> : <span>{text}</span>}
              {current && <strong>{i18n.t("run.shown")}</strong>}
              {run.status === "failed" && run.error && <span>{run.error}</span>}
            </li>
          );
        })}
      </ul>
    </details>
  );
}
