import { useEffect, useId, useState } from "preact/hooks";
import { matchesFormat } from "../../../application/usecases/importEvidence";
import type { UploadFile } from "../../../application/ports";
import { IMPORT_KIND_LABELS, labelKey } from "../../../domain/codes";
import type { DocumentImportResult, ImportFormat, ImportPreview } from "../../../domain/models";
import { Button, ButtonLink } from "../../components/Button";
import { Field, formStyles } from "../../components/Field";
import { focusField, ValidationSummary } from "../../components/ValidationSummary";
import { FormatGuide } from "./FormatGuide";
import { Disclosure } from "../../components/Disclosure";
import { Notice } from "../../components/Notice";
import { useI18n } from "../../i18n/I18nProvider";
import { errorMessage } from "../../errors";
import { useUseCases } from "../../services/context";
import { DocumentList } from "./DocumentList";
import styles from "./Input.module.css";

type Step =
  | { readonly state: "idle" }
  | { readonly state: "loadingSample" }
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
/** A bundled sample CSV the user can put through the same preview → import. */
export interface CsvSample {
  readonly kind: string;
  readonly rows: number;
  readonly downloadHref: string;
  readonly load: () => Promise<UploadFile>;
}

export function CsvImport({ projectId, formats, onImported, sample }: { projectId: string; formats: readonly ImportFormat[]; onImported: () => void; sample?: CsvSample | undefined }) {
  const i18n = useI18n();
  const { t } = i18n;
  const { input } = useUseCases();
  const fileInputId = useId();
  const [kind, setKind] = useState(formats.find((f) => f.kind === "documents")?.kind ?? formats[0]?.kind ?? "");
  const [step, setStep] = useState<Step>({ state: "idle" });
  const [dragging, setDragging] = useState(false);
  const format = formats.find((f) => f.kind === kind);
  useEffect(() => { if (step.state === "error") focusField(fileInputId); }, [step, fileInputId]);

  const choose = (file: File | undefined) => {
    if (!file || !format) return;
    runPreview({ name: file.name, blob: file }, format);
  };

  const runPreview = (upload: UploadFile, target: ImportFormat) => {
    const kind = target.kind;
    if (!matchesFormat(upload.name, target)) {
      setStep({ state: "error", message: t("input.csv.wrongType", { name: upload.name, extensions: target.extensions.join(", ") }) });
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

  const useSample = () => {
    const sampleFormat = sample && formats.find((f) => f.kind === sample.kind);
    if (!sample || !sampleFormat) return;
    setKind(sampleFormat.kind);
    setStep({ state: "loadingSample" });
    sample.load().then(
      (upload) => runPreview(upload, sampleFormat),
      (err: unknown) => setStep({ state: "error", message: errorMessage(err, t) }),
    );
  };

  const kindLabel = (k: string) => (labelKey(IMPORT_KIND_LABELS, k) ? i18n.label(IMPORT_KIND_LABELS, k) : k);
  const busy = step.state === "previewing" || step.state === "importing" || step.state === "loadingSample";

  return (
    <div class={styles.chooser}>
      {sample && (
        <div class={styles.sample} role="group" aria-label={t("input.sample.label")}>
          <div>
            <p class={styles.sampleTitle}>{t("input.sample.title")}</p>
            <p class={styles.meta}>{t("input.sample.hint", { format: kindLabel(sample.kind), rows: sample.rows })}</p>
          </div>
          <div class={styles.kinds}>
            <Button id="use-sample-csv" variant="primary" onClick={useSample} disabled={busy}>{t("input.sample.preview")}</Button>
            <ButtonLink size="small" variant="ghost" href={sample.downloadHref} download>{t("samples.download")}</ButtonLink>
          </div>
        </div>
      )}
      <p class={styles.meta}>{t("input.csv.steps")}</p>
      <ValidationSummary errors={step.state === "error" ? [{ id: fileInputId, label: t("project.csvFile"), message: step.message }] : []} />
      <fieldset class={styles.kinds} aria-describedby={`${fileInputId}-selected`}>
        <legend>{t("input.csv.kind")} <span class={formStyles.required}>{t("common.required")}</span></legend>
        {formats.filter((f) => f.kind === "documents").map((f) => (
          <label key={f.kind} class={styles.kind}>
            <input type="radio" name="import-kind" value={f.kind} checked={kind === f.kind} required aria-required="true" disabled={busy}
              onChange={() => { setKind(f.kind); setStep({ state: "idle" }); }} />
            {kindLabel(f.kind)}
          </label>
        ))}
        {formats.some((f) => f.kind !== "documents") && <Disclosure summary={t("input.csv.otherFormats")} open={kind !== "documents"}>
          {formats.filter((f) => f.kind !== "documents").map((f) => (
            <label key={f.kind} class={styles.kind}>
              <input type="radio" name="import-kind" value={f.kind} checked={kind === f.kind} required aria-required="true" disabled={busy}
                onChange={() => { setKind(f.kind); setStep({ state: "idle" }); }} />
              {kindLabel(f.kind)}
            </label>
          ))}
        </Disclosure>}
      </fieldset>
      <p id={`${fileInputId}-selected`} class={styles.meta}>{t("input.csv.selectedFormat", { format: kindLabel(kind) })}</p>
      {format ? <FormatGuide formats={[format]} /> : <Notice kind="warning">{t("input.csv.noFormats")}</Notice>}
      <div
        class={[styles.drop, dragging && styles.dragging].filter(Boolean).join(" ")}
        onDragOver={(e) => { e.preventDefault(); setDragging(true); }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => {
          e.preventDefault(); setDragging(false);
          if (busy || !format || !e.dataTransfer?.files.length) return;
          const control = document.getElementById(fileInputId);
          if (control instanceof HTMLInputElement) control.files = e.dataTransfer.files;
          choose(e.dataTransfer.files[0]);
        }}
      >
        <Field label={t("project.csvFile")} htmlFor={fileInputId} requirement="required"
          hint={t("input.csv.dropHint", { extensions: format?.extensions.join(", ") ?? "" })}
          error={step.state === "error" ? step.message : undefined}>
          {(control) => <input {...control} key={kind} class={formStyles.control} type="file" name="file"
            accept={format ? [...format.extensions, ...format.mediaTypes].join(",") : undefined} disabled={busy || !format}
            onChange={(e) => choose(e.currentTarget.files?.[0])} />}
        </Field>
        {"file" in step && step.file && <span class={styles.fileName}>{step.file.name}</span>}
      </div>

      {(step.state === "previewing" || step.state === "loadingSample") && <Notice>{t("input.csv.previewing")}</Notice>}
      {step.state === "done" && (
        <Notice kind="success">
          <p>{t("input.csv.importedNext")}</p>
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
    [preview.importable, t("input.preview.importable")],
    [preview.totalDocuments, t("input.preview.documents")],
    [preview.skipped, t("input.preview.skipped")],
  ] as const;
  return (
    <section aria-label={t("input.preview.title")}>
      <h3>{t("input.preview.title")}</h3>
      <p class={styles.meta}>{t("input.preview.confirmHint")}</p>
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
