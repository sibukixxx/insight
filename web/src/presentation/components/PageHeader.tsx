import type { ComponentChildren } from "preact";
import styles from "./PageHeader.module.css";

interface Props {
  readonly title: string;
  readonly eyebrow?: string;
  readonly subtitle?: ComponentChildren;
  readonly back?: { readonly href: string; readonly label: string };
  readonly actions?: ComponentChildren;
}

export function PageHeader({ title, eyebrow, subtitle, back, actions }: Props) {
  return (
    <div class={styles.header}>
      {back && <a class={styles.back} href={back.href}>{"←"} {back.label}</a>}
      <div class={styles.row}>
        <div>
          {eyebrow && <div class={styles.eyebrow}>{eyebrow}</div>}
          <h1 class={styles.title}>{title}</h1>
          {subtitle && <p class={styles.subtitle}>{subtitle}</p>}
        </div>
        {actions && <div class={styles.actions}>{actions}</div>}
      </div>
    </div>
  );
}
