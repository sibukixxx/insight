import { MODE_LABELS } from "../../../domain/codes";
import type { MessageKey } from "../../../domain/messages";
import type { AnalysisRun } from "../../../domain/models";
import { provenanceItems, type ProvenanceItem } from "../../../domain/runs";
import { badgeStyles } from "../../components/Badge";
import { useI18n } from "../../i18n/I18nProvider";

const FIELD_LABELS: Readonly<Record<ProvenanceItem["field"], MessageKey>> = {
  mode: "provenance.mode",
  model: "provenance.model",
  prompt: "provenance.prompt",
  rules: "provenance.rules",
  engine: "provenance.engine",
  execution: "provenance.execution",
  input: "provenance.input",
  question: "provenance.question",
};

/** What the run recorded about itself; "not recorded" is shown as such, never as blank. */
export function ProvenanceChips({ run }: { run: AnalysisRun }) {
  const { t, label } = useI18n();
  return (
    <ul class={badgeStyles.chips} aria-label={t("provenance.title")}>
      {provenanceItems(run).map((item) => {
        const missing = item.value.kind === "notRecorded";
        const shown = item.value.kind === "notRecorded" ? t("common.notRecorded")
          : item.value.kind === "noModel" ? t("provenance.noModel")
            : item.isModeCode ? label(MODE_LABELS, item.value.value) : item.value.value;
        const full = item.value.kind === "value" ? item.value.full ?? shown : shown;
        return (
          <li key={item.field} class={`${badgeStyles.badge} ${badgeStyles.chip} ${missing ? badgeStyles.missing : badgeStyles.accent}`} title={full} data-provenance={item.field}>
            {t(FIELD_LABELS[item.field])}: {shown}
          </li>
        );
      })}
    </ul>
  );
}
