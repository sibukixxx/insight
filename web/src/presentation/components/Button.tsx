import type { ComponentChildren, JSX } from "preact";
import styles from "./Button.module.css";

type Variant = "primary" | "secondary" | "ghost" | "danger";

interface Look {
  readonly variant?: Variant | undefined;
  readonly size?: "small" | "normal" | undefined;
  readonly block?: boolean | undefined;
}

interface Common extends Look {
  readonly children: ComponentChildren;
}

function classes({ variant = "secondary", size = "normal", block = false }: Look): string {
  return [styles.button, variant !== "secondary" && styles[variant], size === "small" && styles.small, block && styles.block].filter(Boolean).join(" ");
}

export function Button({ variant, size, block, children, type = "button", ...rest }: Common & Omit<JSX.ButtonHTMLAttributes<HTMLButtonElement>, "size">) {
  return <button type={type} class={classes({ variant, size, block })} {...rest}>{children}</button>;
}

export function ButtonLink({ variant, size, block, children, ...rest }: Common & JSX.AnchorHTMLAttributes<HTMLAnchorElement>) {
  return <a class={classes({ variant, size, block })} {...rest}>{children}</a>;
}
