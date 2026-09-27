import type { Insight } from "../../../domain/models";
import { Meter } from "../../components/Meter";
import { useI18n } from "../../i18n/I18nProvider";
import { buildHash } from "../../router/routes";
import { QualityBadges } from "../evidence/QualityBadges";
import styles from "./Findings.module.css";

export function InsightCard({ insight }: { insight: Insight }) {
  const { t, percent } = useI18n();
  return (
    <a class={[styles.card, insight.qualityFlags.length > 0 && styles.flagged].filter(Boolean).join(" ")} href={buildHash({ name: "insight", insightId: insight.id })}>
      <div class={styles.title}>{insight.title}</div>
      <div class={styles.hypothesis}>{insight.hypothesis || insight.latentNeed}</div>
      {insight.surprisingFact && <div class={styles.trace}>{t("project.deviation", { fact: insight.surprisingFact })}</div>}
      <Meter label={t("insight.confidence")} percent={insight.confidence * 100} valueText={percent(insight.confidence)} />
      <QualityBadges flags={insight.qualityFlags} />
    </a>
  );
}

export function InsightList({ insights }: { insights: readonly Insight[] }) {
  return (
    <ul class={styles.list}>
      {insights.map((i) => <li key={i.id}><InsightCard insight={i} /></li>)}
    </ul>
  );
}
