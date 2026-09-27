import type { EvidenceChain } from "../../../domain/evidence";
import { useI18n } from "../../i18n/I18nProvider";
import styles from "./Evidence.module.css";

/**
 * Observation → pattern → hypothesis → evidence, as the server linked them.
 * Counts link to the sections below; nothing here is inferred.
 */
export function EvidenceMap({ chain, hypothesis }: { chain: EvidenceChain; hypothesis: string }) {
  const { t } = useI18n();
  return (
    <ol class={styles.map} aria-label={t("evidenceMap.label")}>
      <li class={styles.node}>
        <span class={styles.nodeCount}>{chain.observations}</span>
        <span class={styles.nodeLabel}>{t("evidenceMap.observations")}</span>
        <span class={styles.nodeDetail}>{t("evidenceMap.fromDocuments", { documents: chain.documents })}</span>
        <a class={styles.nodeLink} href="#trail" onClick={jump("trail")}>{t("evidenceMap.show")}</a>
      </li>
      <li class={styles.node}>
        <span class={styles.nodeCount}>{chain.traces + chain.repetitions}</span>
        <span class={styles.nodeLabel}>{t("evidenceMap.patterns")}</span>
        <span class={styles.nodeDetail}>{t("evidenceMap.patternKinds", { traces: chain.traces, repetitions: chain.repetitions })}</span>
      </li>
      <li class={`${styles.node} ${styles.nodeHyp}`}>
        <span class={styles.nodeLabel}>{t("evidenceMap.hypothesis")}</span>
        <span class={styles.nodeDetail}>{hypothesis || t("common.notRecordedTitle")}</span>
      </li>
      <li class={`${styles.node} ${chain.counter === 0 ? "" : styles.nodeCounter}`}>
        <span class={styles.nodeCount}>{chain.support}</span>
        <span class={styles.nodeLabel}>{t("evidenceMap.evidence")}</span>
        <span class={styles.nodeDetail}>{t("evidenceMap.counter", { counter: chain.counter })}</span>
        <a class={styles.nodeLink} href="#evidence" onClick={jump("evidence")}>{t("evidenceMap.show")}</a>
      </li>
    </ol>
  );
}

// In-page jumps must not touch the hash, which holds the route.
function jump(id: string) {
  return (e: Event) => {
    e.preventDefault();
    const target = document.getElementById(id);
    target?.scrollIntoView({ behavior: "smooth", block: "start" });
    target?.focus({ preventScroll: true });
  };
}
