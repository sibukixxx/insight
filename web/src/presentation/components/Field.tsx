import type { ComponentChildren } from "preact";
import { useI18n } from "../i18n/I18nProvider";
import styles from "./Form.module.css";

export type Requirement = "required" | "required-later" | "optional";
export interface FieldControlProps {
  readonly id: string;
  readonly required: boolean;
  readonly "aria-required": boolean;
  readonly "aria-invalid": boolean;
  readonly "aria-describedby": string;
}
interface Props {
  readonly label: string;
  readonly htmlFor: string;
  readonly requirement: Requirement;
  readonly requirementLabel?: string;
  readonly hint?: ComponentChildren;
  readonly hintId?: string;
  readonly example?: ComponentChildren;
  readonly error?: string | undefined;
  /** Spread onto the actual control; Field never clones or mutates children. */
  readonly children: (control: FieldControlProps) => ComponentChildren;
}
const requirementKeys = { required: "common.required", "required-later": "common.requiredLater", optional: "common.optional" } as const;

export function Field({ label, htmlFor, requirement, requirementLabel, hint, hintId = `${htmlFor}-hint`, example, error, children }: Props) {
  const { t } = useI18n();
  const requirementId = `${htmlFor}-requirement`;
  const exampleId = `${htmlFor}-example`;
  const errorId = `${htmlFor}-error`;
  const describedBy = [requirementId, hint && hintId, example && exampleId, error && errorId].filter(Boolean).join(" ");
  return (
    <div class={styles.field}>
      <div class={styles.labelRow}>
        <label class={styles.label} for={htmlFor}>{label}</label>
        <span id={requirementId} class={requirement === "required" ? styles.required : styles.optional}>{requirementLabel ?? t(requirementKeys[requirement])}</span>
      </div>
      {children({ id: htmlFor, required: requirement === "required", "aria-required": requirement === "required", "aria-invalid": !!error, "aria-describedby": describedBy })}
      {hint && <p class={styles.hint} id={hintId}>{hint}</p>}
      {example && <p class={styles.hint} id={exampleId}>{example}</p>}
      {error && <p class={styles.error} id={errorId}>{error}</p>}
    </div>
  );
}
export const formStyles = styles;
