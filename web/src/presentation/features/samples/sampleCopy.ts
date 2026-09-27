import { SAMPLE_COPY } from "../../../domain/codes";
import type { SampleScenario } from "../../../domain/models";
import type { I18n } from "../../i18n/I18nProvider";

/** Pre-analysis copy for a scenario; an unknown scenario falls back to its server name. */
export function sampleText(i18n: Pick<I18n, "t">, scenario: SampleScenario) {
  const copy = SAMPLE_COPY[scenario.id];
  if (!copy) return { title: scenario.projectName, question: undefined, learn: undefined, caution: scenario.limitations[0] };
  return { title: i18n.t(copy.title), question: i18n.t(copy.question), learn: i18n.t(copy.learn), caution: i18n.t(copy.caution) };
}
