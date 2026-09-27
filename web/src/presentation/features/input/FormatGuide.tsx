import { IMPORT_KIND_LABELS, labelKey } from "../../../domain/codes";
import type { MessageKey } from "../../../domain/messages";
import type { ImportFormat } from "../../../domain/models";
import { ButtonLink } from "../../components/Button";
import { Notice } from "../../components/Notice";
import { useI18n } from "../../i18n/I18nProvider";
import { useUseCases } from "../../services/context";
import styles from "./Input.module.css";

const HINTS: Readonly<Record<string, MessageKey>> = {
  documents: "project.importCsvHint",
  analysis: "project.importAnalysisCsvHint",
};

/** The formats the server accepts, as reported by the server, with header-only templates. */
export function FormatGuide({ formats }: { formats: readonly ImportFormat[] }) {
  const { t, label } = useI18n();
  const { exports } = useUseCases();
  return (
    <>
      <ul class={styles.formats}>
        {formats.map((f) => {
          const hint = HINTS[f.kind];
          return (
            <li key={f.kind} class={styles.format} data-format={f.kind}>
              <span class={styles.formatTitle}>{labelKey(IMPORT_KIND_LABELS, f.kind) ? label(IMPORT_KIND_LABELS, f.kind) : f.kind}</span>
              {hint && <span class={styles.formatHint}>{t(hint)}</span>}
              <span class={styles.meta}>{t("input.format.fileType", { extensions: f.extensions.join(", "), encoding: f.encoding })}</span>
              <span class={styles.meta}>{t("input.format.columns")}</span>
              <ul class={styles.columns} aria-label={t("input.format.columns")}>
                {f.columns.map((c) => <li key={c.name}><code>{c.name}</code>{c.required ? ` ${t("common.required")}` : ` ${t("input.format.optional")}`}</li>)}
              </ul>
              <ButtonLink size="small" href={exports.templateLink(f.kind)} download>{"↓"} {t("input.format.template")}</ButtonLink>
            </li>
          );
        })}
      </ul>
      <p class={styles.meta}>{t("input.format.text.title")}{": "}{t("input.format.text.hint")}</p>
      <div class={styles.unsupported}>
        <Notice kind="info">{t("input.format.unsupported")}</Notice>
      </div>
    </>
  );
}
