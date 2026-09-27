import styles from "./Meter.module.css";

interface Props {
  readonly label: string;
  /** 0–100 */
  readonly percent: number;
  readonly valueText: string;
  readonly hideLabel?: boolean;
}

/** A labelled progress/confidence bar; the width is a CSS custom property. */
export function Meter({ label, percent, valueText, hideLabel = false }: Props) {
  const clamped = Math.max(0, Math.min(100, Math.round(percent)));
  return (
    <div class={styles.row}>
      <span class={hideLabel ? "visually-hidden" : styles.label}>{label}</span>
      <div class={styles.track} role="progressbar" aria-label={label} aria-valuemin={0} aria-valuemax={100} aria-valuenow={clamped} aria-valuetext={valueText}>
        <div class={styles.fill} style={{ "--meter-value": `${clamped}%` }} />
      </div>
      <span class={styles.value} aria-hidden="true">{valueText}</span>
    </div>
  );
}
