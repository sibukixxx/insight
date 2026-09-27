import { useState } from "preact/hooks";
import type { StartAnalysisInput } from "../../application/ports";
import type { AnalysisWorkspace } from "../../application/usecases/analysis";
import type { AnalysisRun } from "../../domain/models";
import { Button, ButtonLink } from "../components/Button";
import { Card } from "../components/Card";
import { Notice } from "../components/Notice";
import { Loading, PageError } from "../components/States";
import { AnalysisForm } from "../features/analysis/AnalysisForm";
import { ReadinessList } from "../features/analysis/ReadinessList";
import { useRunWatcher } from "../features/analysis/useRunWatcher";
import { RunStatus } from "../features/runs/RunStatus";
import { useAsync } from "../hooks/useAsync";
import { useI18n } from "../i18n/I18nProvider";
import { errorMessage } from "../errors";
import { projectHash } from "../router/routes";
import { useUseCases } from "../services/context";
import { isActive } from "../../domain/runs";
import { ProjectFrame } from "./ProjectFrame";
import styles from "./Page.module.css";

export function AnalysisPage({ projectId }: { projectId: string }) {
  const { analysis } = useUseCases();
  const state = useAsync(() => analysis.loadAnalysis(projectId), [analysis, projectId]);
  if (state.status === "loading") return <Loading />;
  if (state.status === "error") return <PageError error={state.error} onRetry={state.reload} />;
  return <AnalysisView data={state.data} onChanged={state.reload} />;
}

type Outcome = { readonly type: "completed" } | { readonly type: "failed"; readonly message?: string } | undefined;

function AnalysisView({ data, onChanged }: { data: AnalysisWorkspace; onChanged: () => void }) {
  const { t } = useI18n();
  const { analysis } = useUseCases();
  const [started, setStarted] = useState<AnalysisRun | undefined>(undefined);
  const [error, setError] = useState<string | undefined>(undefined);
  const [busy, setBusy] = useState(false);
  const [outcome, setOutcome] = useState<Outcome>(undefined);
  const { project, readiness } = data;
  const latest = started ?? data.latest;
  const watching = isActive(latest) ? latest?.id : undefined;

  const live = useRunWatcher(watching, (event) => {
    setOutcome(event.type === "completed" ? { type: "completed" } : event.message === undefined ? { type: "failed" } : { type: "failed", message: event.message });
    setStarted(undefined);
    onChanged();
  });

  const start = (promise: Promise<AnalysisRun>) => {
    setBusy(true);
    setError(undefined);
    setOutcome(undefined);
    promise.then((run) => setStarted(run), (e: unknown) => setError(errorMessage(e, t))).finally(() => setBusy(false));
  };
  const onStart = (input: StartAnalysisInput) => start(analysis.startAnalysis(project.id, input));
  const failed = latest?.status === "failed" && !watching;

  return (
    <ProjectFrame project={project} current="analysis" title={t("analysisPage.title")} subtitle={t("analysisPage.lead")}>
      <Card title={t("readiness.title")}>
        {!readiness.canStart && <Notice kind="warning">{t("analysis.notReady")}</Notice>}
        <ReadinessList readiness={readiness} projectId={project.id} />
      </Card>
      <Card title={t("analysisPage.status")}>
        <div id="analysis-panel" class={styles.stack}>
          <RunStatus run={latest} live={live} />
          {outcome?.type === "failed" && <Notice kind="error">{t("analysis.streamFailed", { message: outcome.message ?? t("analysis.unknownError") })}</Notice>}
          {failed && latest && (
            <div class={styles.actions}>
              <span class={styles.hint}>{t("analysisPage.retryHint")}</span>
              <Button id="retry-analysis" onClick={() => start(analysis.retryAnalysis(latest))} disabled={busy || !readiness.canStart}>{t("analysisPage.retry")}</Button>
            </div>
          )}
          {latest?.status === "completed" && !watching && (
            <div class={styles.nextAction}>
              <span class={styles.nextTitle}>{t("analysisPage.completedNext")}</span>
              <ButtonLink variant="primary" href={projectHash(project.id, "findings", latest.id)}>{t("analysisPage.viewResults")} {"→"}</ButtonLink>
            </div>
          )}
        </div>
      </Card>
      <Card title={t("analysisPage.newRun")}>
        {error && <Notice kind="error" spaced>{error}</Notice>}
        <AnalysisForm canStart={readiness.canStart && !watching} busy={busy} onStart={onStart} />
      </Card>
    </ProjectFrame>
  );
}
