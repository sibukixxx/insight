import { useI18n } from "../../i18n/I18nProvider";
import styles from "./Onboarding.module.css";

/** The three steps of every analysis, shown until the first project exists. */
export function GettingStarted() {
  const { t } = useI18n();
  const steps = [
    ["onboarding.step1.title", "onboarding.step1.hint"],
    ["onboarding.step2.title", "onboarding.step2.hint"],
    ["onboarding.step3.title", "onboarding.step3.hint"],
  ] as const;
  return (
    <ol class={styles.steps} aria-label={t("onboarding.stepsLabel")}>
      {steps.map(([title, hint], i) => (
        <li key={title} class={styles.step}>
          <span class={styles.stepNum} aria-hidden="true">{String(i + 1)}</span>
          <span class={styles.stepTitle}>{t(title)}</span>
          <span class={styles.stepHint}>{t(hint)}</span>
        </li>
      ))}
    </ol>
  );
}
