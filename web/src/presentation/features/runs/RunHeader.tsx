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
    <div>
      <p class={styles.header}>{tRich("run.header", { id: <code>{run.id}</code>, finished: dateTime(run.finishedAt) })}</p>
      <Disclosure summary={t("advanced.runProvenance")}>
        <ProvenanceChips run={run} />
      </Disclosure>
    </div>
  );
}
