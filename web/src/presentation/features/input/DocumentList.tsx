import { useState } from "preact/hooks";
import type { EvidenceDocument } from "../../../domain/models";
import { Button } from "../../components/Button";
import { EmptyState } from "../../components/States";
import { useI18n } from "../../i18n/I18nProvider";
import { SourceTag } from "./SourceTag";
import styles from "./Input.module.css";

function DocumentItem({ doc }: { doc: EvidenceDocument }) {
  const { t } = useI18n();
  const [expanded, setExpanded] = useState(false);
  const long = doc.content.length > 400 || doc.content.split("\n").length > 5;
  return (
    <li class={styles.doc}>
      <div class={styles.docHead}>
        <SourceTag source={doc.source} />
        <span class={styles.docTitle}>{doc.title ? doc.title : t("project.untitled")}</span>
      </div>
      <div class={[styles.docContent, (expanded || !long) && styles.docExpanded].filter(Boolean).join(" ")}>{doc.content}</div>
      {long && <Button variant="ghost" size="small" aria-expanded={expanded} onClick={() => setExpanded(!expanded)}>{expanded ? t("common.showLess") : t("common.showMore")}</Button>}
    </li>
  );
}

export function DocumentList({ documents, emptyText }: { documents: readonly EvidenceDocument[]; emptyText?: string }) {
  const { t } = useI18n();
  if (documents.length === 0) return <EmptyState>{emptyText ?? t("project.noDocuments")}</EmptyState>;
  return <ul class={styles.docs}>{documents.map((d) => <DocumentItem key={d.id} doc={d} />)}</ul>;
}
