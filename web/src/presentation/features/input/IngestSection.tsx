import { useEffect, useRef, useState } from "preact/hooks";
import { INGEST_STATE_LABELS, IMPORT_KIND_LABELS, labelKey } from "../../../domain/codes";
import { ingestPercent, isTerminalIngest } from "../../../domain/ingest";
import type { IngestReceipt } from "../../../domain/models";
import { Badge, type Tone } from "../../components/Badge";
import { Button, ButtonLink } from "../../components/Button";
import { Card } from "../../components/Card";
import { Meter } from "../../components/Meter";
import { Notice } from "../../components/Notice";
import { useI18n } from "../../i18n/I18nProvider";
import { errorMessage } from "../../errors";
import { projectHash } from "../../router/routes";
import { useUseCases } from "../../services/context";
import { DocumentList } from "./DocumentList";
import styles from "./Input.module.css";

const MAX_RECEIPTS = 5;
const MAX_LISTED_ERRORS = 10;
const MIB = 1024 * 1024;

const TONES: Readonly<Record<string, Tone>> = { QUEUED: "neutral", VALIDATING: "accent", READY: "success", FAILED: "danger", CANCELLED: "warning" };

/**
 * Large-CSV imports of this project, newest first. The list is read from
 * the server, so leaving the page and coming back shows each receipt's
 * recorded state; a receipt that is still running keeps updating itself.
 */
export function IngestSection({ projectId, refreshKey, onReady }: { projectId: string; refreshKey: number; onReady: () => void }) {
  const { t } = useI18n();
  const { input } = useUseCases();
  const [receipts, setReceipts] = useState<readonly IngestReceipt[]>([]);
  const [failure, setFailure] = useState<string | undefined>(undefined);
  useEffect(() => {
    let live = true;
    input.listIngests(projectId).then(
      (list) => { if (live) { setReceipts(list); setFailure(undefined); } },
      (e: unknown) => { if (live) setFailure(errorMessage(e, t)); },
    );
    return () => { live = false; };
  }, [input, projectId, refreshKey]);
  if (receipts.length === 0 && !failure) return null;
  return (
    <Card title={t("ingest.title")} description={t("ingest.lead")}>
      <div id="ingest-list" class={styles.chooser}>
        {failure && <Notice kind="warning">{t("ingest.listFailed", { message: failure })}</Notice>}
        {receipts.slice(0, MAX_RECEIPTS).map((r) => <IngestReceiptCard key={r.id} receipt={r} onReady={onReady} />)}
        {receipts.length > MAX_RECEIPTS && <p class={styles.meta}>{t("ingest.older", { count: receipts.length - MAX_RECEIPTS })}</p>}
      </div>
    </Card>
  );
}

