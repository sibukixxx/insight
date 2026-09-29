import type { RunExploration } from "../../../domain/models";
import { Badge } from "../../components/Badge";
import { ButtonLink } from "../../components/Button";
import { Card } from "../../components/Card";
import { Notice } from "../../components/Notice";
import { useI18n } from "../../i18n/I18nProvider";
import { buildHash } from "../../router/routes";
import styles from "./Exploration.module.css";

/**
 * A question-only run's result (#158). It is always shown as unverified:
 * the candidates are model proposals made with no evidence, so nothing here
 * carries an evidence, confidence or verified state.
 */
export function ExplorationPanel({ projectId, exploration }: { projectId: string; exploration: RunExploration }) {
  const { t } = useI18n();
  return (
    <div id="exploration-result" data-verified={String(exploration.verified)} class={styles.stack}>
      <Card title={t("exploration.title")} description={t("exploration.lead")}
        actions={<Badge tone="warning" title={t("exploration.unverifiedHint")}>{t("exploration.unverified")}</Badge>}>
        <p class={styles.question}><span class={styles.label}>{t("exploration.question")}</span> {exploration.question}</p>
        <ol class={styles.candidates}>
          {exploration.candidates.map((c, i) => (
            <li key={i} class={styles.candidate} data-candidate={i + 1}>
              <div class={styles.head}>
                <span class={styles.letter} aria-hidden="true">{String.fromCharCode(65 + i)}</span>
                <h3 class={styles.candidateTitle}>{c.title}</h3>
                <Badge tone="violet">{t("exploration.candidateBadge")}</Badge>
              </div>
              <p class={styles.body}>{c.explanation}</p>
              {c.competingExplanations.length > 0 && (
                <section aria-label={t("exploration.competing")}>
                  <h4 class={styles.sub}>{t("exploration.competing")}</h4>
                  <ul class={styles.points}>{c.competingExplanations.map((x, j) => <li key={j}>{x.title ? <strong>{x.title}: </strong> : null}{x.explanation}</li>)}</ul>
                </section>
              )}
              <section aria-label={t("exploration.falsification")}>
                <h4 class={styles.sub}>{t("exploration.falsification")}</h4>
                <ul class={styles.points}>{c.falsificationConditions.map((x, j) => <li key={j}>{x}</li>)}</ul>
              </section>
              <section aria-label={t("exploration.requiredData")}>
                <h4 class={styles.sub}>{t("exploration.requiredData")}</h4>
                <ul class={styles.points}>{c.requiredData.map((x, j) => <li key={j}>{x.description}{x.why ? <span class={styles.why}> — {x.why}</span> : null}</li>)}</ul>
              </section>
            </li>
          ))}
        </ol>
      </Card>
      <Card title={t("exploration.limitations")}>
        <Notice kind="warning">{t("exploration.noEvidenceNote")}</Notice>
        {exploration.limitations.length > 0 && <ul class={styles.points}>{exploration.limitations.map((x, j) => <li key={j}>{x}</li>)}</ul>}
      </Card>
      <Card title={t("exploration.nextTitle")}>
        <p class={styles.body}>{t("exploration.nextHint")}</p>
        <ButtonLink id="add-evidence-to-verify" variant="primary" href={buildHash({ name: "input", projectId })}>{t("exploration.addEvidence")} {"→"}</ButtonLink>
      </Card>
    </div>
  );
}
