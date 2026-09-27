import styles from "./Stepper.module.css";

export interface StepItem {
  readonly label: string;
  readonly state: "done" | "current" | "todo";
  readonly stateLabel: string;
  readonly href?: string;
}

export function Stepper({ steps, label }: { steps: readonly StepItem[]; label: string }) {
  return (
    <ol class={styles.stepper} aria-label={label}>
      {steps.map((step, i) => (
        <li key={step.label} class={[styles.step, step.state === "done" && styles.done, step.state === "current" && styles.current].filter(Boolean).join(" ")} aria-current={step.state === "current" ? "step" : undefined}>
          <span class={styles.num} aria-hidden="true">{step.state === "done" ? "✓" : String(i + 1)}</span>
          <span class={styles.text}>
            {step.href ? <a href={step.href}>{step.label}</a> : step.label}
            <span class={styles.state}>{step.stateLabel}</span>
          </span>
        </li>
      ))}
    </ol>
  );
}
