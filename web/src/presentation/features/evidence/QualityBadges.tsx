import { QUALITY_FLAG_LABELS } from "../../../domain/codes";
import type { QualityFlag } from "../../../domain/models";
import { Badge } from "../../components/Badge";
import { useI18n, type I18n } from "../../i18n/I18nProvider";
import styles from "./Evidence.module.css";

function meta(code: string, t: I18n["t"]): { label: string; desc: string } {
  const keys = QUALITY_FLAG_LABELS[code];
  return keys ? { label: t(keys.label), desc: t(keys.desc) } : { label: code, desc: "" };
}

/** App-side quality warnings; hints for the reader, never verdicts. */
export function QualityBadges({ flags, withDescriptions = false }: { flags: readonly QualityFlag[]; withDescriptions?: boolean }) {
  const { t } = useI18n();
  if (flags.length === 0) return null;
  const badges = (
    <div class={styles.badges}>
      {flags.map((f) => {
        const m = meta(f.code, t);
        return <Badge key={f.code} tone="danger" title={m.desc}>{"⚠"} {f.detail ? t("quality.badgeWithDetail", { label: m.label, detail: f.detail }) : m.label}</Badge>;
      })}
    </div>
  );
  if (!withDescriptions) return badges;
  return (
    <div class={styles.qualityBox}>
      <div class={styles.qualityTitle}>{t("quality.boxTitle")}</div>
      {badges}
      <ul class={styles.qualityList}>
        {flags.map((f) => {
          const m = meta(f.code, t);
          return <li key={f.code}><strong>{f.detail ? t("quality.badgeWithDetail", { label: m.label, detail: f.detail }) : m.label}</strong> {"—"} {m.desc}</li>;
        })}
      </ul>
    </div>
  );
}
