import type { ComponentChildren } from "preact";
import styles from "./Form.module.css";

interface Props {
  readonly label: string;
  readonly htmlFor: string;
  readonly hint?: ComponentChildren;
  readonly hintId?: string;
  readonly optionalLabel?: string;
  readonly children: ComponentChildren;
}

export function Field({ label, htmlFor, hint, hintId, optionalLabel, children }: Props) {
  return (
    <div class={styles.field}>
      <label class={styles.label} for={htmlFor}>
        {label}
        {optionalLabel && <span class={styles.optional}> {optionalLabel}</span>}
      </label>
      {children}
      {hint && <p class={styles.hint} id={hintId}>{hint}</p>}
    </div>
  );
}

export const formStyles = styles;
