import { useId, useState } from "preact/hooks";
import { matchesFormat } from "../../../application/usecases/importEvidence";
import type { UploadFile } from "../../../application/ports";
import { IMPORT_KIND_LABELS, labelKey } from "../../../domain/codes";
import type { DocumentImportResult, ImportFormat, ImportPreview } from "../../../domain/models";
import { Button } from "../../components/Button";
import { Disclosure } from "../../components/Disclosure";
import { Notice } from "../../components/Notice";
import { useI18n } from "../../i18n/I18nProvider";
import { errorMessage } from "../../errors";
import { useUseCases } from "../../services/context";
import { DocumentList } from "./DocumentList";
import styles from "./Input.module.css";

type Step =
  | { readonly state: "idle" }
  | { readonly state: "previewing"; readonly file: UploadFile }
  | { readonly state: "preview"; readonly file: UploadFile; readonly preview: ImportPreview }
  | { readonly state: "importing"; readonly file: UploadFile; readonly preview: ImportPreview }
  | { readonly state: "done"; readonly result: DocumentImportResult }
  | { readonly state: "error"; readonly message: string; readonly file?: UploadFile };

const MAX_LISTED_ERRORS = 10;

/**
 * Upload → server-side dry run (the importer's own validation and the Data
 * Triage profile) → explicit import. Nothing is stored before "Import".
 */
export function CsvImport({ projectId, formats, onImported }: { projectId: string; formats: readonly ImportFormat[]; onImported: () => void }) {
  const i18n = useI18n();
  const { t } = i18n;
  const { input } = useUseCases();
  const fileInputId = useId();
  const [kind, setKind] = useState(formats[0]?.kind ?? "documents");
  const [step, setStep] = useState<Step>({ state: "idle" });
  const [dragging, setDragging] = useState(false);
  const format = formats.find((f) => f.kind === kind);

  const choose = (file: File | undefined) => {
    if (!file || !format) return;
    const upload: UploadFile = { name: file.name, blob: file };
    if (!matchesFormat(file.name, format)) {
      setStep({ state: "error", message: t("input.csv.wrongType", { name: file.name, extensions: format.extensions.join(", ") }) });
      return;
    }
    setStep({ state: "previewing", file: upload });
    input.previewImport(projectId, kind, upload).then(
      (preview) => setStep({ state: "preview", file: upload, preview }),
      (err: unknown) => setStep({ state: "error", message: errorMessage(err, t), file: upload }),
    );
  };

  const confirm = () => {
    if (step.state !== "preview") return;
    setStep({ state: "importing", file: step.file, preview: step.preview });
    input.importFile(projectId, kind, step.file).then(
      (result) => { setStep({ state: "done", result }); onImported(); },
      (err: unknown) => setStep({ state: "error", message: errorMessage(err, t), file: step.file }),
    );
  };

  const kindLabel = (k: string) => (labelKey(IMPORT_KIND_LABELS, k) ? i18n.label(IMPORT_KIND_LABELS, k) : k);
  const busy = step.state === "previewing" || step.state === "importing";

  return (
    <div class={styles.chooser}>
      <fieldset class={styles.kinds}>
        <legend>{t("input.csv.kind")}</legend>
        {formats.map((f) => (
          <label key={f.kind} class={styles.kind}>
            <input type="radio" name="import-kind" value={f.kind} checked={kind === f.kind} disabled={busy}
              onChange={() => { setKind(f.kind); setStep({ state: "idle" }); }} />
            {kindLabel(f.kind)}
          </label>
        ))}
      </fieldset>

      <div
        class={[styles.drop, dragging && styles.dragging].filter(Boolean).join(" ")}
        onDragOver={(e) => { e.preventDefault(); setDragging(true); }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => { e.preventDefault(); setDragging(false); if (!busy) choose(e.dataTransfer?.files[0]); }}
      >
        <label for={fileInputId}>{t("input.csv.dropHint", { extensions: format?.extensions.join(", ") ?? "" })}</label>
        <input id={fileInputId} type="file" name="file" accept={format ? [...format.extensions, ...format.mediaTypes].join(",") : undefined}
          aria-label={t("project.csvFile")} disabled={busy}
          onChange={(e) => { choose(e.currentTarget.files?.[0]); e.currentTarget.value = ""; }} />
        {"file" in step && step.file && <span class={styles.fileName}>{step.file.name}</span>}
      </div>

      {step.state === "previewing" && <Notice>{t("input.csv.previewing")}</Notice>}
      {step.state === "error" && <Notice kind="error">{step.message}</Notice>}
      {step.state === "done" && (
        <Notice kind="success">
          {step.result.recordsRead !== undefined
            ? t("project.analysisCsvImported", { read: step.result.recordsRead, imported: step.result.imported, skipped: step.result.skipped })
            : t("project.csvImported", { imported: step.result.imported, skipped: step.result.skipped })}
        </Notice>
      )}
      {(step.state === "preview" || step.state === "importing") && (
        <PreviewPanel preview={step.preview} busy={step.state === "importing"} onConfirm={confirm} onCancel={() => setStep({ state: "idle" })} />
      )}
    </div>
  );
}

