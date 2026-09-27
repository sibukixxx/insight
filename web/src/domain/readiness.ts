// Pre-run input checks. They mirror conditions the server enforces or will
// fail on so the user sees them before starting a run; the server stays
// authoritative and its own error is always shown if a run still fails.

import type { AnalysisRun, EvidenceDocument, LlmSettings } from "./models";
import type { MessageKey } from "./messages";
import { isActive } from "./runs";

export type CheckLevel = "ok" | "info" | "warning" | "blocked";

export interface ReadinessCheck {
  readonly id: "evidence" | "activeRun" | "model";
  readonly level: CheckLevel;
  readonly message: MessageKey;
  readonly params?: Readonly<Record<string, string | number>>;
  /** Where the user can fix it. */
  readonly fix?: "input" | "settings";
}

export interface Readiness {
  readonly checks: readonly ReadinessCheck[];
  readonly canStart: boolean;
}

export function assessReadiness(documents: readonly EvidenceDocument[], runs: readonly AnalysisRun[], settings: LlmSettings | undefined): Readiness {
  const checks: ReadinessCheck[] = [];
  const datasets = documents.filter((d) => d.source === "dataset").length;

  if (documents.length === 0) {
    checks.push({ id: "evidence", level: "blocked", message: "readiness.noEvidence", fix: "input" });
  } else {
    checks.push({ id: "evidence", level: "ok", message: "readiness.evidence", params: { documents: documents.length, datasets } });
  }

  const active = runs.find((r) => isActive(r));
  if (active) checks.push({ id: "activeRun", level: "blocked", message: "readiness.activeRun", params: { id: active.id } });

  if (settings === undefined) {
    checks.push({ id: "model", level: "warning", message: "readiness.settingsUnknown", fix: "settings" });
  } else if (settings.configured) {
    checks.push({ id: "model", level: "ok", message: "readiness.modelConfigured", params: { model: settings.model } });
  } else if (datasets > 0) {
    // pipeline.go: without a model only dataset documents are pre-analyzed.
    checks.push({ id: "model", level: "info", message: "readiness.deterministicOnly", fix: "settings" });
  } else if (documents.length > 0) {
    checks.push({ id: "model", level: "blocked", message: "readiness.noModelNoDataset", fix: "settings" });
  }

  return { checks, canStart: !checks.some((c) => c.level === "blocked") };
}
