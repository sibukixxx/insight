import { SOURCE_LABELS } from "../../../domain/codes";
import { Badge, type Tone } from "../../components/Badge";
import { useI18n } from "../../i18n/I18nProvider";

const TONES: Readonly<Record<string, Tone>> = { dataset: "success", interview: "accent", review: "success", support: "warning", sales: "violet", survey: "danger" };

export function SourceTag({ source }: { source: string }) {
  const { label } = useI18n();
  return <Badge tone={TONES[source] ?? "neutral"}>{label(SOURCE_LABELS, source)}</Badge>;
}
