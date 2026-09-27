import { QUALITY_FLAG_LABELS } from "../../domain/codes";
import type { MessageKey } from "../../domain/messages";
import type { EvaluationMetrics } from "../../domain/models";
import { Card } from "../components/Card";
import { Notice } from "../components/Notice";
import { Loading, PageError } from "../components/States";
import { RunHeader } from "../features/runs/RunHeader";
import { RunSelector } from "../features/runs/RunSelector";
import { useAsync } from "../hooks/useAsync";
import { useI18n } from "../i18n/I18nProvider";
import { projectHash } from "../router/routes";
import { navigate } from "../router/useRoute";
import { useUseCases } from "../services/context";
import { ProjectFrame } from "./ProjectFrame";
import styles from "./EvaluationPage.module.css";

const RATES: readonly (readonly [MessageKey, MessageKey, keyof EvaluationMetrics])[] = [
  ["evaluation.evidenceCoverage.label", "evaluation.evidenceCoverage.desc", "evidenceCoverage"],
  ["evaluation.unsupportedClaimRate.label", "evaluation.unsupportedClaimRate.desc", "unsupportedClaimRate"],
  ["evaluation.counterEvidenceCoverage.label", "evaluation.counterEvidenceCoverage.desc", "counterEvidenceCoverage"],
  ["evaluation.insightDuplication.label", "evaluation.insightDuplication.desc", "insightDuplicationRate"],
  ["evaluation.traceBacked.label", "evaluation.traceBacked.desc", "traceBackedInsightRate"],
  ["evaluation.qualityFlagged.label", "evaluation.qualityFlagged.desc", "qualityFlaggedInsightRate"],
];

export function EvaluationPage({ projectId, runId }: { projectId: string; runId?: string | undefined }) {
  const { t } = useI18n();
  const { results } = useUseCases();
  const state = useAsync(() => results.loadEvaluation(projectId, runId), [results, projectId, runId]);
  if (state.status === "loading") return <Loading />;
  if (state.status === "error") return <PageError error={state.error} onRetry={state.reload} backHref={projectHash(projectId, undefined, runId)} />;
  const { project, runs, selection, data } = state.data;
  const run = selection.run;
  return (
    <ProjectFrame project={project} current="evaluation" runId={run?.id} title={t("evaluation.title", { project: project.name })} subtitle={t("evaluation.lead")}>
      {selection.requestedRunMissing && <Notice kind="warning">{t("run.notFoundNotice")}</Notice>}
      <Card title={t("findings.run")}>
        <RunHeader run={run} />
        <RunSelector runs={runs} selected={run} onSelect={(id) => navigate({ name: "evaluation", projectId: project.id, runId: id })} />
      </Card>
      {data && <Metrics metrics={data} />}
    </ProjectFrame>
  );
}

function Metrics({ metrics }: { metrics: EvaluationMetrics }) {
  const { t, percent, number } = useI18n();
  const value = (v: number | undefined, format: (x: number) => string) => (v === undefined ? t("common.notRecorded") : format(v));
  const counts = metrics.qualityFlagCounts ?? {};
  const flagSummary = Object.keys(QUALITY_FLAG_LABELS).filter((code) => counts[code])
    .map((code) => {
      const keys = QUALITY_FLAG_LABELS[code];
      return `${keys ? t(keys.label) : code}: ${counts[code]}`;
    }).join(" / ");
  const n = (v: number | undefined) => value(v, (x) => number(x));
  return (
    <Card title={t("evaluation.metrics")}>
      <ul class={styles.grid}>
        {RATES.map(([labelKey, descKey, field]) => (
          <li key={field} class={styles.tile}>
            <span class={styles.value}>{value(metrics[field] as number | undefined, percent)}</span>
            <span class={styles.label}>{t(labelKey)}</span>
            <span class={styles.desc}>{t(descKey)}</span>
          </li>
        ))}
        <li class={styles.tile}>
          <span class={styles.value}>{value(metrics.averageEvidencePerInsight, (x) => number(x, 1))}</span>
          <span class={styles.label}>{t("evaluation.avgEvidence.label")}</span>
          <span class={styles.desc}>{t("evaluation.avgEvidence.desc")}</span>
        </li>
      </ul>
      <div class={styles.summary}>
        <p>{t("evaluation.summaryObservations", { grounded: n(metrics.groundedObservations), total: n(metrics.totalObservationCandidates) })}</p>
        <p>{t("evaluation.summaryFindings", { patterns: n(metrics.patternCount), traces: n(metrics.traceCount ?? 0), drafts: n(metrics.totalInsightDrafts), final: n(metrics.finalInsightCount) })}</p>
        <p>{flagSummary ? t("evaluation.qualityWarnings", { summary: flagSummary }) : t("evaluation.noQualityWarnings")}</p>
      </div>
    </Card>
  );
}
