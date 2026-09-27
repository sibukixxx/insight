import { useState } from "preact/hooks";
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
  const [error, setError] = useState<string | undefined>(undefined);
  const [notice, setNotice] = useState<string | undefined>(undefined);
  const sources = sourceTypes.length ? sourceTypes : Object.keys(SOURCE_LABELS);
  const onSubmit = (e: Event) => {
    e.preventDefault();
    const form = e.currentTarget as HTMLFormElement;
    const data = new FormData(form);
    setBusy(true);
    setError(undefined);
    setNotice(undefined);
    input.addTextEvidence(projectId, {
      source: String(data.get("source") ?? ""), title: String(data.get("title") ?? ""), content: String(data.get("content") ?? ""),
    }).then(
      (doc) => { form.reset(); setNotice(t("input.text.added", { title: doc.title || t("project.untitled") })); onAdded(); },
      (err: unknown) => setError(errorMessage(err, t)),
    ).finally(() => setBusy(false));
  };
  return (
    <form id="paste-form" class={formStyles.form} onSubmit={onSubmit}>
      <div class={formStyles.row}>
        <Field label={t("project.sourceType")} htmlFor="paste-source" requirement="required" requirementLabel={t("common.required")}>
          <select id="paste-source" name="source" class={formStyles.control}>
            {sources.map((code) => <option key={code} value={code}>{label(SOURCE_LABELS, code)}</option>)}
          </select>
        </Field>
        <Field label={t("project.docTitle")} htmlFor="paste-title" requirement="optional" requirementLabel={t("common.optional")}>
          <input id="paste-title" name="title" type="text" class={formStyles.control} placeholder={t("project.docTitlePlaceholder")} />
        </Field>
      </div>
      <Field label={t("project.docContent")} htmlFor="paste-content" requirement="required" requirementLabel={t("common.required")}>
        <textarea id="paste-content" aria-label={t("project.docContent")} name="content" class={formStyles.control} placeholder={t("project.docContentPlaceholder")} required />
      </Field>
      {error && <Notice kind="error">{error}</Notice>}
      {notice && <Notice kind="success">{notice}</Notice>}
      <div class={formStyles.actions}>
        <Button type="submit" variant="primary" disabled={busy}>{t("project.addDocument")}</Button>
      </div>
    </form>
  );
}
