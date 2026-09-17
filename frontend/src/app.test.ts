import { describe, expect, it } from "vitest";
import { navigationSections } from "./app";

describe("navigationSections", () => {
  it("provides the initial auditor navigation", () => {
    expect(navigationSections).toEqual([
      "Visão geral",
      "Ambientes",
      "Audit runs",
      "Inventário",
      "Mappings",
      "Findings",
    ]);
  });
});
