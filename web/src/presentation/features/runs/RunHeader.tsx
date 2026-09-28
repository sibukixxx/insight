import type { AnalysisRun } from "../../../domain/models";
import { Disclosure } from "../../components/Disclosure";
import { EmptyState } from "../../components/States";
import { useI18n } from "../../i18n/I18nProvider";
import { ProvenanceChips } from "./ProvenanceChips";
import styles from "./Runs.module.css";

export function RunHeader({ run }: { run: AnalysisRun | undefined }) {
  const { t, tRich, dateTime } = useI18n();
  if (!run) return <EmptyState>{t("run.noCompletedYet")}</EmptyState>;
  return (
    <div data-run-id={run.id}>
      <p class={styles.header}>{t("run.header", { finished: dateTime(run.finishedAt) })}</p>
      <Disclosure summary={t("advanced.runProvenance")}>
        <p class={styles.runId}>{tRich("run.id", { id: <code>{run.id}</code> })}</p>
        <ProvenanceChips run={run} />
      </Disclosure>
    </div>
  );
}
