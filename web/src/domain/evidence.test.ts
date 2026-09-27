import { describe, expect, it } from "vitest";
import { evidenceChain, quoteContext } from "./evidence";

describe("quoteContext", () => {
  it("splits content around the quoted span", () => {
    expect(quoteContext("abcdef", 2, 4)).toEqual({ before: "ab", quote: "cd", after: "ef" });
  });

  it("clamps offsets outside the content instead of throwing", () => {
    expect(quoteContext("abc", -5, 99)).toEqual({ before: "", quote: "abc", after: "" });
    expect(quoteContext("abc", 2, 1)).toEqual({ before: "ab", quote: "", after: "c" });
  });
});

describe("evidenceChain", () => {
  it("counts linked observations, documents, patterns and evidence without inference", () => {
    const obs = (id: string, documentId: string) => ({ id, documentId, quote: "q", behavior: "", startOffset: 0, endOffset: 1 });
    const chain = evidenceChain(
      [{ id: "p1", kind: "deviation", title: "", observations: [obs("o1", "d1"), obs("o2", "d2")] }, { id: "p2", kind: "repetition", title: "", observations: [obs("o1", "d1")] }],
      [{ id: "e1", documentId: "d3", quote: "", type: "support", relevanceScore: 1, startOffset: 0, endOffset: 1 }, { id: "e2", documentId: "d1", quote: "", type: "counter", relevanceScore: 1, startOffset: 0, endOffset: 1 }],
    );
    expect(chain).toEqual({ observations: 2, documents: 3, traces: 1, repetitions: 1, support: 1, counter: 1 });
  });
});
