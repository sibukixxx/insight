import { useState } from "preact/hooks";
import { ValidationSummary } from "../components/ValidationSummary";
import { useFormValidation } from "../hooks/useFormValidation";
import type { PromotionView } from "../../application/usecases/research";
import { CONTRIBUTION_LABELS, PROMOTION_JUDGMENTS, PROMOTION_STATE_LABELS, TERMINAL_PROMOTION_STATES } from "../../domain/codes";
import type { PromotionReview, ResearchIteration } from "../../domain/models";
import { Button, ButtonLink } from "../components/Button";
import { Card } from "../components/Card";
import { Field, formStyles } from "../components/Field";
import { Notice } from "../components/Notice";
import { PageHeader } from "../components/PageHeader";
import { Loading, PageError } from "../components/States";
import { useAsync } from "../hooks/useAsync";
import { useI18n } from "../i18n/I18nProvider";
import { errorMessage } from "../errors";
import { projectHash } from "../router/routes";
import { useUseCases } from "../services/context";
import styles from "./Page.module.css";

export function PromotionPage({ researchRunId }: { researchRunId: string }) {
  const { t } = useI18n();
  const { research } = useUseCases();
  const state = useAsync(() => research.openPromotion(researchRunId), [research, researchRunId]);
  if (state.status === "loading") return <Loading />;
  if (state.status === "error") return <PageError error={state.error} onRetry={state.reload} />;
  const { run, iteration } = state.data;
  return (
    <>
      <PageHeader back={{ href: projectHash(run.projectId, "research"), label: t("nav.backToResearch") }} eyebrow={t("research.title")} title={run.question} />
      {iteration ? <Promotion view={state.data} iteration={iteration} onChanged={state.reload} /> : <Notice kind="error">{t("promotion.noIterations")}</Notice>}
    </>
  );
}

function Promotion({ view, iteration, onChanged }: { view: PromotionView; iteration: ResearchIteration; onChanged: () => void }) {
  const { t, label, tRich } = useI18n();
  const { research, exports } = useUseCases();
  const [error, setError] = useState<string | undefined>(undefined);
  const [busy, setBusy] = useState(false);
  const validation = useFormValidation();
  const runId = view.run.id;
  const state = iteration.promotion?.state || "DRAFT";
  const terminal = TERMINAL_PROMOTION_STATES.includes(state);

  const submit = (action: Promise<void>) => {
    setBusy(true);
    setError(undefined);
    action.then(onChanged, (e: unknown) => setError(errorMessage(e, t))).finally(() => setBusy(false));
  };
  const onReview = (e: Event) => {
    e.preventDefault();
    const data = new FormData(e.currentTarget as HTMLFormElement);
    if (busy || !validation.validate(data.get("contribution") ? [] : [{ id: "promotion-contribution", label: "promotion.contribution", message: "form.selectValue" }])) return;
    const review = Object.fromEntries(PROMOTION_JUDGMENTS.map(([key]) => [key, data.has(key)])) as Omit<PromotionReview, "contribution">;
    submit(research.submitReview(runId, iteration.id, { ...review, contribution: String(data.get("contribution") ?? "") }));
  };

  return (
    <div class={styles.stack}>
      <Card title={t("promotion.state")}>
        <h2>{label(PROMOTION_STATE_LABELS, state)} <code>{state}</code></h2>
        <p class={styles.hint}>{t("promotion.iteration", { sequence: iteration.sequence })}</p>
        {(iteration.promotion?.reasons.length ?? 0) > 0 && <ul>{iteration.promotion?.reasons.map((r) => <li key={r}>{r}</li>)}</ul>}
        <div class={styles.actions}>
          <ButtonLink size="small" href={exports.researchReportLink(runId)} download>{t("promotion.report")}</ButtonLink>
          <ButtonLink size="small" href={exports.researchArtifactLink(runId)} target="_blank" rel="noopener noreferrer">{t("promotion.currentArtifact")}</ButtonLink>
          {iteration.approvedArtifactReference !== undefined && (
            <ButtonLink size="small" href={exports.approvedArtifactLink(runId)} target="_blank" rel="noopener noreferrer">{t("promotion.approvedArtifact")}</ButtonLink>
          )}
        </div>
        {iteration.approvedArtifactReference !== undefined && <p class={styles.meta}>{tRich("promotion.approvedReference", { reference: <code>{iteration.approvedArtifactReference}</code> })}</p>}
        {Object.keys(iteration.checklist).length > 0 && (
          <ul class={styles.list} aria-label={t("promotion.checklist")}>
            {Object.entries(iteration.checklist).map(([key, ok]) => <li key={key}>{ok ? "✓" : t("promotion.missing")} <code>{key}</code></li>)}
          </ul>
        )}
      </Card>
      {error && <Notice kind="error">{error}</Notice>}
      {!terminal && (
        <>
          <Card title={t("promotion.reviewTitle")} description={t("promotion.reviewHint")}>
            <form id="promotion-review" class={formStyles.form} noValidate onSubmit={onReview}>
              <ValidationSummary errors={validation.errors} />
              <Field label={t("promotion.contribution")} htmlFor="promotion-contribution" error={validation.error("promotion-contribution")} requirement="required">
            {(control) => (<select {...control} name="contribution" onChange={() => validation.clear("promotion-contribution")} class={formStyles.control}>
                  <option value="">{t("promotion.chooseContribution")}</option>
                  {Object.keys(CONTRIBUTION_LABELS).map((code) => <option key={code} value={code}>{label(CONTRIBUTION_LABELS, code)}</option>)}
                </select>)}
          </Field>
              {PROMOTION_JUDGMENTS.map(([key, labelKey]) => (
                <label key={key} class={formStyles.check}><input type="checkbox" name={key} /> {t(labelKey)}</label>
              ))}
              <div class={formStyles.actions}><Button type="submit" variant="primary" disabled={busy}>{t("promotion.saveReview")}</Button></div>
            </form>
          </Card>
          <Card title={t("promotion.decision")}>
            <div class={styles.actions}>
              {state === "PUBLICATION_READY" && iteration.approvedArtifactReference !== undefined && (
                <>
                  <Button id="mark-published" onClick={() => submit(research.transition(runId, iteration.id, "PUBLISHED"))} disabled={busy}>{t("promotion.markPublished")}</Button>
                  <span class={styles.hint}>{t("promotion.markPublishedHint")}</span>
                </>
              )}
              <Button id="reject-publication" variant="danger" onClick={() => submit(research.transition(runId, iteration.id, "REJECTED_FOR_PUBLICATION"))} disabled={busy}>{t("promotion.reject")}</Button>
            </div>
          </Card>
        </>
      )}
    </div>
  );
}
