import type { MessageKey } from "../../../domain/messages";
import type { Insight } from "../../../domain/models";
import { useI18n } from "../../i18n/I18nProvider";
import styles from "./Evidence.module.css";

/**
 * The abductive triad: a surprising fact that broke an expectation, and a
 * hypothesis under which the fact would be a matter of course. Every step is
 * shown even when empty so a missing link is visible.
 */
export function AbductionTrail({ insight }: { insight: Insight }) {
  const { t } = useI18n();
  const steps: readonly (readonly [MessageKey, string, string | undefined])[] = [
    ["insight.expectation", insight.expectation, undefined],
    ["insight.surprisingFact", insight.surprisingFact, styles.stepFact],
    ["insight.hypothesis", insight.latentNeed, styles.stepHyp],
    ["insight.explanation", insight.rationale, styles.stepWhy],
  ];
  return (
    <ol class={styles.abduction}>
      {steps.map(([key, value, cls], i) => (
        <li key={key} class={[styles.step, cls].filter(Boolean).join(" ")}>
          <span class={styles.stepNum} aria-hidden="true">{String(i + 1)}</span>
          <div class={styles.stepBody}>
            <div class={styles.fieldLabel}>{t(key)}</div>
            <div>{value ? value : <span class={styles.missing}>{t("common.notRecordedTitle")}</span>}</div>
          </div>
        </li>
      ))}
    </ol>
  );
}