function PreviewPanel({ preview, busy, onConfirm, onCancel }: { preview: ImportPreview; busy: boolean; onConfirm: () => void; onCancel: () => void }) {
  const { t, number } = useI18n();
  const stats = [
    [preview.recordsRead, t("input.preview.rows")],
    [preview.totalDocuments, t("input.preview.documents")],
    [preview.skipped, t("input.preview.skipped")],
  ] as const;
  return (
    <section aria-label={t("input.preview.title")}>
      <h3>{t("input.preview.title")}</h3>
      <div class={styles.summary}>
        {stats.map(([value, text]) => (
          <div key={text} class={styles.stat}><div class={styles.statValue}>{number(value)}</div><div class={styles.statLabel}>{text}</div></div>
        ))}
      </div>
      {preview.totalDocuments === 0 && <Notice kind="warning">{t("input.preview.nothingToImport")}</Notice>}
      {preview.errors.length > 0 && (
        <Notice kind="warning">
          {t("input.preview.rowErrors", { count: preview.errors.length })}
          <ul class={styles.errors}>
            {preview.errors.slice(0, MAX_LISTED_ERRORS).map((e) => <li key={e.row}>{t("input.preview.rowError", { row: e.row, reason: e.reason })}</li>)}
          </ul>
          {preview.errors.length > MAX_LISTED_ERRORS && t("input.preview.moreErrors", { count: preview.errors.length - MAX_LISTED_ERRORS })}
        </Notice>
      )}
      {preview.documents.length > 0 && (
        <>
          <p class={styles.meta}>{t("input.preview.firstDocuments", { shown: preview.documents.length, total: preview.totalDocuments })}</p>
          <DocumentList documents={preview.documents} />
        </>
      )}
      <Disclosure summary={t("input.preview.profileTitle")}>
        {preview.profile ? (
          <>
            <p class={styles.meta}>{t("input.preview.profileMeta", { rows: preview.profile.rowCount, version: preview.profile.profilerVersion })} <code>{preview.fileHash}</code></p>
            <div class={styles.tableWrap}>
              <table class={styles.table}>
                <thead><tr>
                  <th scope="col">{t("input.profile.column")}</th><th scope="col">{t("input.profile.type")}</th>
                  <th scope="col" class={styles.num}>{t("input.profile.filled")}</th><th scope="col" class={styles.num}>{t("input.profile.empty")}</th>
                  <th scope="col" class={styles.num}>{t("input.profile.distinct")}</th><th scope="col">{t("input.profile.samples")}</th>
                </tr></thead>
                <tbody>
                  {preview.profile.columns.map((c) => (
                    <tr key={c.name}>
                      <td><code>{c.name}</code></td><td><code>{c.type}</code></td>
                      <td class={styles.num}>{number(c.nonNullCount)}</td><td class={styles.num}>{number(c.nullCount)}</td>
                      <td class={styles.num}>{number(c.distinctCount)}{c.distinctCapped ? "+" : ""}</td>
                      <td>{c.sampleValues.join(" | ")}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        ) : <p class={styles.meta}>{t("input.preview.noProfile", { reason: preview.profileError ?? "" })}</p>}
      </Disclosure>
      <div class={styles.kinds}>
        <Button variant="primary" onClick={onConfirm} disabled={busy || preview.totalDocuments === 0}>
          {preview.kind === "analysis" ? t("project.importAnalysisData") : t("project.import")}
        </Button>
        <Button onClick={onCancel} disabled={busy}>{t("common.cancel")}</Button>
      </div>
    </section>
  );
}
