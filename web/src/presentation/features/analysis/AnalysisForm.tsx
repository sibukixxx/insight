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
}

export function AnalysisForm({ readiness, busy, onStart, emphasized = true, initialOutputLocale = "" }: Props) {
  const { t, label } = useI18n();
  const [question, setQuestion] = useState("");
  const [profile, setProfile] = useState("GENERAL_RESEARCH");
  const [outputLocale, setOutputLocale] = useState(initialOutputLocale in OUTPUT_LOCALE_LABELS ? initialOutputLocale : "");
  return (
    <form class={formStyles.form} onSubmit={(e) => { e.preventDefault(); if (readiness.canStart && !busy) onStart({ researchQuestion: question, reasoningProfile: profile, outputLocale }); }}>
      <p class={formStyles.hint}>{t("analysis.defaultSettings", { question: question.trim() || t("analysis.openQuestion"), profile: label(REASONING_PROFILE_LABELS, profile), locale: label(OUTPUT_LOCALE_LABELS, outputLocale) })}</p>
      <Field label={t("analysis.outputLocale")} htmlFor="output-locale" hint={t("analysis.outputLocaleHint")} requirement="optional">
        {(control) => (<select {...control} class={formStyles.control} value={outputLocale} onChange={(e) => setOutputLocale(e.currentTarget.value)}>
          {Object.keys(OUTPUT_LOCALE_LABELS).map((code) => <option key={code} value={code}>{label(OUTPUT_LOCALE_LABELS, code)}</option>)}
        </select>)}
      </Field>
      <Disclosure summary={t("analysis.advancedSettings")}>
      <Field label={t("project.questionLabel")} htmlFor="research-question" requirement="optional" requirementLabel={t("common.optional")} hint={t("analysis.questionHint")}>
            {(control) => (<input {...control} class={formStyles.control} type="text" value={question}
          placeholder={t("project.questionPlaceholder")} onInput={(e) => setQuestion(e.currentTarget.value)} />)}
          </Field>
      <div class={formStyles.row}>
        <Field label={t("project.profileLabel")} htmlFor="reasoning-profile" hint={t("analysis.profileHint")} requirement="optional">
            {(control) => (<select {...control} class={formStyles.control} value={profile} onChange={(e) => setProfile(e.currentTarget.value)}>
            {Object.keys(REASONING_PROFILE_LABELS).map((code) => <option key={code} value={code}>{label(REASONING_PROFILE_LABELS, code)}</option>)}
          </select>)}
          </Field>
      </div>
      </Disclosure>
      <div id="analysis-start-help" class={formStyles.hint} aria-live="polite">
        {readiness.canStart ? (readiness.status === "unknown" ? t("readiness.settingsUnknown") : t("readiness.ready")) : (
          <><p>{t("analysis.startBlocked")}</p><ul>{readiness.checks.filter((c) => c.level === "blocked").map((c) => <li key={c.id}>{t(c.message, c.params)}</li>)}</ul></>
        )}
      </div>
      <div class={formStyles.actions}>
        <Button id="run-analysis" type="submit" variant={emphasized ? "primary" : "secondary"} aria-describedby="analysis-start-help" disabled={!readiness.canStart || busy}>{t("project.runAnalysis")}</Button>
      </div>
    </form>
  );
}
