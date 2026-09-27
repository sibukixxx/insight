import type { ComponentChildren } from "preact";
import styles from "./Disclosure.module.css";

/** Collapsed-by-default detail, used for the Advanced view of technical data. */
export function Disclosure({ summary, children, open = false }: { summary: string; children: ComponentChildren; open?: boolean }) {
  return (
    <details class={styles.details} open={open}>
      <summary class={styles.summary}>{summary}</summary>
      <div class={styles.body}>{children}</div>
    </details>
  );
}