/** One receipt. Polls the server until the ingest is READY, FAILED or CANCELLED. */
export function IngestReceiptCard({ receipt: initial, onReady }: { receipt: IngestReceipt; onReady: () => void }) {
  const i18n = useI18n();
  const { t, label, number } = i18n;
  const { input, exports } = useUseCases();
  const [receipt, setReceipt] = useState(initial);
  const [unknown, setUnknown] = useState(false);
  const [actionError, setActionError] = useState<string | undefined>(undefined);
  const [cancelling, setCancelling] = useState(false);
  // The parent reloads documents once when this receipt becomes READY while it is on screen.
  const wasOpen = useRef(!isTerminalIngest(initial.state));
  useEffect(() => input.watchIngest(initial.projectId, initial.id, (next) => {
    setReceipt(next);
    if (wasOpen.current && next.state === "READY") { wasOpen.current = false; onReady(); }
    if (isTerminalIngest(next.state)) wasOpen.current = false;
  }, setUnknown), [input, initial.projectId, initial.id]);

  const terminal = isTerminalIngest(receipt.state);
  const cancel = () => {
    setCancelling(true);
    setActionError(undefined);
    input.cancelIngest(receipt.projectId, receipt.id).then(setReceipt, (e: unknown) => setActionError(errorMessage(e, t))).finally(() => setCancelling(false));
  };
  const kind = labelKey(IMPORT_KIND_LABELS, receipt.kind) ? label(IMPORT_KIND_LABELS, receipt.kind) : receipt.kind;
  const percent = ingestPercent(receipt);
  const shownErrors = receipt.errorExamples.slice(0, MAX_LISTED_ERRORS);
  const stateText = labelKey(INGEST_STATE_LABELS, receipt.state) ? label(INGEST_STATE_LABELS, receipt.state) : receipt.state;

  return (
    <section class={styles.receipt} aria-label={t("ingest.receiptLabel", { name: receipt.fileName ?? receipt.id })} data-ingest-id={receipt.id} data-ingest-state={receipt.state}>
      <div class={styles.docHead}>
        <span class={styles.fileName}>{receipt.fileName ?? receipt.id}</span>
        <Badge tone={TONES[receipt.state] ?? "neutral"}><span class="ingest-state">{stateText}</span></Badge>
        <span class={styles.meta}>{kind} · {t("ingest.size", { size: number(receipt.sizeBytes / MIB, 1) })}</span>
      </div>
      {unknown && !terminal && <Notice kind="warning">{t("ingest.stateUnknown")}</Notice>}
      {!terminal && (
        <>
          <Meter label={t("ingest.progressLabel")} percent={percent} valueText={`${percent}%`} />
          <p class={styles.meta}>{t("ingest.progress", { read: number(receipt.bytesRead / MIB, 1), total: number(receipt.sizeBytes / MIB, 1), rows: number(receipt.rowsRead) })}</p>
          <div class={styles.kinds}>
            <Button id="cancel-ingest" onClick={cancel} disabled={cancelling}>{t("ingest.cancel")}</Button>
          </div>
        </>
      )}
      {actionError && <Notice kind="error">{actionError}</Notice>}
      {receipt.state === "READY" && (
        <Notice kind="success">
          <p>{t("ingest.ready", { documents: number(receipt.documentsCreated), rows: number(receipt.rowsRead), skipped: number(receipt.rowsSkipped) })}</p>
          <ButtonLink variant="primary" href={projectHash(receipt.projectId, "analysis")}>{t("nextStep.analysis.action")} {"→"}</ButtonLink>
        </Notice>
      )}
      {receipt.state === "FAILED" && <Notice kind="error">{t("ingest.failed", { message: receipt.failure ?? t("analysis.unknownError") })}</Notice>}
      {receipt.state === "CANCELLED" && <Notice kind="info">{t("ingest.cancelled")}</Notice>}
      {!labelKey(INGEST_STATE_LABELS, receipt.state) && <Notice kind="warning">{t("ingest.unknownState", { state: receipt.state })}</Notice>}
      {receipt.errorCount > 0 && (
        <Notice kind="warning">
          {t("ingest.rowErrors", { count: number(receipt.errorCount) })}
          <ul class={styles.errors}>
            {shownErrors.map((e) => <li key={e.row}>{t("input.preview.rowError", { row: e.row, reason: e.reason })}</li>)}
          </ul>
          {receipt.errorCount > shownErrors.length && <p>{t("ingest.errorsCapped", { shown: shownErrors.length, total: number(receipt.errorCount) })}</p>}
          <ButtonLink size="small" variant="ghost" href={exports.ingestErrorsLink(receipt.projectId, receipt.id)} download>{t("ingest.downloadErrors")}</ButtonLink>
        </Notice>
      )}
      {receipt.state === "READY" && receipt.preview && receipt.preview.documents.length > 0 && (
        <>
          <p class={styles.meta}><Badge tone="warning" title={t("ingest.sampleHint")}>{t("ingest.sample")}</Badge> {t("ingest.sampleDocs", { shown: receipt.preview.documents.length, total: number(receipt.documentsCreated) })}</p>
          <DocumentList documents={receipt.preview.documents} />
        </>
      )}
    </section>
  );
}
