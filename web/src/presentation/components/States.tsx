import type { ComponentChildren } from "preact";
import { useI18n } from "../i18n/I18nProvider";
import { errorMessage } from "../errors";
import { Button } from "./Button";
import { Notice } from "./Notice";
import styles from "./States.module.css";

export function EmptyState({ children, action }: { children: ComponentChildren; action?: ComponentChildren }) {
  return (
    <div class={styles.empty}>
      <p>{children}</p>
      {action && <div class={styles.emptyAction}>{action}</div>}
    </div>
  );
}

export function Loading() {
  const { t } = useI18n();
  return <div class={styles.loading} role="status" aria-live="polite"><span class={styles.spinner} aria-hidden="true" />{t("common.loading")}</div>;
}

export function ErrorState({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const { t } = useI18n();
  return (
    <Notice kind="error" spaced action={onRetry && <Button size="small" onClick={onRetry}>{t("common.retry")}</Button>}>
      {errorMessage(error, t)}
    </Notice>
  );
}

/** A page that could not load: a way back plus the error. */
export function PageError({ error, onRetry, backHref = "#/" }: { error: unknown; onRetry?: () => void; backHref?: string }) {
  const { t } = useI18n();
  return (
    <div>
      <p><a href={backHref}>{"←"} {t("nav.back")}</a></p>
      <ErrorState error={error} {...(onRetry ? { onRetry } : {})} />
    </div>
  );
}
