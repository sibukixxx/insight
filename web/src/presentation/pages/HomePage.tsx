import { useState } from "preact/hooks";
import { ValidationSummary } from "../components/ValidationSummary";
import { useFormValidation } from "../hooks/useFormValidation";
import { Button } from "../components/Button";
import { Card } from "../components/Card";
import { Field, formStyles } from "../components/Field";
import { Notice } from "../components/Notice";
import { EmptyState, ErrorState, Loading } from "../components/States";
import { GettingStarted } from "../features/onboarding/GettingStarted";
import { SampleGallery } from "../features/samples/Samples";
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
  const [question, setQuestion] = useState("");

  const firstRun = state.status === "ok" && state.data.projects.length === 0;
  // With projects, Home is a place to resume: no introduction, samples on request (#141).
  const returning = state.status === "ok" && !firstRun;
  const [showSamples, setShowSamples] = useState(false);
  const galleryOpen = firstRun || showSamples;
  const revealSamples = () => {
    setShowSamples(true);
    setTimeout(() => jumpTo("samples"), 0);
  };

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

  const startQuestion = (e: Event) => {
    e.preventDefault();
    if (busy || !validation.validate(question.trim() ? [] : [{ id: "home-question", label: "home.question.label", message: "form.requiredValue" }])) return;
    setBusy(true);
    setError(undefined);
    dashboard.createFromQuestion(question).then(
      (p) => navigate({ name: "analysis", projectId: p.id }),
      (err: unknown) => { setError(errorMessage(err, t)); setBusy(false); },
    );
  };

  const jumpTo = (id: string) => {
    const target = document.getElementById(id);
    target?.scrollIntoView({ block: "start" });
    target?.focus({ preventScroll: true });
  };

  const projectsCard = (
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
  );

  return (
    <div class={styles.stack}>
      <section class={onboarding.hero} aria-labelledby="home-title">
        <p class={onboarding.eyebrow}>{t("app.tagline")}</p>
        <h1 id="home-title">Insight Lab</h1>
        {!returning && <p class={onboarding.lead}>{t("home.lead")}</p>}
        <div class={onboarding.heroActions}>
          {build.demoBuild && <Button variant="primary" onClick={revealSamples}>{t("home.hero.trySample")}</Button>}
          <button type="button" class={onboarding.onInk} onClick={() => jumpTo("new-project-name")}>{t("home.hero.ownCsv")}</button>
        </div>
        {firstRun && <GettingStarted />}
      </section>

      {error && <Notice kind="error">{error}</Notice>}

      {returning && projectsCard}

      <Card title={t("home.question.title")}>
        <form id="question-first" class={onboarding.path} noValidate onSubmit={startQuestion}>
          <p class={onboarding.hint}>{t("home.question.hint")}</p>
          <Field label={t("home.question.label")} htmlFor="home-question" hint={t("home.question.fieldHint")} error={validation.error("home-question")} requirement="required" requirementLabel={t("common.required")}>
            {(control) => (<textarea {...control} class={formStyles.control} rows={3} value={question} placeholder={t("home.question.placeholder")} onInput={(e) => { setQuestion(e.currentTarget.value); validation.clear("home-question"); }} />)}
          </Field>
          <div><Button id="start-question" type="submit" variant="primary" disabled={busy}>{t("home.question.start")}</Button></div>
        </form>
      </Card>

      {build.demoBuild && returning && (
        <div>
          <Button id="toggle-samples" aria-expanded={showSamples} aria-controls="samples" onClick={() => setShowSamples(!showSamples)}>
            {showSamples ? t("home.samples.hide") : t("home.samples.show")}
          </Button>
        </div>
      )}
      {build.demoBuild && state.status === "ok" && galleryOpen && <SampleGallery projects={state.data.projects} onError={setError} compact={returning} />}

      <div class={onboarding.paths}>
        <Card title={t("home.sample.title")}>
          <div class={onboarding.path}>
            {build.demoBuild ? (
              <>
                <p class={onboarding.hint}>{t("home.sample.hint")}</p>
                <div><Button id="try-demo" onClick={openSample} disabled={busy}>{t("home.tryDemo")}</Button></div>
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

      {!returning && projectsCard}
    </div>
  );
}
