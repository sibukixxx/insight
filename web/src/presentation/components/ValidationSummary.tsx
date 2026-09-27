import { useI18n } from "../i18n/I18nProvider";
import styles from "./Form.module.css";
export interface FieldError { readonly id: string; readonly label: string; readonly message: string }
export function focusField(id: string) {
  const control = document.getElementById(id);
  for (let parent = control?.parentElement; parent; parent = parent.parentElement) {
    if (parent instanceof HTMLDetailsElement) parent.open = true;
  }
  control?.focus();
}
export function ValidationSummary({ errors }: { errors: readonly FieldError[] }) {
  const { t } = useI18n();
  if (!errors.length) return null;
  return <div class={styles.summary} role="alert">
    <p>{t("form.errorSummary", { count: errors.length })}</p>
    <ul>{errors.map((error) => <li key={error.id}>
      <a href={`#${error.id}`} onClick={(e) => { e.preventDefault(); focusField(error.id); }}>{error.label}{": "}{error.message}</a>
    </li>)}</ul>
  </div>;
}
