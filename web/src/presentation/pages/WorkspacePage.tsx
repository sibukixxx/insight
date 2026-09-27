import type { Workspace } from "../../application/usecases/workspace";
import { nextActions } from "../../application/usecases/results";
import type { MessageKey } from "../../domain/messages";
import { ButtonLink } from "../components/Button";
import { Card } from "../components/Card";
import { Disclosure } from "../components/Disclosure";
import { Notice } from "../components/Notice";
import { EmptyState, Loading, PageError } from "../components/States";
import { Stepper, type StepItem } from "../components/Stepper";
import { InsightList } from "../features/findings/InsightCard";
import { NextActions } from "../features/findings/NextActions";
import { ProvenanceChips } from "../features/runs/ProvenanceChips";
import { RunList } from "../features/runs/RunList";
import { RunSelector } from "../features/runs/RunSelector";
import { RunStatus } from "../features/runs/RunStatus";
import { useRunWatcher } from "../features/analysis/useRunWatcher";
import { useAsync } from "../hooks/useAsync";
import { useI18n } from "../i18n/I18nProvider";
import { projectHash } from "../router/routes";
import { navigate } from "../router/useRoute";
import { useUseCases } from "../services/context";
import { ProjectFrame } from "./ProjectFrame";
import styles from "./Page.module.css";

const PREVIEW_INSIGHTS = 3;

export function WorkspacePage({ projectId, runId }: { projectId: string; runId?: string | undefined }) {
  const { workspace } = useUseCases();
  const state = useAsync(() => workspace.openWorkspace(projectId, runId), [workspace, projectId, runId]);
  if (state.status === "loading") return <Loading />;
  if (state.status === "error") return <PageError error={state.error} onRetry={state.reload} />;
  return <WorkspaceView ws={state.data} runId={runId} onChanged={state.reload} />;
}

function WorkspaceView({ ws, runId, onChanged }: { ws: Workspace; runId: string | undefined; onChanged: () => void }) {
  const { t } = useI18n();
  const { exports } = useUseCases();
  const { project, documents, runs, selection, insights } = ws;
  const selected = selection.run;
  const live = useRunWatcher(ws.activeRun?.id, () => {
    // A finished run becomes the shown run: drop ?run= so the latest completed run is picked.
    if (runId) navigate({ name: "workspace", projectId: project.id });
    else onChanged();
  });

  const step = (key: MessageKey, state: StepItem["state"], stateKey: MessageKey, href: string): StepItem =>
    ({ label: t(key), state, stateLabel: t(stateKey), href });
  const steps: StepItem[] = [
    step("workflow.input", documents.length ? "done" : "current", documents.length ? "workflow.inputDone" : "workflow.todo", projectHash(project.id, "input")),
    step("workflow.analysis", ws.step === "running" ? "current" : runs.some((r) => r.status === "completed") ? "done" : ws.step === "analysis" ? "current" : "todo",
      ws.step === "running" ? "workflow.running" : runs.some((r) => r.status === "completed") ? "workflow.analysisDone" : "workflow.todo", projectHash(project.id, "analysis")),
    step("workflow.results", selected ? "current" : "todo", selected ? "workflow.resultsReady" : "workflow.todo", projectHash(project.id, "findings", selected?.id)),
  ];

  return (
    <ProjectFrame project={project} current="overview" runId={selected?.id}
      subtitle={t("project.counts", { documents: documents.length, runs: runs.length, insights: insights.length })}
      actions={selected && <ButtonLink href={exports.reportLink(project.id, selected.id)} download>{t("project.downloadReport")}</ButtonLink>}>
      <Stepper steps={steps} label={t("workflow.label")} />
      {selection.requestedRunMissing && <Notice kind="warning">{t("run.notFoundNotice")}</Notice>}
      <NextStep ws={ws} />

      <Card title={t("project.analysis")} actions={<ButtonLink size="small" href={projectHash(project.id, "analysis")}>{t("workspace.openAnalysis")}</ButtonLink>}>
        <div id="analysis-panel"><RunStatus run={ws.runs[0]} live={live} /></div>
        <RunSelector runs={runs} selected={selected} onSelect={(id) => navigate({ name: "workspace", projectId: project.id, runId: id })} />
        {selected && (
          <Disclosure summary={t("advanced.provenance")}>
            <ProvenanceChips run={selected} />
            <RunList runs={runs} selected={selected} hrefFor={(id) => projectHash(project.id, undefined, id)} />
          </Disclosure>
        )}
      </Card>

      {selected && (
        <Card title={t("next.title")}>
          <NextActions projectId={project.id} run={selected} next={nextActions(runs, insights)} />
        </Card>
      )}

      <Card title={selected ? t("project.insightsForRun", { id: selected.id }) : t("project.insights")}
        actions={insights.length > PREVIEW_INSIGHTS && selected && <ButtonLink size="small" href={projectHash(project.id, "findings", selected.id)}>{t("workspace.allFindings", { count: insights.length })}</ButtonLink>}>
        {insights.length ? <InsightList insights={insights.slice(0, PREVIEW_INSIGHTS)} /> : <EmptyState>{t("project.noInsights")}</EmptyState>}
      </Card>
    </ProjectFrame>
  );
}

/** The single most useful next action for where the project is. */
function NextStep({ ws }: { ws: Workspace }) {
  const { t } = useI18n();
  const id = ws.project.id;
  const content: Readonly<Record<Workspace["step"], readonly [MessageKey, MessageKey, string] | undefined>> = {
    input: ["nextStep.input.title", "nextStep.input.action", projectHash(id, "input")],
    analysis: ["nextStep.analysis.title", "nextStep.analysis.action", projectHash(id, "analysis")],
    running: ["nextStep.running.title", "nextStep.running.action", projectHash(id, "analysis")],
    results: undefined,
  };
  const entry = content[ws.step];
  if (!entry) return null;
  const [title, action, href] = entry;
  return (
    <div class={styles.nextAction} role="region" aria-label={t("nextStep.label")}>
      <span class={styles.nextTitle}>{t(title)}</span>
      <ButtonLink variant="primary" href={href}>{t(action)} {"→"}</ButtonLink>
    </div>
  );
}
