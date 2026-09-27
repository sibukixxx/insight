import type { ComponentChildren } from "preact";
import { useId } from "preact/hooks";
import styles from "./Card.module.css";

interface Props {
  readonly title?: string;
  readonly description?: ComponentChildren;
  readonly actions?: ComponentChildren;
  readonly headingLevel?: 2 | 3;
  readonly class?: string;
  readonly children: ComponentChildren;
}

/** A titled section; the title labels the region for assistive technology. */
export function Card({ title, description, actions, headingLevel = 2, class: extra, children }: Props) {
  const id = useId();
  const Heading = headingLevel === 2 ? "h2" : "h3";
  return (
    <section class={[styles.card, extra].filter(Boolean).join(" ")} aria-labelledby={title ? id : undefined}>
      {(title || actions) && (
        <div class={styles.header}>
          {title && <Heading id={id} class={styles.title}>{title}</Heading>}
          {actions && <div class={styles.actions}>{actions}</div>}
        </div>
      )}
      {description && <p class={styles.description}>{description}</p>}
      {children}
    </section>
  );
}
