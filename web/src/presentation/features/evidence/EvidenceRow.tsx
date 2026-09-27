import { useState } from "preact/hooks";
import { quoteContext } from "../../../domain/evidence";
import type { EvidenceDocument, QuoteSpan } from "../../../domain/models";
import { useI18n } from "../../i18n/I18nProvider";
import { errorMessage } from "../../errors";
import { useUseCases } from "../../services/context";
import styles from "./Evidence.module.css";

type Panel = { readonly state: "closed" } | { readonly state: "loading" } | { readonly state: "open"; readonly doc: EvidenceDocument } | { readonly state: "error"; readonly message: string };

/** A quote that reveals itself inside its source document. All text is rendered as text. */
export function EvidenceRow({ span }: { span: QuoteSpan }) {
  const { t } = useI18n();
  const { input } = useUseCases();
  const [panel, setPanel] = useState<Panel>({ state: "closed" });
  const [doc, setDoc] = useState<EvidenceDocument | undefined>(undefined);
  const toggle = () => {
    if (panel.state === "open") return setPanel({ state: "closed" });
    if (doc) return setPanel({ state: "open", doc });
    setPanel({ state: "loading" });
    input.sourceDocument(span.documentId).then(
      (loaded) => { setDoc(loaded); setPanel({ state: "open", doc: loaded }); },
      (error: unknown) => setPanel({ state: "error", message: errorMessage(error, t) }),
    );
  };
  const context = panel.state === "open" ? quoteContext(panel.doc.content, span.startOffset, span.endOffset) : undefined;
  return (
    <div class={styles.row}>
      <button type="button" class={styles.toggle} aria-expanded={panel.state === "open"} onClick={toggle}>
        <span class={styles.quote}>{"\""}{span.quote}{"\""}</span>
        <span class={styles.reveal}>{t("evidence.viewInSource")} {panel.state === "open" ? "↑" : "↓"}</span>
      </button>
      {panel.state === "loading" && <div class={styles.context} role="status">{t("common.loading")}</div>}
      {panel.state === "error" && <div class={styles.context} role="alert">{panel.message}</div>}
      {panel.state === "open" && context && (
        <div class={styles.context}>
          <div class={styles.docTitle}>{panel.doc.title || panel.doc.id}</div>
          <div class={styles.contextText}>{context.before}<mark>{context.quote}</mark>{context.after}</div>
        </div>
      )}
    </div>
  );
}
