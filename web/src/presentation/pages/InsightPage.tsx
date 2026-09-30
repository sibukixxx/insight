import { spanKey, splitEvidence, splitPatterns } from "../../domain/evidence";
import type { MessageKey } from "../../domain/messages";
import type { InsightDetail } from "../../domain/models";
import { Card } from "../components/Card";
import { Meter } from "../components/Meter";
import { Notice } from "../components/Notice";
import { PageHeader } from "../components/PageHeader";
import { EmptyState, Loading, PageError } from "../components/States";
import { AbductionTrail } from "../features/evidence/AbductionTrail";
import { EvidenceMap } from "../features/evidence/EvidenceMap";
import { EvidenceRow } from "../features/evidence/EvidenceRow";
import evidenceStyles from "../features/evidence/Evidence.module.css";
import { PatternBlock } from "../features/evidence/PatternBlock";
import { QualityBadges } from "../features/evidence/QualityBadges";
import { useAsync } from "../hooks/useAsync";
import { useI18n } from "../i18n/I18nProvider";
import { projectHash } from "../router/routes";
import { useUseCases } from "../services/context";
import styles from "./InsightPage.module.css";

export function InsightPage({ insightId }: { insightId: string }) {
  const { t, percent } = useI18n();
  const { results } = useUseCases();
  const state = useAsync(() => results.inspectInsight(insightId), [results, insightId]);
  if (state.status === "loading") return <Loading />;
  if (state.status === "error") return <PageError error={state.error} onRetry={state.reload} />;
  const { insight, chain } = state.data;
  const back = projectHash(insight.projectId, "findings", insight.analysisId);
  return (
    <>
      <PageHeader back={{ href: back, label: t("nav.backToProject") }} eyebrow={t("insight.title")} title={insight.title} />
      <div class={styles.stack}>
        <Card title={t("evidenceMap.title")} description={t("evidenceMap.hint")}>
          <EvidenceMap chain={chain} hypothesis={insight.latentNeed} />
          <Meter label={t("insight.confidence")} percent={insight.confidence * 100} valueText={percent(insight.confidence)} />
          <QualityBadges flags={insight.qualityFlags} withDescriptions />
        </Card>
        <Trail insight={insight} />
        <Fields insight={insight} />
        <EvidenceSections insight={insight} />
      </div>
    </>
  );
}

function Trail({ insight }: { insight: InsightDetail }) {
  const { t } = useI18n();
  const { traces, repetitions } = splitPatterns(insight.patterns);
  // Only what the Evidence cards below actually list counts as already shown.
  const listed = splitEvidence(insight.evidence);
  const shared = new Set([...listed.support, ...listed.counter].map(spanKey));
  return (
    <section id="trail" tabIndex={-1} class={styles.trail} aria-labelledby="trail-title">
      <h2 id="trail-title" class={styles.sectionTitle}>{t("insight.trailTitle")}</h2>
      <p class={styles.hint}>{t("insight.trailHint")}</p>
      <AbductionTrail insight={insight} />
      <h3 class={styles.subtitle}>{t("insight.sourceObservations")}</h3>
      {insight.patterns.length === 0 ? <EmptyState>{t("pattern.noneForInsight")}</EmptyState> : (
        <>
          {traces.length ? traces.map((p) => <PatternBlock key={p.id} pattern={p} shared={shared} />) : <Notice kind="warning" spaced>{t("pattern.repetitionOnly")}</Notice>}
          {repetitions.map((p) => <PatternBlock key={p.id} pattern={p} shared={shared} />)}
        </>
      )}
    </section>
  );
}

/** Customer-research fields are optional: an empty one is hidden, never shown as a value. */
const present = (value: string): boolean => value.trim() !== "";

function Fields({ insight }: { insight: InsightDetail }) {
  const { t } = useI18n();
  // Legacy fields that the run never recorded are left out; the rest say so.
  const rows: readonly (readonly [MessageKey, string, string | undefined, boolean])[] = [
    ["insight.observation", insight.observation, styles.fact, true],
    ["insight.statedNeed", insight.statedNeed, undefined, present(insight.statedNeed)],
    ["insight.hypothesis", insight.latentNeed, styles.latent, true],
    ["insight.jtbd", insight.jtbd, undefined, present(insight.jtbd)],
    ["insight.interpretation", insight.interpretation, styles.interpretation, true],
    ["insight.alternative", insight.alternativeInterpretation, styles.alternative, true],
    ["insight.productOpportunity", insight.productOpportunity, undefined, present(insight.productOpportunity)],
    ["insight.monetizationAngle", insight.monetizationAngle, styles.money, present(insight.monetizationAngle)],
  ];
  return (
    <Card title={t("insight.details")}>
      <dl class={styles.fields}>
        {rows.filter(([, , , show]) => show).map(([key, value, cls]) => (
          <div key={key} class={[styles.field, cls].filter(Boolean).join(" ")}>
            <dt class={evidenceStyles.fieldLabel}>{t(key)}</dt>
            <dd>{present(value) ? value : t("common.notRecorded")}</dd>
          </div>
        ))}
      </dl>
    </Card>
  );
}

function EvidenceSections({ insight }: { insight: InsightDetail }) {
  const { t } = useI18n();
  const { support, counter } = splitEvidence(insight.evidence);
  return (
    <div id="evidence" tabIndex={-1} class={styles.evidenceGrid}>
      <Card title={t("insight.evidence")}>
        {support.length ? support.map((e) => <EvidenceRow key={e.id} span={e} />) : <EmptyState>{t("common.none")}</EmptyState>}
      </Card>
      <Card title={t("insight.counterEvidence")}>
        {counter.length ? counter.map((e) => <EvidenceRow key={e.id} span={e} />) : <EmptyState>{t("insight.noCounterEvidence")}</EmptyState>}
      </Card>
    </div>
  );
}
