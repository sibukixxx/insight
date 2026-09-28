import { nextActions } from "../../application/usecases/results";
import { ButtonLink } from "../components/Button";
import { Card } from "../components/Card";
import { Notice } from "../components/Notice";
import { EmptyState, Loading, PageError } from "../components/States";
import { ExplorationPanel } from "../features/findings/ExplorationPanel";
import { InsightList } from "../features/findings/InsightCard";
import { NextActions } from "../features/findings/NextActions";
import { RunHeader } from "../features/runs/RunHeader";
import { RunSelector } from "../features/runs/RunSelector";
import { useAsync } from "../hooks/useAsync";
import { useI18n } from "../i18n/I18nProvider";
import { projectHash } from "../router/routes";
import { navigate } from "../router/useRoute";
import { useUseCases } from "../services/context";
import { ProjectFrame } from "./ProjectFrame";

export function FindingsPage({ projectId, runId }: { projectId: string; runId?: string | undefined }) {
  const { t } = useI18n();
  const { results, exports } = useUseCases();
  const state = useAsync(() => results.loadFindings(projectId, runId), [results, projectId, runId]);
  if (state.status === "loading") return <Loading />;
  if (state.status === "error") return <PageError error={state.error} onRetry={state.reload} backHref={projectHash(projectId, undefined, runId)} />;
  const { project, runs, selection, data: insights = [] } = state.data;
  const run = selection.run;
  return (
    <ProjectFrame project={project} current="findings" runId={run?.id} title={t("findings.title")} subtitle={t("findings.lead")}
      actions={run && <ButtonLink variant="primary" href={exports.reportLink(project.id, run.id)} download>{t("project.downloadReport")}</ButtonLink>}>
      {selection.requestedRunMissing && <Notice kind="warning">{t("run.notFoundNotice")}</Notice>}
      <Card title={t("findings.run")}>
        <RunHeader run={run} />
        <RunSelector runs={runs} selected={run} onSelect={(id) => navigate({ name: "findings", projectId: project.id, runId: id })} />
      </Card>
      {run?.metrics?.exploration ? (
        <ExplorationPanel projectId={project.id} exploration={run.metrics.exploration} />
      ) : run ? (
        <>
          <Card title={t("project.insightsForRun")} description={t("findings.hint")}>
            {insights.length ? <InsightList insights={insights} /> : <EmptyState>{t("project.noInsights")}</EmptyState>}
          </Card>
          <Card title={t("next.title")}><NextActions projectId={project.id} run={run} next={nextActions(runs, insights)} /></Card>
        </>
      ) : (
        <EmptyState action={<ButtonLink variant="primary" href={projectHash(project.id, "analysis")}>{t("nextStep.analysis.action")}</ButtonLink>}>{t("run.noCompletedYet")}</EmptyState>
      )}
    </ProjectFrame>
  );
}
