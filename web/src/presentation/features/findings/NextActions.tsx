import type { NextActions as Next } from "../../../application/usecases/results";
import type { AnalysisRun } from "../../../domain/models";
import { ButtonLink } from "../../components/Button";
import { useI18n } from "../../i18n/I18nProvider";
import { projectHash } from "../../router/routes";
import { useUseCases } from "../../services/context";
import styles from "./Findings.module.css";

/** What to do once a run has completed. */
export function NextActions({ projectId, run, next }: { projectId: string; run: AnalysisRun; next: Next }) {
  const { t } = useI18n();
  const { exports } = useUseCases();
  return (
    <ul class={styles.actions}>
      <li class={styles.action}>
        <strong>{t("next.review.title")}</strong>
        <span class={styles.actionHint}>{next.hasFindings ? t("next.review.hint", { flagged: next.flaggedInsights }) : t("next.review.none")}</span>
        <ButtonLink size="small" href={projectHash(projectId, "findings", run.id)}>{t("next.review.action")}</ButtonLink>
      </li>
      <li class={styles.action}>
        <strong>{t("next.patterns.title")}</strong>
        <span class={styles.actionHint}>{t("next.patterns.hint")}</span>
        <ButtonLink size="small" href={projectHash(projectId, "patterns", run.id)}>{t("project.viewPatterns")}</ButtonLink>
      </li>
      <li class={styles.action}>
        <strong>{t("next.report.title")}</strong>
        <span class={styles.actionHint}>{t("next.report.hint")}</span>
        <ButtonLink size="small" variant="primary" href={exports.reportLink(projectId, run.id)} download>{t("project.downloadReport")}</ButtonLink>
      </li>
      <li class={styles.action}>
        <strong>{t("next.quality.title")}</strong>
        <span class={styles.actionHint}>{next.canCompare ? t("next.quality.hintCompare") : t("next.quality.hint")}</span>
        <ButtonLink size="small" href={next.canCompare ? projectHash(projectId, "runs") : projectHash(projectId, "evaluation", run.id)}>
          {next.canCompare ? t("next.quality.compare") : t("project.viewEvaluation")}
        </ButtonLink>
      </li>
    </ul>
  );
}
