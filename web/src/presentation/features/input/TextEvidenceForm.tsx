import { useState } from "preact/hooks";
import { ValidationSummary } from "../../components/ValidationSummary";
import { useFormValidation } from "../../hooks/useFormValidation";
import { SOURCE_LABELS } from "../../../domain/codes";
import { Button } from "../../components/Button";
import { Field, formStyles } from "../../components/Field";
import { Notice } from "../../components/Notice";
import { useI18n } from "../../i18n/I18nProvider";
import { errorMessage } from "../../errors";
import { useUseCases } from "../../services/context";

export function TextEvidenceForm({ projectId, sourceTypes, onAdded }: { projectId: string; sourceTypes: readonly string[]; onAdded: () => void }) {
  const { t, label } = useI18n();
  const { input } = useUseCases();
  const [busy, setBusy] = useState(false);
  const validation = useFormValidation();
  const [error, setError] = useState<string | undefined>(undefined);
  const [notice, setNotice] = useState<string | undefined>(undefined);
  const sources = sourceTypes;
  const onSubmit = (e: Event) => {
    e.preventDefault();
    const form = e.currentTarget as HTMLFormElement;
    const data = new FormData(form);
    const source = String(data.get("source") ?? "");
    const content = String(data.get("content") ?? "");
    if (busy || !validation.validate([
      ...(!sources.includes(source) ? [{ id: "paste-source", label: "project.sourceType" as const, message: "form.selectValue" as const }] : []),
      ...(!content.trim() ? [{ id: "paste-content", label: "project.docContent" as const, message: "form.requiredValue" as const }] : []),
    ])) return;
    setBusy(true);
    setError(undefined);
    setNotice(undefined);
    input.addTextEvidence(projectId, {
      source: String(data.get("source") ?? ""), title: String(data.get("title") ?? ""), content: String(data.get("content") ?? ""),
    }).then(
      (doc) => { form.reset(); validation.clear(); setNotice(t("input.text.added", { title: doc.title || t("project.untitled") })); onAdded(); },
      (err: unknown) => setError(errorMessage(err, t)),
    ).finally(() => setBusy(false));
  };
  return (
    <form id="paste-form" class={formStyles.form} noValidate onSubmit={onSubmit}>
      <ValidationSummary errors={validation.errors} />
      <div class={formStyles.row}>
        <Field label={t("project.sourceType")} htmlFor="paste-source" hint={t("form.sourceHint")} error={validation.error("paste-source")} requirement="required" requirementLabel={t("common.required")}>
            {(control) => (<select {...control} name="source" onChange={() => validation.clear("paste-source")} class={formStyles.control}>
            {sources.map((code) => <option key={code} value={code}>{label(SOURCE_LABELS, code)}</option>)}
          </select>)}
          </Field>
        <Field label={t("project.docTitle")} htmlFor="paste-title" hint={t("form.titleHint")} requirement="optional" requirementLabel={t("common.optional")}>
            {(control) => (<input {...control} name="title" type="text" class={formStyles.control} placeholder={t("project.docTitlePlaceholder")} />)}
          </Field>
      </div>
      <Field label={t("project.docContent")} htmlFor="paste-content" hint={t("form.contentHint")} example={t("form.contentExample")} error={validation.error("paste-content")} requirement="required" requirementLabel={t("common.required")}>
            {(control) => (<textarea {...control} aria-label={t("project.docContent")} name="content" onInput={() => validation.clear("paste-content")} class={formStyles.control} placeholder={t("project.docContentPlaceholder")} />)}
          </Field>
      {error && <Notice kind="error">{error}</Notice>}
      {notice && <Notice kind="success">{notice}</Notice>}
      <div class={formStyles.actions}>
        <Button type="submit" variant="primary" disabled={busy}>{t("project.addDocument")}</Button>
      </div>
    </form>
  );
}
