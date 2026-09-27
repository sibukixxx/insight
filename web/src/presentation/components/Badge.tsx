import type { ComponentChildren } from "preact";
import styles from "./Badge.module.css";

export type Tone = "neutral" | "accent" | "success" | "warning" | "danger" | "violet" | "missing";

export function Badge({ tone = "neutral", title, children }: { tone?: Tone; title?: string; children: ComponentChildren }) {
  return <span class={`${styles.badge} ${styles[tone]}`} title={title}>{children}</span>;
}

export const badgeStyles = styles;
