import { useState } from "preact/hooks";
import type { RunHistory } from "../../application/usecases/history";
import { ATTRIBUTION_LABELS, AXIS_STATE_LABELS, RUN_STATUS_LABELS } from "../../domain/codes";
import type { RunComparison } from "../../domain/models";
import { completedRuns, runProvenance, shortFingerprint } from "../../domain/runs";
import { Badge, type Tone } from "../components/Badge";
import { Button } from "../components/Button";
import { Card } from "../components/Card";
import { Field, formStyles } from "../components/Field";
import { EmptyState, ErrorState, Loading, PageError } from "../components/States";
import { runLabel } from "../features/runs/runLabel";
import { useAsync } from "../hooks/useAsync";
import { useI18n } from "../i18n/I18nProvider";
import { projectHash } from "../router/routes";
import { navigate } from "../router/useRoute";
import { useUseCases } from "../services/context";
import { ProjectFrame } from "./ProjectFrame";
import styles from "./RunsPage.module.css";

const STATUS_TONES: Readonly<Record<string, Tone>> = { completed: "success", failed: "danger", running: "accent", queued: "neutral" };

export function RunsPage({ projectId, from, to }: { projectId: string; from?: string | undefined; to?: string | undefined }) {
  const { history } = useUseCases();
  const state = useAsync(() => history.loadHistory(projectId), [history, projectId]);
  if (state.status === "loading") return <Loading />;
  if (state.status === "error") return <PageError error={state.error} onRetry={state.reload} />;
  return <RunsView data={state.data} from={from} to={to} />;
}

