import { splitPatterns } from "../../domain/evidence";
import { Card } from "../components/Card";
import { Notice } from "../components/Notice";
import { EmptyState, Loading, PageError } from "../components/States";
import { PatternBlock } from "../features/evidence/PatternBlock";
import { RunHeader } from "../features/runs/RunHeader";
import { RunSelector } from "../features/runs/RunSelector";
import { useAsync } from "../hooks/useAsync";
import { useI18n } from "../i18n/I18nProvider";
import { projectHash } from "../router/routes";
import { navigate } from "../router/useRoute";
import { useUseCases } from "../services/context";
import { ProjectFrame } from "./ProjectFrame";

export function PatternsPage({ projectId, runId }: { projectId: string; runId?: string | undefined }) {
  const { t } = useI18n();
  const { results } = useUseCases();
  const state = useAsync(() => results.loadPatterns(projectId, runId), [results, projectId, runId]);
  if (state.status === "loading") return <Loading />;
  if (state.status === "error") return <PageError error={state.error} onRetry={state.reload} backHref={projectHash(projectId, undefined, runId)} />;
  const { project, runs, selection, data = [] } = state.data;
  const run = selection.run;
  const { traces, repetitions } = splitPatterns(data);
  return (
    <ProjectFrame project={project} current="patterns" runId={run?.id} title={t("patterns.title", { project: project.name })} subtitle={t("patterns.hint")}>
      {selection.requestedRunMissing && <Notice kind="warning">{t("run.notFoundNotice")}</Notice>}
      <Card title={t("findings.run")}>
        <RunHeader run={run} />
        <RunSelector runs={runs} selected={run} onSelect={(id) => navigate({ name: "patterns", projectId: project.id, runId: id })} />
      </Card>
      <Card title={t("patterns.traces", { count: traces.length })} description={t("patterns.tracesHint")}>
        {traces.length ? traces.map((p) => <PatternBlock key={p.id} pattern={p} />) : <EmptyState>{t("patterns.noTraces")}</EmptyState>}
      </Card>
      <Card title={t("patterns.repetitions", { count: repetitions.length })} description={t("patterns.repetitionsHint")}>
        {repetitions.length ? repetitions.map((p) => <PatternBlock key={p.id} pattern={p} />) : <EmptyState>{t("patterns.noRepetitions")}</EmptyState>}
      </Card>
    </ProjectFrame>
  );
}
