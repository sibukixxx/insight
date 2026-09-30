import { IMPORT_KIND_LABELS, SAMPLE_DATA_KIND_LABELS, SAMPLE_SOURCE_KIND_LABELS } from "../../../domain/codes";
import type { LlmSettings, Project, SampleScenario, SampleSource } from "../../../domain/models";
import type { SampleCard } from "../../../application/usecases/samples";
import { Button, ButtonLink } from "../../components/Button";
import { Disclosure } from "../../components/Disclosure";
import { Notice } from "../../components/Notice";
import { ErrorState, Loading } from "../../components/States";
import { useAsync } from "../../hooks/useAsync";
import { useI18n } from "../../i18n/I18nProvider";
import { errorMessage } from "../../errors";
import { projectHash } from "../../router/routes";
import { navigate } from "../../router/useRoute";
import { useBuild, useUseCases } from "../../services/context";
import { sampleText } from "./sampleCopy";
import styles from "./Samples.module.css";

/** Synthetic / official / mixed, never coloured like an epistemic status. */
export function DataKindChip({ kind }: { kind: string }) {
  const { label } = useI18n();
  const known = kind === "synthetic" || kind === "official" || kind === "mixed";
  return <span class={`${styles.kind} ${known ? styles[kind] : ""}`} data-kind={kind}>{label(SAMPLE_DATA_KIND_LABELS, kind)}</span>;
}

/**
 * What the samples can do with the current model setting. Documents CSV
 * analysis needs a model; only the analysis CSV runs deterministically.
 */
export function ModelNotice({ settings }: { settings: LlmSettings | undefined | "unknown" }) {
  const { t } = useI18n();
  if (settings === undefined) return null;
  if (settings === "unknown") return <Notice kind="warning">{t("samples.ai.unknown")}</Notice>;
  if (settings.configured) return <Notice kind="success">{t("samples.ai.configured", { model: settings.model })}</Notice>;
  return (
    <Notice kind="warning">
      <p>{t("samples.ai.missing")}</p>
      <p><a href="#/settings">{t("samples.ai.openSettings")}</a></p>
    </Notice>
  );
}

function useModelSettings(): LlmSettings | undefined | "unknown" {
  const { settings } = useUseCases();
  const state = useAsync(() => settings.loadSettings(), [settings]);
  if (state.status === "loading") return undefined;
  return state.status === "ok" ? state.data : "unknown";
}

function sourceSummary(t: ReturnType<typeof useI18n>["t"], sources: readonly SampleSource[]): string {
  const names = sources.map((s) => (s.kind === "official"
    ? [s.publisher, s.survey && `「${s.survey}」`].filter(Boolean).join(" ")
    : t("samples.syntheticSource")));
  return [...new Set(names)].join(" / ");
}

