// Pure helpers for showing a quote inside its source document.

import type { Evidence, Pattern } from "./models";

export interface QuoteContext {
  readonly before: string;
  readonly quote: string;
  readonly after: string;
}

/** Splits content around [start, end); out-of-range offsets are clamped. */
export function quoteContext(content: string, start: number, end: number): QuoteContext {
  const s = Math.max(0, Math.min(content.length, Number.isFinite(start) ? start : 0));
  const e = Math.max(s, Math.min(content.length, Number.isFinite(end) ? end : s));
  return { before: content.slice(0, s), quote: content.slice(s, e), after: content.slice(e) };
}

export function splitEvidence(evidence: readonly Evidence[]): { support: readonly Evidence[]; counter: readonly Evidence[]; other: readonly Evidence[] } {
  return {
    support: evidence.filter((e) => e.type === "support"),
    counter: evidence.filter((e) => e.type === "counter"),
    other: evidence.filter((e) => e.type !== "support" && e.type !== "counter"),
  };
}

export function splitPatterns(patterns: readonly Pattern[]): { traces: readonly Pattern[]; repetitions: readonly Pattern[] } {
  return {
    traces: patterns.filter((p) => p.kind === "deviation"),
    repetitions: patterns.filter((p) => p.kind !== "deviation"),
  };
}

/**
 * The evidence chain of one insight, counted for the relationship view:
 * source observations → patterns → hypothesis → support / counter evidence.
 * It restates what the server linked; it adds no inference.
 */
export interface EvidenceChain {
  readonly observations: number;
  readonly documents: number;
  readonly traces: number;
  readonly repetitions: number;
  readonly support: number;
  readonly counter: number;
}

export function evidenceChain(patterns: readonly Pattern[], evidence: readonly Evidence[]): EvidenceChain {
  const observations = patterns.flatMap((p) => p.observations);
  const { traces, repetitions } = splitPatterns(patterns);
  const { support, counter } = splitEvidence(evidence);
  const documents = new Set([...observations.map((o) => o.documentId), ...evidence.map((e) => e.documentId)]);
  return {
    observations: new Set(observations.map((o) => o.id)).size,
    documents: documents.size,
    traces: traces.length,
    repetitions: repetitions.length,
    support: support.length,
    counter: counter.length,
  };
}
