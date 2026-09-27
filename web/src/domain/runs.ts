// Run selection and provenance read models. Every result view is bound to
// exactly one analysis run: the one named by ?run=<id>, else the latest
// completed run. Queued, running and failed runs have no results.

import type { AnalysisRun, ExecutionSnapshot, RunProvenance } from "./models";

/** A provenance value that was recorded, or explicitly not recorded. */
export type Recorded =
  | { readonly kind: "value"; readonly value: string; readonly full?: string }
  | { readonly kind: "noModel" }
  | { readonly kind: "notRecorded" };

const notRecorded: Recorded = { kind: "notRecorded" };
const value = (v: string, full?: string): Recorded => (full === undefined ? { kind: "value", value: v } : { kind: "value", value: v, full });

export interface RunSelection {
  readonly run: AnalysisRun | undefined;
  /** True when ?run= named a run this project does not have. */
  readonly requestedRunMissing: boolean;
}

function finishedOrCreated(run: AnalysisRun): string {
  return run.finishedAt ?? run.createdAt;
}

/** Same rule as the server: the most recently finished completed run. */
export function latestCompletedRun(runs: readonly AnalysisRun[]): AnalysisRun | undefined {
  return runs
    .filter((r) => r.status === "completed")
    .slice()
    .sort((a, b) => finishedOrCreated(b).localeCompare(finishedOrCreated(a)))[0];
}

export function pickRun(runs: readonly AnalysisRun[], requestedId: string | undefined): RunSelection {
  if (requestedId) {
    const found = runs.find((r) => r.id === requestedId);
    if (found) return { run: found, requestedRunMissing: false };
  }
  return { run: latestCompletedRun(runs), requestedRunMissing: Boolean(requestedId) };
}

export function isActive(run: AnalysisRun | undefined): boolean {
  return run !== undefined && (run.status === "running" || run.status === "queued");
}

/** The newest run by creation, whatever its status (the list is newest first). */
export function latestRun(runs: readonly AnalysisRun[]): AnalysisRun | undefined {
  return runs[0];
}

export function runProvenance(run: AnalysisRun | undefined): RunProvenance {
  return run?.metrics?.provenance ?? {};
}

// Runs recorded before snapshots existed carry no fingerprints; that is
// "not recorded", never "same as another run".
export function shortFingerprint(fp: string | undefined): Recorded {
  return fp ? value(fp.replace(/^sha256:/, "").slice(0, 7), fp) : notRecorded;
}

export function engineLabel(execution: ExecutionSnapshot | undefined): Recorded {
  if (!execution) return notRecorded;
  const commit = execution.gitCommit && execution.gitCommit !== "UNKNOWN" ? `@${execution.gitCommit.slice(0, 7)}` : "";
  const dirty = execution.gitDirty === "true" ? "+dirty" : "";
  return value(`${execution.engineVersion ?? "UNKNOWN"}${commit}${dirty}`, execution.gitCommit);
}

// A deterministic run used no model; that is a recorded fact, not a gap.
function modelScoped(prov: RunProvenance, v: string | undefined, full?: string): Recorded {
  if (v) return value(v, full);
  if (prov.mode === "deterministic") return { kind: "noModel" };
  return notRecorded;
}

export interface ProvenanceItem {
  readonly field: "mode" | "model" | "prompt" | "rules" | "engine" | "execution" | "input" | "question";
  readonly value: Recorded;
  /** True when value.value is a raw code (e.g. the mode) to be labelled. */
  readonly isModeCode?: boolean;
}

export function provenanceItems(run: AnalysisRun): readonly ProvenanceItem[] {
  const prov = runProvenance(run);
  const fingerprint = prov.promptFingerprint ? prov.promptFingerprint.slice(0, 7) : undefined;
  const items: ProvenanceItem[] = [
    { field: "mode", value: prov.mode ? value(prov.mode) : notRecorded, isModeCode: true },
    { field: "model", value: modelScoped(prov, prov.model) },
    { field: "prompt", value: modelScoped(prov, fingerprint, prov.promptFingerprint) },
    { field: "rules", value: prov.ruleVersion ? value(prov.ruleVersion) : notRecorded },
    { field: "engine", value: engineLabel(run.executionSnapshot) },
    { field: "execution", value: shortFingerprint(run.executionFingerprint) },
    { field: "input", value: shortFingerprint(run.inputFingerprint) },
  ];
  if (run.researchQuestion) items.push({ field: "question", value: value(run.researchQuestion, run.researchQuestion) });
  return items;
}

/** Completed runs, newest first, eligible for comparison. */
export function completedRuns(runs: readonly AnalysisRun[]): readonly AnalysisRun[] {
  return runs.filter((r) => r.status === "completed");
}