/** Home: 「何を調べてみますか？」 — one card per bundled scenario. */
/** `compact`: a returning user has seen the introduction, so only the cards are shown. */
export function SampleGallery({ projects, onError, compact = false }: { projects: readonly Project[]; onError: (message: string) => void; compact?: boolean }) {
  const i18n = useI18n();
  const { t, label, number } = i18n;
  const build = useBuild();
  const { samples, exports } = useUseCases();
  const state = useAsync(() => samples.loadGallery(build, projects), [samples, build, projects]);
  const model = useModelSettings();

  const open = (card: SampleCard) => {
    samples.open(card.scenario.id).then(
      (p) => navigate({ name: "input", projectId: p.id }),
      (e: unknown) => onError(errorMessage(e, t)),
    );
  };

  if (state.status === "loading") return <Loading />;
  if (state.status === "error") return <ErrorState error={state.error} onRetry={state.reload} />;
  if (state.data.length === 0) return null;
  return (
    <section id="samples" class={styles.gallery} aria-labelledby="samples-title" tabIndex={-1}>
      <div class={styles.galleryHead}>
        <h2 id="samples-title" class={styles.galleryTitle}>{t("samples.title")}</h2>
        {!compact && <p class={styles.lead}>{t("samples.lead")}</p>}
      </div>
      <ModelNotice settings={model} />
      <ol class={styles.cards}>
        {state.data.map((card, i) => {
          const s = card.scenario;
          const text = sampleText(i18n, s);
          return (
            <li key={s.id} class={styles.card} data-scenario={s.id}>
              <div class={styles.cardTop}>
                <span class={styles.num} aria-hidden="true">{String(i + 1).padStart(2, "0")}</span>
                <DataKindChip kind={s.dataKind} />
              </div>
              <h3 id={`sample-${s.id}-title`} class={styles.cardTitle}>{text.title}</h3>
              {text.question && <p class={styles.question}><span class={styles.questionLabel}>{t("samples.questionLabel")}</span>{text.question}</p>}
              {/* The data kind and the caution stay visible; the rest is one deliberate step away. */}
              {text.caution && <dl class={styles.facts}><div><dt>{t("samples.cautionLabel")}</dt><dd>{text.caution}</dd></div></dl>}
              <Disclosure summary={t("samples.details")}>
                <dl class={styles.facts}>
                  <div><dt>{t("samples.inputLabel")}</dt><dd>{t("samples.inputValue", { format: label(IMPORT_KIND_LABELS, s.importKind), rows: number(s.rows) })}</dd></div>
                  {text.learn && <div><dt>{t("samples.learnLabel")}</dt><dd>{text.learn}</dd></div>}
                  <div><dt>{t("samples.sourceLabel")}</dt><dd>{sourceSummary(t, s.sources)}</dd></div>
                </dl>
              </Disclosure>
              <div class={styles.actions}>
                <Button variant="primary" onClick={() => open(card)} aria-describedby={`sample-${s.id}-title`}>{t("samples.try")}</Button>
                <ButtonLink size="small" variant="ghost" href={exports.sampleInputLink(s.id)} download>{t("samples.download")}</ButtonLink>
              </div>
              {card.latestCompleted
                ? <a class={styles.resultLink} href={projectHash(s.projectId, "findings", card.latestCompleted.id)}>{t("samples.viewResult")} {"→"}</a>
                : card.projectExists && <a class={styles.resultLink} href={projectHash(s.projectId)}>{t("samples.continue")} {"→"}</a>}
            </li>
          );
        })}
      </ol>
      <p class={styles.footnote}>{t("samples.previewNote")}</p>
    </section>
  );
}

