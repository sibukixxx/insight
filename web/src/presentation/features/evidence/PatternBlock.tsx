import { DEVIATION_LABELS } from "../../../domain/codes";
import { spanKey } from "../../../domain/evidence";
import type { Pattern } from "../../../domain/models";
import { Badge } from "../../components/Badge";
import { Disclosure } from "../../components/Disclosure";
import { EmptyState } from "../../components/States";
import { useI18n } from "../../i18n/I18nProvider";
import { EvidenceRow } from "./EvidenceRow";
import styles from "./Evidence.module.css";

/**
 * `shared` holds the spans the page already lists under support / counter
 * evidence. Those quotes stay reachable here, folded, instead of repeating.
 */
export function PatternBlock({ pattern, shared }: { pattern: Pattern; shared?: ReadonlySet<string> | undefined }) {
  const { t, label } = useI18n();
  const repeated = pattern.observations.filter((o) => shared?.has(spanKey(o)));
  const own = pattern.observations.filter((o) => !shared?.has(spanKey(o)));
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
          <div class={styles.cell}><div class={styles.fieldLabel}>{t("pattern.expected")}</div><div>{pattern.expectation || t("common.notRecorded")}</div></div>
          <div class={styles.neq} aria-hidden="true">{"≠"}</div>
          <div class={styles.cell}><div class={styles.fieldLabel}>{t("pattern.observed")}</div><div>{pattern.description || t("common.notRecorded")}</div></div>
        </div>
      ) : pattern.description ? <p class={styles.patternDesc}>{pattern.description}</p> : null}
      <div>
        {pattern.observations.length === 0 && <EmptyState>{t("pattern.noObservations")}</EmptyState>}
        {own.map((o) => <EvidenceRow key={o.id} span={o} />)}
        {repeated.length > 0 && (
          <Disclosure summary={t("pattern.sharedQuotes", { count: repeated.length })}>
            {repeated.map((o) => <EvidenceRow key={o.id} span={o} />)}
          </Disclosure>
        )}
      </div>
    </article>
  );
}
