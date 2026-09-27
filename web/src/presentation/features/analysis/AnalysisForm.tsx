import { useState } from "preact/hooks";
import type { StartAnalysisInput } from "../../../application/ports";
import { OUTPUT_LOCALE_LABELS, REASONING_PROFILE_LABELS } from "../../../domain/codes";
import { Button } from "../../components/Button";
import { Disclosure } from "../../components/Disclosure";
import { Field, formStyles } from "../../components/Field";
import { useI18n } from "../../i18n/I18nProvider";

interface Props {
  readonly canStart: boolean;
  readonly busy: boolean;
  readonly onStart: (input: StartAnalysisInput) => void;
}

export function AnalysisForm({ canStart, busy, onStart }: Props) {
  const { t, label } = useI18n();
  const [question, setQuestion] = useState("");
  const [profile, setProfile] = useState("GENERAL_RESEARCH");
  const [outputLocale, setOutputLocale] = useState("");
  return (
    <form class={formStyles.form} onSubmit={(e) => { e.preventDefault(); onStart({ researchQuestion: question, reasoningProfile: profile, outputLocale }); }}>
      <p class={formStyles.hint}>{t("analysis.defaultSettings")}</p>
      <Disclosure summary={t("analysis.advancedSettings")}>
      <Field label={t("project.questionLabel")} htmlFor="research-question" requirement="optional" requirementLabel={t("common.optional")} hint={t("analysis.questionHint")}>
        <input id="research-question" class={formStyles.control} type="text" maxLength={2000} value={question}
          placeholder={t("project.questionPlaceholder")} onInput={(e) => setQuestion(e.currentTarget.value)} />
      </Field>
      <div class={formStyles.row}>
        <Field label={t("project.profileLabel")} htmlFor="reasoning-profile" hint={t("analysis.profileHint")}>
          <select id="reasoning-profile" class={formStyles.control} value={profile} onChange={(e) => setProfile(e.currentTarget.value)}>
            {Object.keys(REASONING_PROFILE_LABELS).map((code) => <option key={code} value={code}>{label(REASONING_PROFILE_LABELS, code)}</option>)}
          </select>
        </Field>
        <Field label={t("analysis.outputLocale")} htmlFor="output-locale" hint={t("analysis.outputLocaleHint")}>
          <select id="output-locale" class={formStyles.control} value={outputLocale} onChange={(e) => setOutputLocale(e.currentTarget.value)}>
            {Object.keys(OUTPUT_LOCALE_LABELS).map((code) => <option key={code} value={code}>{label(OUTPUT_LOCALE_LABELS, code)}</option>)}
          </select>
        </Field>
      </div>
      </Disclosure>
      <div class={formStyles.actions}>
        <Button id="run-analysis" type="submit" variant="primary" disabled={!canStart || busy}>{t("project.runAnalysis")}</Button>
      </div>
    </form>
  );
}
