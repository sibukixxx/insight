import { describe, expect, it } from "vitest";
import { SOURCE_LABELS, groupSourceTypes } from "./codes";

describe("groupSourceTypes", () => {
  it("separates customer-research sources from generic ones in server order", () => {
    expect(groupSourceTypes(["document", "interview", "web", "social_post"])).toEqual({ general: ["document", "web"], customerResearch: ["interview", "social_post"] });
  });

  it("keeps unknown codes in the generic group instead of hiding or guessing them", () => {
    expect(groupSourceTypes(["future_kind"])).toEqual({ general: ["future_kind"], customerResearch: [] });
  });

  it("returns an empty customer-research group when the server offers none", () => {
    expect(groupSourceTypes(["document", "dataset"]).customerResearch).toEqual([]);
  });

  it("has a display label for every customer-research source", () => {
    for (const code of groupSourceTypes(Object.keys(SOURCE_LABELS)).customerResearch) expect(SOURCE_LABELS[code]).toBeDefined();
  });
});