function RunsView({ data, from, to }: { data: RunHistory; from: string | undefined; to: string | undefined }) {
  const i18n = useI18n();
  const { t, label, dateTime } = i18n;
  const { project, runs } = data;
  const completed = completedRuns(runs);
  const pair = from && to ? ([from, to] as const) : data.defaultPair;
  return (
    <ProjectFrame project={project} current="runs" title={t("runs.title")} subtitle={t("runs.lead")}>
      <Card title={t("runs.history", { count: runs.length })}>
        {runs.length === 0 ? <EmptyState>{t("analysis.none")}</EmptyState> : (
          <div class={styles.tableWrap}>
            <table class={styles.table}>
              <thead><tr>
                <th scope="col">{t("runs.col.created")}</th><th scope="col">{t("runs.col.status")}</th><th scope="col">{t("runs.col.mode")}</th>
                <th scope="col">{t("provenance.question")}</th><th scope="col" class={styles.num}>{t("runs.col.insights")}</th><th scope="col">{t("provenance.input")}</th><th scope="col">{t("runs.col.run")}</th>
              </tr></thead>
              <tbody>
                {runs.map((run) => {
                  const input = shortFingerprint(run.inputFingerprint);
                  return (
                    <tr key={run.id}>
                      <td>{dateTime(run.createdAt)}</td>
                      <td><Badge tone={STATUS_TONES[run.status] ?? "neutral"}>{label(RUN_STATUS_LABELS, run.status)}</Badge>{run.status === "failed" && run.error ? <div>{run.error}</div> : null}</td>
                      <td>{runProvenance(run).mode ?? t("common.notRecorded")}</td>
                      <td>{run.researchQuestion ?? "-"}</td>
                      <td class={styles.num}>{run.metrics?.finalInsightCount ?? "-"}</td>
                      <td><code>{input.kind === "value" ? input.value : t("common.notRecorded")}</code></td>
                      <td>{run.status === "completed" ? <a href={projectHash(project.id, "findings", run.id)}>{t("runs.open")}</a> : <code>{run.id}</code>}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </Card>
      <Card title={t("compare.title")} description={t("compare.hint")}>
        {completed.length < 2 ? <EmptyState>{t("compare.needTwo")}</EmptyState> : (
          <ComparePicker projectId={project.id} options={completed.map((r) => [r.id, runLabel(r, i18n)] as const)} pair={pair} />
        )}
        {pair && completed.length >= 2 && <ComparisonResult projectId={project.id} fromId={pair[0]} toId={pair[1]} />}
      </Card>
    </ProjectFrame>
  );
}

function ComparePicker({ projectId, options, pair }: { projectId: string; options: readonly (readonly [string, string])[]; pair: readonly [string, string] | undefined }) {
  const { t } = useI18n();
  const [a, setA] = useState(pair?.[0] ?? options[1]?.[0] ?? "");
  const [b, setB] = useState(pair?.[1] ?? options[0]?.[0] ?? "");
  const select = (id: string, value: string, set: (v: string) => void, labelText: string) => (
    <Field label={labelText} htmlFor={id} requirement="optional">
            {(control) => (<select {...control} id={id} class={formStyles.control} value={value} onChange={(e) => set(e.currentTarget.value)}>
        {options.map(([runId, text]) => <option key={runId} value={runId}>{text}</option>)}
      </select>)}
          </Field>
  );
  return (
    <form class={styles.pair} onSubmit={(e) => { e.preventDefault(); navigate({ name: "runs", projectId, from: a, to: b }); }}>
      {select("compare-a", a, setA, t("compare.from"))}
      {select("compare-b", b, setB, t("compare.to"))}
      <div><Button type="submit" variant="primary" disabled={!a || !b || a === b}>{t("compare.action")}</Button></div>
    </form>
  );
}

function ComparisonResult({ projectId, fromId, toId }: { projectId: string; fromId: string; toId: string }) {
  const { history } = useUseCases();
  const state = useAsync(() => history.compareRuns(projectId, fromId, toId), [history, projectId, fromId, toId]);
  if (state.status === "loading") return <Loading />;
  if (state.status === "error") return <ErrorState error={state.error} onRetry={state.reload} />;
  return <ComparisonView c={state.data} />;
}

function ComparisonView({ c }: { c: RunComparison }) {
  const { t, label, number } = useI18n();
  const fmt = (v: number | null) => (v === null ? t("common.notRecorded") : number(v, 3));
  return (
    <section aria-label={t("compare.result")}>
      <h3>{t("compare.result")}</h3>
      <div class={styles.axes}>
        <div class={styles.axis}><span class={styles.axisLabel}>{t("compare.attribution")}</span><strong>{label(ATTRIBUTION_LABELS, c.attribution)}</strong><code>{c.attribution}</code></div>
        <div class={styles.axis}>
          <span class={styles.axisLabel}>{t("compare.input")}</span><strong>{label(AXIS_STATE_LABELS, c.inputState)}</strong>
          <span>{t("compare.documentsChanged", { added: c.documentsAdded.length, removed: c.documentsRemoved.length })}</span>
          {c.researchQuestionChange && <span>{t("compare.questionChanged", { from: c.researchQuestionChange.from || "-", to: c.researchQuestionChange.to || "-" })}</span>}
        </div>
        <div class={styles.axis}>
          <span class={styles.axisLabel}>{t("compare.execution")}</span><strong>{label(AXIS_STATE_LABELS, c.executionState)}</strong>
          {c.executionChanges.length > 0 && <ul class={styles.changes}>{c.executionChanges.map((ch) => <li key={ch.field}><code>{ch.field}</code>{": "}{ch.from || "-"} {"→"} {ch.to || "-"}</li>)}</ul>}
        </div>
        <div class={styles.axis}>
          <span class={styles.axisLabel}>{t("compare.insights")}</span>
          <span>{t("compare.insightCounts", { added: c.insightsAdded.length, removed: c.insightsRemoved.length, matched: c.insightsMatched })}</span>
        </div>
      </div>
      {c.metrics.length > 0 && (
        <div class={styles.tableWrap}>
          <table class={styles.table}>
            <thead><tr><th scope="col">{t("compare.metric")}</th><th scope="col" class={styles.num}>{t("compare.from")}</th><th scope="col" class={styles.num}>{t("compare.to")}</th><th scope="col" class={styles.num}>{t("compare.delta")}</th></tr></thead>
            <tbody>{c.metrics.map((m) => <tr key={m.metric}><td><code>{m.metric}</code></td><td class={styles.num}>{fmt(m.from)}</td><td class={styles.num}>{fmt(m.to)}</td><td class={styles.num}>{fmt(m.delta)}</td></tr>)}</tbody>
          </table>
        </div>
      )}
      <ul class={styles.explanation}>{c.explanation.map((line) => <li key={line}>{line}</li>)}</ul>
    </section>
  );
}
