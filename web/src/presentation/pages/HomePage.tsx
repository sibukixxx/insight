import { useState } from "preact/hooks";
import { ValidationSummary } from "../components/ValidationSummary";
import { useFormValidation } from "../hooks/useFormValidation";
import { Button } from "../components/Button";
import { Card } from "../components/Card";
import { Field, formStyles } from "../components/Field";
import { Notice } from "../components/Notice";
import { EmptyState, ErrorState, Loading } from "../components/States";
import { GettingStarted } from "../features/onboarding/GettingStarted";
import onboarding from "../features/onboarding/Onboarding.module.css";
import { useAsync } from "../hooks/useAsync";
import { useI18n } from "../i18n/I18nProvider";
import { errorMessage } from "../errors";
import { navigate } from "../router/useRoute";
import { useBuild, useUseCases } from "../services/context";
import styles from "./Page.module.css";

export function HomePage() {
  const { t, tRich, dateTime } = useI18n();
  const build = useBuild();
  const { dashboard } = useUseCases();
  const state = useAsync(() => dashboard.loadDashboard(), [dashboard]);
  const [error, setError] = useState<string | undefined>(undefined);
  const [busy, setBusy] = useState(false);
  const validation = useFormValidation();
  const [name, setName] = useState("");

  const firstRun = state.status === "ok" && state.data.projects.length === 0;

  const openSample = () => {
    setBusy(true);
    setError(undefined);
    dashboard.openSample(build).then(
      (p) => navigate({ name: "workspace", projectId: p.id }),
      (e: unknown) => { setError(errorMessage(e, t)); setBusy(false); },
    );
  };
  const create = (e: Event) => {
    e.preventDefault();
    if (busy || !validation.validate(name.trim() ? [] : [{ id: "new-project-name", label: "home.projectNamePrompt", message: "form.requiredValue" }])) return;
    setBusy(true);
    setError(undefined);
    dashboard.createProject(name).then(
      (p) => navigate({ name: "input", projectId: p.id }),
      (err: unknown) => { setError(errorMessage(err, t)); setBusy(false); },
    );
  };

  return (
    <div class={styles.stack}>
      <div class={onboarding.hero}>
        <h1>Insight Lab</h1>
        <p class={onboarding.lead}>{t("home.lead")}</p>
        {firstRun && <GettingStarted />}
      </div>

      {error && <Notice kind="error">{error}</Notice>}

      <div class={onboarding.paths}>
        <Card title={t("home.sample.title")}>
          <div class={onboarding.path}>
            {build.demoBuild ? (
              <>
                <p class={onboarding.hint}>{t("home.sample.hint")}</p>
                <div><Button id="try-demo" variant={firstRun ? "primary" : "secondary"} onClick={openSample} disabled={busy}>{t("home.tryDemo")}</Button></div>
              </>
            ) : (
              <>
                <p class={onboarding.hint}>{tRich("home.deliveryHint", { command: <code>make build-demo</code> })}</p>
                <div><Button id="try-demo" disabled>{t("home.tryDemo")}</Button></div>
              </>
            )}
          </div>
        </Card>
        <Card title={t("home.newProject")}>
          <form class={onboarding.path} noValidate onSubmit={create}>
            <ValidationSummary errors={validation.errors} />
            <p class={onboarding.hint}>{t("home.newProjectHint")}</p>
            <div class={onboarding.inline}>
              <Field label={t("home.projectNamePrompt")} htmlFor="new-project-name" hint={t("form.projectHint")} example={t("form.projectExample")} error={validation.error("new-project-name")} requirement="required" requirementLabel={t("common.required")}>
            {(control) => (<input {...control} aria-label={t("home.projectNamePrompt")} class={formStyles.control} type="text" value={name} onInput={(e) => { setName(e.currentTarget.value); validation.clear("new-project-name"); }} />)}
          </Field>
              <Button id="new-project" type="submit" variant={firstRun && !build.demoBuild ? "primary" : "secondary"} disabled={busy}>{t("home.createProject")}</Button>
            </div>
          </form>
        </Card>
      </div>

      <Card title={t("home.projects")}>
        {state.status === "loading" && <Loading />}
        {state.status === "error" && <ErrorState error={state.error} onRetry={state.reload} />}
        {state.status === "ok" && (state.data.projects.length === 0
          ? <EmptyState>{t("home.noProjects")}</EmptyState>
          : (
            <ul class={styles.list}>
              {state.data.projects.map((p) => (
                <li key={p.id}>
                  <a class={styles.projectItem} href={`#/projects/${encodeURIComponent(p.id)}`}>
                    <span>
                      <span class={styles.projectName}>{p.name}</span>
                      <span class={styles.meta}> {"·"} {dateTime(p.createdAt)}</span>
                    </span>
                    <span>{t("home.open")} {"→"}</span>
                  </a>
                </li>
              ))}
            </ul>
          ))}
      </Card>
    </div>
  );
}
