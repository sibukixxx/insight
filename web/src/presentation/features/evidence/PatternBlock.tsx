import { DEVIATION_LABELS } from "../../../domain/codes";
import type { Pattern } from "../../../domain/models";
import { Badge } from "../../components/Badge";
import { EmptyState } from "../../components/States";
import { useI18n } from "../../i18n/I18nProvider";
import { EvidenceRow } from "./EvidenceRow";
import styles from "./Evidence.module.css";

export function PatternBlock({ pattern }: { pattern: Pattern }) {
  const { t, label } = useI18n();
  const isTrace = pattern.kind === "deviation";
  return (
    <article class={[styles.pattern, isTrace && styles.trace].filter(Boolean).join(" ")}>
      <div class={styles.patternHead}>
        {isTrace ? <Badge tone="warning">{t("pattern.trace")}</Badge> : <Badge tone="accent">{t("pattern.repetition")}</Badge>}
        {isTrace && pattern.deviationType && <Badge tone="neutral">{label(DEVIATION_LABELS, pattern.deviationType)}</Badge>}
        <h3 class={styles.patternTitle}>{pattern.title}</h3>
      </div>
      {isTrace ? (
        <div class={styles.gap}>
          <div class={styles.cell}><div class={styles.fieldLabel}>{t("pattern.expected")}</div><div>{pattern.expectation || "-"}</div></div>
          <div class={styles.neq} aria-hidden="true">{"≠"}</div>
          <div class={styles.cell}><div class={styles.fieldLabel}>{t("pattern.observed")}</div><div>{pattern.description || "-"}</div></div>
        </div>
      ) : pattern.description ? <p class={styles.patternDesc}>{pattern.description}</p> : null}
      <div>
        {pattern.observations.length ? pattern.observations.map((o) => <EvidenceRow key={o.id} span={o} />) : <EmptyState>{t("pattern.noObservations")}</EmptyState>}
      </div>
    </article>
  );
}