/** Provenance: publisher, dataset, region, period, unit, retrieval, licence and checksums. */
export function SampleSourceDetails({ scenario }: { scenario: SampleScenario }) {
  const { t, label, dateTime } = useI18n();
  return (
    <Disclosure summary={t("samples.provenance.title")}>
      <div class={styles.provenance}>
        {scenario.sources.map((s, i) => (
          <section key={i} class={styles.source} data-source-kind={s.kind}>
            <h4 class={styles.sourceTitle}>{label(SAMPLE_SOURCE_KIND_LABELS, s.kind)}</h4>
            {s.description && <p class={styles.meta}>{s.description}</p>}
            <dl class={styles.sourceFacts}>
              {s.publisher && <><dt>{t("samples.provenance.publisher")}</dt><dd>{[s.publisher, s.survey].filter(Boolean).join(" / ")}</dd></>}
              {s.dataset && <><dt>{t("samples.provenance.dataset")}</dt><dd>{s.dataset}{s.indicatorCode && <> <code>{s.indicatorCode}</code></>}</dd></>}
              {s.provider && <><dt>{t("samples.provenance.provider")}</dt><dd>{s.provider}</dd></>}
              {s.regions.length > 0 && <><dt>{t("samples.provenance.regions")}</dt><dd>{s.regions.map((r) => `${r.name} (${r.code})`).join("、")}</dd></>}
              {s.periods.length > 0 && <><dt>{t("samples.provenance.periods")}</dt><dd>{s.periods.join("、")}</dd></>}
              {s.unit && <><dt>{t("samples.provenance.unit")}</dt><dd>{s.unit}</dd></>}
              {s.retrievedAt && <><dt>{t("samples.provenance.retrievedAt")}</dt><dd>{dateTime(s.retrievedAt)}</dd></>}
              {s.license && <><dt>{t("samples.provenance.license")}</dt><dd>{s.licenseUrl ? <a href={s.licenseUrl} target="_blank" rel="noopener noreferrer">{s.license}</a> : s.license}</dd></>}
              {s.attribution && <><dt>{t("samples.provenance.attribution")}</dt><dd>{s.attribution}</dd></>}
              {s.url && <><dt>{t("samples.provenance.url")}</dt><dd><a href={s.url} target="_blank" rel="noopener noreferrer">{s.url}</a></dd></>}
              {s.rawFile && <><dt>{t("samples.provenance.raw")}</dt><dd><code>{s.rawFile}</code>{s.rawSha256 && <>{" · sha256 "}<code>{s.rawSha256}</code></>}</dd></>}
              {s.rows.length > 0 && <><dt>{t("samples.provenance.rows")}</dt><dd><code>{s.rows.join(", ")}</code></dd></>}
            </dl>
          </section>
        ))}
        <dl class={styles.sourceFacts}>
          <dt>{t("samples.provenance.input")}</dt><dd>{"sha256 "}<code>{scenario.inputSha256}</code></dd>
          <dt>{t("samples.provenance.transform")}</dt>
          <dd>{scenario.transform.description}{scenario.transform.script && <>{" ("}<code>{scenario.transform.script}</code>{` v${scenario.transform.version})`}</>}</dd>
        </dl>
        {scenario.limitations.length > 0 && (
          <>
            <h4 class={styles.sourceTitle}>{t("samples.provenance.limitations")}</h4>
            <ul class={styles.limitations}>{scenario.limitations.map((l) => <li key={l}>{l}</li>)}</ul>
          </>
        )}
      </div>
    </Disclosure>
  );
}

/** Input page of a sample project: the question, the three steps, model capability and provenance. */
export function SampleScenarioPanel({ scenario, hasDocuments }: { scenario: SampleScenario; hasDocuments: boolean }) {
  const i18n = useI18n();
  const { t } = i18n;
  const text = sampleText(i18n, scenario);
  const model = useModelSettings();
  const steps = ["samples.flow.import", "samples.flow.analyze", "samples.flow.review"] as const;
  return (
    <section class={styles.panel} aria-labelledby="sample-panel-title" data-scenario={scenario.id}>
      <div class={styles.cardTop}>
        <span class={styles.panelEyebrow}>{t("samples.panel.eyebrow")}</span>
        <DataKindChip kind={scenario.dataKind} />
      </div>
      <h2 id="sample-panel-title" class={styles.cardTitle}>{text.title}</h2>
      {text.question && <p class={styles.question}><span class={styles.questionLabel}>{t("samples.questionLabel")}</span>{text.question}</p>}
      <ol class={styles.flow} aria-label={t("samples.flow.label")}>
        {steps.map((key, i) => (
          <li key={key} class={styles.flowStep} aria-current={(hasDocuments ? i === 1 : i === 0) ? "step" : undefined}>
            <span class={styles.flowNum} aria-hidden="true">{i + 1}</span>{t(key)}
          </li>
        ))}
      </ol>
      <ModelNotice settings={model} />
      <SampleSourceDetails scenario={scenario} />
    </section>
  );
}

/** A one-line reminder on every screen of a sample project, so results are never read without their data kind. */
export function SampleStrip({ scenario }: { scenario: SampleScenario }) {
  const i18n = useI18n();
  return (
    <p class={styles.strip} data-scenario={scenario.id}>
      <span>{i18n.t("samples.strip.label")}</span>
      <DataKindChip kind={scenario.dataKind} />
      <span class={styles.stripTitle}>{sampleText(i18n, scenario).title}</span>
      <a href={projectHash(scenario.projectId, "input")}>{i18n.t("samples.strip.sources")}</a>
    </p>
  );
}
