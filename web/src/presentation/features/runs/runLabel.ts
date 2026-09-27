import { RUN_STATUS_LABELS } from "../../../domain/codes";
import type { AnalysisRun } from "../../../domain/models";
import { runProvenance } from "../../../domain/runs";
import type { I18n } from "../../i18n/I18nProvider";

export function runLabel(run: AnalysisRun, { dateTime, label }: Pick<I18n, "dateTime" | "label">): string {
  const model = runProvenance(run).model;
  return `${dateTime(run.finishedAt ?? run.createdAt)} · ${label(RUN_STATUS_LABELS, run.status)}${model ? ` · ${model}` : ""} · ${run.id}`;
}
