import { useState } from "preact/hooks";
import { ValidationSummary } from "../components/ValidationSummary";
import { useFormValidation } from "../hooks/useFormValidation";
import { Button } from "../components/Button";
import { Card } from "../components/Card";
import { Field, formStyles } from "../components/Field";
import { Notice } from "../components/Notice";
import { EmptyState, Loading, PageError } from "../components/States";
import { useAsync } from "../hooks/useAsync";
import { useI18n } from "../i18n/I18nProvider";
import { errorMessage } from "../errors";
import { buildHash } from "../router/routes";
import { navigate } from "../router/useRoute";
import { useUseCases } from "../services/context";
import { ProjectFrame } from "./ProjectFrame";
import styles from "./Page.module.css";

export function ResearchPage({ projectId }: { projectId: string }) {
  const { t } = useI18n();
  const { research } = useUseCases();
  const state = useAsync(() => research.loadResearch(projectId), [research, projectId]);
  const [error, setError] = useState<string | undefined>(undefined);
  const [busy, setBusy] = useState(false);
  const validation = useFormValidation();
  if (state.status === "loading") return <Loading />;
  if (state.status === "error") return <PageError error={state.error} onRetry={state.reload} />;
  const { project, runs, hasCompletedAnalysis } = state.data;
  const create = (e: Event) => {
    e.preventDefault();
    const question = String(new FormData(e.currentTarget as HTMLFormElement).get("question") ?? "");
    if (busy || !validation.validate(question.trim() ? [] : [{ id: "research-new-question", label: "research.question", message: "form.requiredValue" }])) return;
    setBusy(true);
    setError(undefined);
    research.createResearch(project.id, question).then(
      (run) => navigate({ name: "promotion", researchRunId: run.id }),
      (err: unknown) => { setError(errorMessage(err, t)); setBusy(false); },
    );
  };
  return (
    <ProjectFrame project={project} current="research" title={t("research.title")} subtitle={t("research.lead")}>
      <Card title={t("research.list")}>
        {runs.length === 0 ? <EmptyState>{t("research.none")}</EmptyState> : (
          <ul class={styles.list}>{runs.map((r) => <li key={r.id}><a href={buildHash({ name: "promotion", researchRunId: r.id })}>{r.question}</a></li>)}</ul>
        )}
      </Card>
      <Card title={t("research.create")}>
        {!hasCompletedAnalysis && <Notice kind="info" spaced>{t("research.needsRun")}</Notice>}
        <form id="new-research" class={formStyles.form} noValidate onSubmit={create}>
          <ValidationSummary errors={validation.errors} />
          <Field label={t("research.question")} htmlFor="research-new-question" error={validation.error("research-new-question")} requirement="required">
            {(control) => (<input {...control} name="question" onInput={() => validation.clear("research-new-question")} class={formStyles.control} />)}
          </Field>
          {error && <Notice kind="error">{error}</Notice>}
          <div class={formStyles.actions}><Button type="submit" variant="primary" disabled={busy}>{t("research.create")}</Button></div>
        </form>
      </Card>
    </ProjectFrame>
  );
}
