import { useState } from "preact/hooks";
import type { StartAnalysisInput } from "../../application/ports";
import { outputLocaleNotRecorded, retryInput, type AnalysisWorkspace } from "../../application/usecases/analysis";
import { OUTPUT_LOCALE_LABELS } from "../../domain/codes";
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
import { assessReadiness } from "../../domain/readiness";
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
  const { t, label } = useI18n();
  const { analysis } = useUseCases();
  const [started, setStarted] = useState<AnalysisRun | undefined>(undefined);
  const [error, setError] = useState<string | undefined>(undefined);
  const [busy, setBusy] = useState(false);
  const [outcome, setOutcome] = useState<Outcome>(undefined);
  const [localeNotRecorded, setLocaleNotRecorded] = useState<string | undefined>(undefined);
  const { project } = data;
  const readiness = assessReadiness(data.documents, started ? [started, ...data.runs] : data.runs, data.settings);
  const latest = started ?? data.latest;
  const watching = isActive(latest) ? latest?.id : undefined;

  const live = useRunWatcher(watching, (event) => {
    setOutcome(event.type === "completed" ? { type: "completed" } : event.message === undefined ? { type: "failed" } : { type: "failed", message: event.message });
    setStarted(undefined);
    onChanged();
  });

  const start = (input: StartAnalysisInput, promise: Promise<AnalysisRun>) => {
    setBusy(true);
    setError(undefined);
    setOutcome(undefined);
    setLocaleNotRecorded(undefined);
    promise.then((run) => {
      setStarted(run);
      if (outputLocaleNotRecorded(input, run)) setLocaleNotRecorded(input.outputLocale);
    }, (e: unknown) => setError(errorMessage(e, t))).finally(() => setBusy(false));
  };
  const onStart = (input: StartAnalysisInput) => {
    if (busy || watching || !(input.exploratory ? readiness.canExplore : readiness.canStart)) return;
    start(input, analysis.startAnalysis(project.id, input));
  };
  const failed = latest?.status === "failed" && !watching;
  const canRetry = latest?.exploratory ? readiness.canExplore : readiness.canStart;
  // Candidates from an earlier question-only run stay reachable once evidence exists.
  const earlierExploration = data.documents.length > 0 ? data.runs.find((r) => r.metrics?.exploration !== undefined) : undefined;

  return (
    <ProjectFrame project={project} current="analysis" title={t("analysisPage.title")} subtitle={t("analysisPage.lead")}>
      <Card title={t("readiness.title")}>
        <ReadinessList readiness={readiness} projectId={project.id} />
      </Card>
      <Card title={t("analysisPage.status")}>
        <div id="analysis-panel" class={styles.stack}>
          <RunStatus run={latest} live={live} />
          {localeNotRecorded && <Notice kind="warning">{t("analysis.outputLocaleNotRecorded", { locale: label(OUTPUT_LOCALE_LABELS, localeNotRecorded) })}</Notice>}
          {outcome?.type === "failed" && <Notice kind="error">{t("analysis.streamFailed", { message: outcome.message ?? t("analysis.unknownError") })}</Notice>}
          {failed && latest && (
            <div class={styles.actions}>
              <span class={styles.hint}>{t("analysisPage.retryHint")}</span>
              <Button id="retry-analysis" onClick={() => { if (canRetry && !busy && !watching) start(retryInput(latest), analysis.retryAnalysis(latest)); }} disabled={busy || !canRetry}>{t("analysisPage.retry")}</Button>
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
        {earlierExploration && (
          <Notice kind="info" spaced>
            <span id="earlier-exploration">{t("analysisPage.earlierExploration", { count: earlierExploration.metrics?.exploration?.candidates.length ?? 0 })}</span>{" "}
            <a href={projectHash(project.id, "findings", earlierExploration.id)}>{t("analysisPage.earlierExplorationLink")}</a>
          </Notice>
        )}
        <AnalysisForm readiness={readiness} busy={busy} onStart={onStart} emphasized={!(latest?.status === "completed" && !watching)} initialOutputLocale={data.latest?.outputLocale} initialQuestion={project.researchQuestion ?? ""} />
      </Card>
    </ProjectFrame>
  );
}
