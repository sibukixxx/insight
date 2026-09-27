import type { AnalysisRun } from "../../../domain/models";
import { useI18n } from "../../i18n/I18nProvider";
import { formStyles } from "../../components/Field";
import { runLabel } from "./runLabel";
import styles from "./Runs.module.css";

interface Props {
  readonly runs: readonly AnalysisRun[];
  readonly selected: AnalysisRun | undefined;
  readonly onSelect: (runId: string) => void;
}

/** Chooses which completed run the results belong to; other runs are listed but disabled. */
export function RunSelector({ runs, selected, onSelect }: Props) {
  const i18n = useI18n();
  if (runs.length === 0) return null;
  return (
    <div class={styles.selector}>
      <label for="run-select">{i18n.t("run.selectorLabel")}</label>
      <select id="run-select" class={formStyles.control} value={selected?.id ?? ""} onChange={(e) => { if (e.currentTarget.value) onSelect(e.currentTarget.value); }}>
        {!selected && <option value="">{i18n.t("run.noCompleted")}</option>}
        {runs.map((run) => <option key={run.id} value={run.id} disabled={run.status !== "completed"}>{runLabel(run, i18n)}</option>)}
      </select>
    </div>
  );
}
