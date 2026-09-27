import type { ComponentChildren } from "preact";
import styles from "./Notice.module.css";

interface Props {
  readonly kind?: "info" | "success" | "warning" | "error";
  readonly action?: ComponentChildren;
  readonly spaced?: boolean;
  readonly children: ComponentChildren;
}

/** Errors are announced assertively; other notices politely. */
export function Notice({ kind = "info", action, spaced = false, children }: Props) {
  return (
    <div class={[styles.notice, styles[kind], spaced && styles.spaced].filter(Boolean).join(" ")} role={kind === "error" ? "alert" : "status"}>
      <div class={styles.body}>{children}</div>
      {action && <div class={styles.action}>{action}</div>}
    </div>
  );
}
