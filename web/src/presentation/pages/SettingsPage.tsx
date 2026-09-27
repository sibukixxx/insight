import { useState } from "preact/hooks";
import { MODE_LABELS } from "../../domain/codes";
import { Button } from "../components/Button";
import { Card } from "../components/Card";
import { Field, formStyles } from "../components/Field";
import { LocaleSelect } from "../components/LocaleSelect";
import { Notice } from "../components/Notice";
import { PageHeader } from "../components/PageHeader";
import { ErrorState, Loading } from "../components/States";
import { useAsync } from "../hooks/useAsync";
import { useI18n } from "../i18n/I18nProvider";
import { errorMessage } from "../errors";
import { useUseCases } from "../services/context";
import styles from "./Page.module.css";

type Feedback = { readonly kind: "success" | "error"; readonly text: string } | undefined;

export function SettingsPage() {
  const { t, label } = useI18n();
  const { settings } = useUseCases();
  const state = useAsync(() => settings.loadSettings(), [settings]);
  const [feedback, setFeedback] = useState<Feedback>(undefined);
  const [busy, setBusy] = useState(false);

  const save = (e: Event) => {
    e.preventDefault();
    const form = e.currentTarget as HTMLFormElement;
    const data = new FormData(form);
    setBusy(true);
    settings.saveSettings({ baseUrl: String(data.get("baseUrl") ?? ""), model: String(data.get("model") ?? ""), apiKey: String(data.get("apiKey") ?? "") }).then(
      () => { setFeedback({ kind: "success", text: t("settings.saved") }); state.reload(); const key = form.elements.namedItem("apiKey"); if (key instanceof HTMLInputElement) key.value = ""; },
      (err: unknown) => setFeedback({ kind: "error", text: errorMessage(err, t) }),
    ).finally(() => setBusy(false));
  };
  const test = () => {
    setBusy(true);
    settings.testConnection().then(
      (mode) => setFeedback({ kind: "success", text: t("settings.connectionOk", { mode: label(MODE_LABELS, mode) }) }),
      (err: unknown) => setFeedback({ kind: "error", text: t("settings.connectionFailed", { message: errorMessage(err, t) }) }),
    ).finally(() => setBusy(false));
  };

  return (
    <>
      <PageHeader back={{ href: "#/", label: t("nav.back") }} title={t("nav.settings")} />
      <div class={styles.stack}>
        <Card title={t("settings.displayLanguage")}>
          <Field label={t("locale.label")} htmlFor="locale-select-settings" hint={t("settings.localeHint")} requirement="optional">
            {(control) => (<LocaleSelect {...control} />)}
          </Field>
        </Card>
        <Card title={t("settings.llm")} description={t("settings.llmHint")}>
          {state.status === "loading" && <Loading />}
          {state.status === "error" && <ErrorState error={state.error} onRetry={state.reload} />}
          {state.status === "ok" && (
            <form id="settings-form" class={formStyles.form} onSubmit={save}>
              <Notice kind={state.data.configured ? "success" : "info"}>
                {state.data.configured ? t("settings.statusConfigured", { model: state.data.model }) : t("settings.statusNotConfigured")}
              </Notice>
              <Field label={t("settings.baseUrl")} htmlFor="settings-base-url" hint={t("form.modelLaterHint")} requirement="required-later">
            {(control) => (<input {...control} name="baseUrl" type="text" class={formStyles.control} defaultValue={state.data.baseUrl} placeholder="https://api.openai.com/v1" />)}
          </Field>
              <Field label={t("settings.model")} htmlFor="settings-model" hint={t("form.modelLaterHint")} requirement="required-later">
            {(control) => (<input {...control} name="model" type="text" class={formStyles.control} defaultValue={state.data.model} placeholder="gpt-5" />)}
          </Field>
              <Field label={state.data.hasApiKey ? t("settings.apiKeyConfigured", { masked: state.data.maskedApiKey }) : t("settings.apiKey")} htmlFor="settings-api-key" hint={t("settings.apiKeyHint")} requirement="optional">
            {(control) => (<input {...control} name="apiKey" type="password" autoComplete="off" class={formStyles.control} placeholder={state.data.hasApiKey ? t("settings.apiKeyReplace") : "sk-..."} />)}
          </Field>
              {feedback && <Notice kind={feedback.kind}>{feedback.text}</Notice>}
              <div class={formStyles.actions}>
                <Button type="submit" variant="primary" disabled={busy}>{t("settings.save")}</Button>
                <Button id="test-connection" onClick={test} disabled={busy}>{t("settings.testConnection")}</Button>
              </div>
            </form>
          )}
        </Card>
      </div>
    </>
  );
}
