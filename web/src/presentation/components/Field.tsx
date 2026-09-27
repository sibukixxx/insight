import type { ComponentChildren } from "preact";
import styles from "./Form.module.css";

interface Props {
  readonly label: string;
  readonly htmlFor: string;
  readonly hint?: ComponentChildren;
  readonly hintId?: string;
  readonly optionalLabel?: string;
  readonly requirement?: "required" | "required-later" | "optional";
  readonly requirementLabel?: string;
  readonly example?: ComponentChildren;
  readonly error?: string;
  readonly children: ComponentChildren;
}

export function Field({ label, htmlFor, hint, hintId, optionalLabel, requirement, requirementLabel, example, error, children }: Props) {
  return (
    <div class={styles.field}>
      <label class={styles.label} for={htmlFor}>
        {label}
        {requirementLabel && <span aria-hidden="true" class={requirement === "required" ? styles.required : styles.optional}> {requirementLabel}</span>}
        {!requirementLabel && optionalLabel && <span aria-hidden="true" class={styles.optional}> {optionalLabel}</span>}
      </label>
      {children}
      {hint && <p class={styles.hint} id={hintId ?? `${htmlFor}-hint`}>{hint}</p>}
      {example && <p class={styles.hint}>{example}</p>}
      {error && <p class={styles.error} id={`${htmlFor}-error`} role="alert">{error}</p>}
    </div>
  );
}

export const formStyles = styles;
