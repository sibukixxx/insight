import { RUN_STATUS_LABELS, STEP_LABELS, labelKey } from "../../../domain/codes";
import type { AnalysisEvent, AnalysisRun } from "../../../domain/models";
import { runOutcome } from "../../../domain/runs";
import { Meter } from "../../components/Meter";
import { Notice } from "../../components/Notice";
import { useI18n } from "../../i18n/I18nProvider";
import styles from "./Runs.module.css";

export type LiveProgress = Extract<AnalysisEvent, { type: "progress" }>;

/**
 * The newest run's state. A known step gets its localized label and keeps
 * the server's message as a tooltip; an unknown step stays visible as sent.
 */
export function RunStatus({ run, live }: { run: AnalysisRun | undefined; live?: LiveProgress | undefined }) {
  const { t, label, dateTime } = useI18n();
  if (!run) return <p class={styles.stepLabel}>{t("analysis.none")}</p>;
  if (run.status === "completed" && !live) return <p class={styles.completed} role="status">{t("analysis.completed", { finished: dateTime(run.finishedAt) })}</p>;
  const outcome = runOutcome(run);
  if (!live && outcome === "cancelled") return <Notice kind="info">{t("analysis.cancelled")}</Notice>;
  if (!live && outcome === "interrupted") return <Notice kind="warning">{t("analysis.interrupted")}</Notice>;
  if (run.status === "failed" && !live) return <Notice kind="error">{t("analysis.failed", { message: run.error ?? "" })}</Notice>;
  if (!live && outcome === "unknown") return <Notice kind="warning">{t("analysis.unknownState", { status: run.status })}</Notice>;
  const step = live?.step ?? run.currentStep;
  const known = labelKey(STEP_LABELS, step);
  // A snapshot of a queued run has no step yet; its status code is labelled too.
  const statusKey = labelKey(RUN_STATUS_LABELS, step);
  const stepText = known ? t(known) : statusKey ? t(statusKey) : live?.message ?? (step ? step : label(RUN_STATUS_LABELS, run.status));
  const progress = live?.progress ?? run.progress;
  return (
    <div class={styles.status} role="status" aria-live="polite">
      <span class={styles.stepLabel} title={known ? live?.message : undefined}>{t("analysis.inProgress", { step: stepText })}</span>
      <Meter label={t("analysis.progressLabel")} percent={progress} valueText={`${progress}%`} hideLabel />
    </div>
  );
}
