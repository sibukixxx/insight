import { useState } from "preact/hooks";
import type { Readiness } from "../../../domain/readiness";
import type { StartAnalysisInput } from "../../../application/ports";
import { OUTPUT_LOCALE_LABELS, REASONING_PROFILE_LABELS } from "../../../domain/codes";
import { Button } from "../../components/Button";
import { Disclosure } from "../../components/Disclosure";
import { Field, formStyles } from "../../components/Field";
import { useI18n } from "../../i18n/I18nProvider";

interface Props {
  readonly readiness: Readiness;
  readonly busy: boolean;
  readonly onStart: (input: StartAnalysisInput) => void;
  /** False once a completed run's "View results" is the screen's main action. */
  readonly emphasized?: boolean;
  /** Preselected output language, e.g. the latest run's; "" = automatic. */
  readonly initialOutputLocale?: string | undefined;
  /** The theme the project was created from; the question field starts with it. */
  readonly initialQuestion?: string;
}

export function AnalysisForm({ readiness, busy, onStart, emphasized = true, initialOutputLocale = "", initialQuestion = "" }: Props) {
  const { t, label } = useI18n();
  const [question, setQuestion] = useState(initialQuestion);
  const [profile, setProfile] = useState("GENERAL_RESEARCH");
  const [outputLocale, setOutputLocale] = useState(initialOutputLocale in OUTPUT_LOCALE_LABELS ? initialOutputLocale : "");
    const explore = readiness.noEvidence;
  const trimmed = question.trim();
  const canRun = explore ? readiness.canExplore && trimmed !== "" : readiness.canStart;
  const submit = (e: Event) => {
    e.preventDefault();
    if (!canRun || busy) return;
    onStart({ researchQuestion: question, reasoningProfile: profile, outputLocale, ...(explore ? { exploratory: true } : {}) });
  };
  return (
    <form class={formStyles.form} onSubmit={submit}>
      <Field label={t("home.question.label")} htmlFor="research-question" requirement={explore ? "required" : "optional"} requirementLabel={explore ? t("common.required") : t("common.optional")} hint={explore ? t("analysis.exploreQuestionHint") : t("analysis.questionHint")}>
        {(control) => (<textarea {...control} class={formStyles.control} rows={3} value={question}
          placeholder={t("home.question.placeholder")} onInput={(e) => setQuestion(e.currentTarget.value)} />)}
      </Field>
      <p class={formStyles.hint}>{t("analysis.defaultSettings", { question: trimmed || t("analysis.openQuestion"), profile: label(REASONING_PROFILE_LABELS, profile), locale: label(OUTPUT_LOCALE_LABELS, outputLocale) })}</p>
      <Field label={t("analysis.outputLocale")} htmlFor="output-locale" hint={t("analysis.outputLocaleHint")} requirement="optional">
        {(control) => (<select {...control} class={formStyles.control} value={outputLocale} onChange={(e) => setOutputLocale(e.currentTarget.value)}>
          {Object.keys(OUTPUT_LOCALE_LABELS).map((code) => <option key={code} value={code}>{label(OUTPUT_LOCALE_LABELS, code)}</option>)}
        </select>)}
      </Field>
      <Disclosure summary={t("analysis.advancedSettings")}>
      <div class={formStyles.row}>
        <Field label={t("project.profileLabel")} htmlFor="reasoning-profile" hint={t("analysis.profileHint")} requirement="optional">
            {(control) => (<select {...control} class={formStyles.control} value={profile} onChange={(e) => setProfile(e.currentTarget.value)}>
            {Object.keys(REASONING_PROFILE_LABELS).map((code) => <option key={code} value={code}>{label(REASONING_PROFILE_LABELS, code)}</option>)}
          </select>)}
          </Field>
      </div>
      </Disclosure>
      <div id="analysis-start-help" class={formStyles.hint} aria-live="polite">
        {explore ? (
          <p>{!readiness.canExplore ? t("analysis.exploreBlocked") : trimmed === "" ? t("analysis.exploreNeedsQuestion") : t("analysis.exploreReady")}</p>
        ) : readiness.canStart ? (readiness.status === "unknown" ? t("readiness.settingsUnknown") : t("readiness.ready")) : (
          <><p>{t("analysis.startBlocked")}</p><ul>{readiness.checks.filter((c) => c.level === "blocked").map((c) => <li key={c.id}>{t(c.message, c.params)}</li>)}</ul></>
        )}
      </div>
      <div class={formStyles.actions}>
        {explore
          ? <Button id="explore-question" type="submit" variant="primary" aria-describedby="analysis-start-help" disabled={!canRun || busy}>{t("analysis.exploreRun")}</Button>
          : <Button id="run-analysis" type="submit" variant={emphasized ? "primary" : "secondary"} aria-describedby="analysis-start-help" disabled={!readiness.canStart || busy}>{t("project.runAnalysis")}</Button>}
      </div>
    </form>
  );
}
