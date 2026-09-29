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
  /** An evidence-backed (or dataset-only deterministic) run can start. */
  readonly canStart: boolean;
  /**
   * A question-only exploration (#158) can start: no evidence yet, no active
   * run, and the model is configured or unknown (the server has the last word).
   * Saving the question never depends on this.
   */
  readonly canExplore: boolean;
  /** No evidence at all: only a question-only exploration is possible. */
  readonly noEvidence: boolean;
  readonly status: "ready" | "exploratory" | "blocked" | "running" | "unknown";
}

export function assessReadiness(documents: readonly EvidenceDocument[], runs: readonly AnalysisRun[], settings: LlmSettings | undefined): Readiness {
  const checks: ReadinessCheck[] = [];
  const datasets = documents.filter((d) => d.source === "dataset").length;

  const noEvidence = documents.length === 0;
  if (noEvidence) {
    // Not a blocker for a question-only exploration; data-backed analysis
    // still needs input (canStart below stays false).
    checks.push({ id: "evidence", level: "info", message: "readiness.noEvidence", fix: "input" });
  } else {
    checks.push({ id: "evidence", level: "ok", message: "readiness.evidence", params: { documents: documents.length, datasets } });
  }

  const active = runs.find((r) => isActive(r));
  if (active) checks.push({ id: "activeRun", level: "blocked", message: "readiness.activeRun", params: { id: active.id } });

  if (settings === undefined) {
    checks.push({ id: "model", level: "warning", message: "readiness.settingsUnknown", fix: "settings" });
  } else if (settings.configured) {
    checks.push({ id: "model", level: "ok", message: "readiness.modelConfigured", params: { model: settings.model } });
  } else if (noEvidence) {
    checks.push({ id: "model", level: "warning", message: "readiness.explorationNeedsModel", fix: "settings" });
  } else if (datasets > 0) {
    // pipeline.go: without a model only dataset documents are pre-analyzed.
    checks.push({ id: "model", level: "info", message: "readiness.deterministicOnly", fix: "settings" });
  } else if (documents.length > 0) {
    checks.push({ id: "model", level: "blocked", message: "readiness.noModelNoDataset", fix: "settings" });
  }

  const blocked = checks.some((c) => c.level === "blocked");
  const canExplore = noEvidence && !active && settings?.configured !== false;
  const status = active ? "running" : blocked ? "blocked" : noEvidence ? (canExplore ? "exploratory" : "blocked") : settings === undefined ? "unknown" : "ready";
  return { checks, canStart: !blocked && !noEvidence, canExplore, noEvidence, status };
}
